package onec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"orderissue/internal/usecase"

	"github.com/sony/gobreaker/v2"
	"golang.org/x/time/rate"
)

// NewClient создаёт клиент. logger может быть nil.
func NewClient(cfg Config, clock usecase.Clock, logger *slog.Logger) (*Client, error) {
	if cfg.BaseURL == "" || cfg.User == "" || cfg.Password == "" {
		// С пустым логином или паролем 1С засчитает неудачный вход — не отправляем.
		return nil, errors.New("onec: не заданы адрес, логин или пароль 1С")
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	limit := rate.Inf
	if cfg.RatePerSecond > 0 {
		limit = rate.Limit(cfg.RatePerSecond)
	}
	c := &Client{
		cfg:     cfg,
		http:    &http.Client{},
		clock:   clock,
		logger:  logger,
		limiter: rate.NewLimiter(limit, max(1, cfg.MaxConcurrent)),
		slots:   make(chan struct{}, max(1, cfg.MaxConcurrent)),
	}
	c.breaker = gobreaker.NewCircuitBreaker[[]byte](gobreaker.Settings{
		Name:        "1С",
		MaxRequests: 1,
		Timeout:     cfg.BreakerOpenFor,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.BreakerFailures
		},
		IsSuccessful: func(err error) bool { return !isOutage(err) },
		OnStateChange: func(_ string, from, to gobreaker.State) {
			c.mu.Lock()
			if to == gobreaker.StateOpen {
				c.breakerOpened = c.clock.Now()
			}
			c.mu.Unlock()
			logger.Warn("размыкатель 1С сменил состояние", "from", from.String(), "to", to.String())
		},
	})
	return c, nil
}

// idempotent — можно ли повторять запрос, не зная, дошёл ли прошлый.
func (c call) idempotent() bool { return c.method == http.MethodGet }

// get выполняет GET с повторами.
func (c *Client) get(ctx context.Context, op, path, query string) ([]byte, error) {
	return c.execute(ctx, call{op: op, method: http.MethodGet, path: path, query: query})
}

// post выполняет POST. Повторяется только 429: 1С отклонила запрос до обработки.
func (c *Client) post(ctx context.Context, op, path string, body []byte) ([]byte, error) {
	return c.execute(ctx, call{op: op, method: http.MethodPost, path: path, query: "$format=json", body: body})
}

func (c *Client) execute(ctx context.Context, cl call) ([]byte, error) {
	started := c.clock.Now()
	raw, err := c.breaker.Execute(func() ([]byte, error) { return c.withRetries(ctx, cl, started) })
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		err = &usecase.UpstreamError{
			Code: usecase.UpstreamCircuitOpen, Operation: cl.op,
			RetryAfter: c.breakerRetryAfter(), Err: err,
		}
	}
	var upErr *usecase.UpstreamError
	if errors.As(err, &upErr) {
		c.remember(upErr)
	}
	return raw, err
}

func (c *Client) withRetries(ctx context.Context, cl call, started time.Time) ([]byte, error) {
	for attempt := 1; ; attempt++ {
		raw, err := c.attempt(ctx, cl)
		var upErr *usecase.UpstreamError
		if !errors.As(err, &upErr) {
			return raw, err // успех или ErrNotFound
		}
		upErr.Operation, upErr.Attempts, upErr.Elapsed = cl.op, attempt, c.clock.Now().Sub(started)

		if !c.retryable(cl, upErr) || attempt >= c.cfg.MaxAttempts {
			return nil, upErr
		}
		pause := c.pauseBefore(attempt+1, upErr)
		// Пауза не помещается в дедлайн — ждать бессмысленно, отдаём конкретную причину сейчас.
		if deadline, ok := ctx.Deadline(); ok && c.clock.Now().Add(pause).After(deadline) {
			return nil, upErr
		}
		c.logger.Warn("повтор запроса к 1С", "op", cl.op, "attempt", attempt+1, "pause", pause, "code", upErr.Code)
		if err := c.clock.Sleep(ctx, pause); err != nil {
			return nil, upErr
		}
	}
}

// retryable решает, повторять ли после ошибки.
func (c *Client) retryable(cl call, e *usecase.UpstreamError) bool {
	switch e.Code {
	case usecase.UpstreamRateLimited:
		return true // запрос не обработан, дубля не будет даже у POST
	case usecase.UpstreamUnavailable, usecase.UpstreamNetwork:
		return cl.idempotent()
	case usecase.UpstreamTimeout:
		// Истёк таймаут одной попытки — можно повторить; истёк общий дедлайн
		// операции или запрос отменили — повторять некогда и незачем.
		return cl.idempotent() && errors.Is(e.Err, errAttemptTimeout)
	default:
		return false
	}
}

// pauseBefore — пауза перед попыткой n: BackoffBase × 2^(n−2) ±20%.
// Разброс не даёт повторам разных запросов совпадать; ±20% вместо полного
// случайного гарантирует, что пауза не станет почти нулевой.
func (c *Client) pauseBefore(n int, e *usecase.UpstreamError) time.Duration {
	base := c.cfg.BackoffBase << (n - 2)
	jitter := 1 + 0.2*(2*rand.Float64()-1) //nolint:gosec // разброс пауз, не криптография
	pause := time.Duration(float64(base) * jitter)
	if e.RetryAfter > pause {
		pause = e.RetryAfter
	}
	return pause
}

// attempt — один обмен с 1С.
func (c *Client) attempt(ctx context.Context, cl call) ([]byte, error) {
	if err := c.authGate(); err != nil {
		return nil, err
	}
	if c.authUnconfirmed() {
		c.probe.Lock()
		defer c.probe.Unlock()
		// Пока ждали очереди, проба могла закончиться отказом.
		if err := c.authGate(); err != nil {
			return nil, err
		}
	}
	if err := c.waitSharedPause(ctx); err != nil {
		return nil, err
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, &usecase.UpstreamError{Code: usecase.UpstreamTimeout, Err: context.DeadlineExceeded}
	}
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return nil, &usecase.UpstreamError{Code: usecase.UpstreamTimeout, Err: ctx.Err()}
	}

	attemptCtx, cancel := context.WithTimeout(ctx, c.cfg.AttemptTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(attemptCtx, cl.method, c.cfg.BaseURL+cl.path+"?"+cl.query, bytes.NewReader(cl.body))
	if err != nil {
		return nil, &usecase.UpstreamError{Code: usecase.UpstreamRequestRejected, Err: err}
	}
	req.SetBasicAuth(c.cfg.User, c.cfg.Password)
	req.Header.Set("Accept", "application/json")
	if cl.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, transportError(ctx, attemptCtx, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, transportError(ctx, attemptCtx, err)
	}
	raw = bytes.TrimPrefix(raw, utf8BOM) // 1С отдаёт JSON с BOM
	return c.interpret(resp, raw)
}

// interpret переводит код ответа 1С в результат.
func (c *Client) interpret(resp *http.Response, raw []byte) ([]byte, error) {
	status := resp.StatusCode
	switch {
	case status >= 200 && status < 300:
		c.confirmAuth()
		return raw, nil
	case status == http.StatusNotFound:
		c.confirmAuth()
		return nil, usecase.ErrNotFound
	case status == http.StatusUnauthorized:
		c.latchAuth(authRejected, 0)
		return nil, upstream(usecase.UpstreamAuthRejected, status, raw, usecase.OutcomeNotApplied)
	case status == http.StatusForbidden:
		c.latchAuth(authLocked, c.cfg.LockBackoff)
		e := upstream(usecase.UpstreamAccountLocked, status, raw, usecase.OutcomeNotApplied)
		e.RetryAfter = c.cfg.LockBackoff
		return nil, e
	case status == http.StatusTooManyRequests:
		e := upstream(usecase.UpstreamRateLimited, status, raw, usecase.OutcomeNotApplied)
		e.RetryAfter = retryAfter(resp.Header.Get("Retry-After"), c.clock.Now())
		c.pauseAll(e.RetryAfter)
		return nil, e
	case status >= 500:
		return nil, upstream(usecase.UpstreamUnavailable, status, raw, usecase.OutcomeUnknown)
	default:
		return nil, upstream(usecase.UpstreamRequestRejected, status, raw, usecase.OutcomeNotApplied)
	}
}

func upstream(code usecase.UpstreamCode, status int, raw []byte, outcome usecase.WriteOutcome) *usecase.UpstreamError {
	return &usecase.UpstreamError{Code: code, UpstreamStatus: status, UpstreamMessage: odataMessage(raw), Outcome: outcome}
}

func transportError(parent, attemptCtx context.Context, err error) *usecase.UpstreamError {
	switch {
	case parent.Err() != nil:
		return &usecase.UpstreamError{Code: usecase.UpstreamTimeout, Outcome: usecase.OutcomeUnknown, Err: parent.Err()}
	case attemptCtx.Err() != nil:
		// %v, а не %w: ошибка HTTP-клиента тоже оборачивает context.DeadlineExceeded,
		// и тогда таймаут попытки не отличить от истёкшего общего дедлайна.
		return &usecase.UpstreamError{Code: usecase.UpstreamTimeout, Outcome: usecase.OutcomeUnknown, Err: fmt.Errorf("%w: %v", errAttemptTimeout, err)} //nolint:errorlint // %v намеренно, см. комментарий выше
	default:
		return &usecase.UpstreamError{Code: usecase.UpstreamNetwork, Outcome: usecase.OutcomeUnknown, Err: err}
	}
}

// isOutage — считать ли ошибку признаком недоступности 1С для размыкателя.
// 404, отказ в авторизации, 429 и отмена запроса пользователем 1С не «кладут».
func isOutage(err error) bool {
	var e *usecase.UpstreamError
	if !errors.As(err, &e) || errors.Is(e.Err, context.Canceled) {
		return false
	}
	return e.Code == usecase.UpstreamUnavailable || e.Code == usecase.UpstreamTimeout || e.Code == usecase.UpstreamNetwork
}

func (c *Client) authGate() *usecase.UpstreamError {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.auth {
	case authRejected:
		return &usecase.UpstreamError{Code: usecase.UpstreamAuthRejected, UpstreamStatus: http.StatusUnauthorized}
	case authLocked:
		if wait := c.lockedUntil.Sub(c.clock.Now()); wait > 0 {
			return &usecase.UpstreamError{Code: usecase.UpstreamAccountLocked, UpstreamStatus: http.StatusForbidden, RetryAfter: wait}
		}
		c.auth = authUnknown // блокировка истекла — пускаем одну пробу
	default:
	}
	return nil
}

func (c *Client) authUnconfirmed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.auth != authConfirmed
}

func (c *Client) confirmAuth() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.auth == authUnknown {
		c.auth = authConfirmed
	}
}

// latchAuth защёлкивает отказ. После 401 запросы не отправляются до перезапуска:
// неверный пароль сам не станет верным, а каждая попытка приближает блокировку.
func (c *Client) latchAuth(state authState, lockFor time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.auth = state
	c.lockedUntil = c.clock.Now().Add(lockFor)
	c.logger.Error("1С отклонила авторизацию, запросы остановлены", "state", state)
}

func (c *Client) pauseAll(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if until := c.clock.Now().Add(d); until.After(c.pauseUntil) {
		c.pauseUntil = until
	}
}

func (c *Client) waitSharedPause(ctx context.Context) error {
	c.mu.Lock()
	wait := c.pauseUntil.Sub(c.clock.Now())
	c.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	if deadline, ok := ctx.Deadline(); ok && c.clock.Now().Add(wait).After(deadline) {
		return &usecase.UpstreamError{Code: usecase.UpstreamRateLimited, RetryAfter: wait, Outcome: usecase.OutcomeNotApplied}
	}
	if err := c.clock.Sleep(ctx, wait); err != nil {
		return &usecase.UpstreamError{Code: usecase.UpstreamTimeout, Err: err}
	}
	return nil
}

func (c *Client) remember(e *usecase.UpstreamError) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastErr, c.lastErrAt = e, c.clock.Now()
}

func (c *Client) breakerRetryAfter() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.breakerOpened.Add(c.cfg.BreakerOpenFor).Sub(c.clock.Now()); wait > 0 {
		return wait
	}
	return 0
}

// Health — состояние связи с 1С без запроса в неё.
func (c *Client) Health() usecase.IntegrationHealth {
	// Состояние размыкателя читаем до захвата c.mu: State() может сам перевести
	// размыкатель в полуоткрытое состояние и вызвать OnStateChange, которому нужен c.mu.
	breakerState := c.breaker.State()
	retry := c.breakerRetryAfter()

	c.mu.Lock()
	defer c.mu.Unlock()
	h := usecase.IntegrationHealth{LastError: c.lastErr, LastErrAt: c.lastErrAt}
	switch {
	case c.auth == authRejected:
		h.State = usecase.StateAuthRejected
	case c.auth == authLocked && c.clock.Now().Before(c.lockedUntil):
		h.State, h.RetryAfter = usecase.StateLocked, c.lockedUntil.Sub(c.clock.Now())
	case breakerState == gobreaker.StateOpen:
		h.State, h.RetryAfter = usecase.StateUnavailable, retry
	case breakerState == gobreaker.StateHalfOpen:
		h.State = usecase.StateDegraded
	case c.lastErr != nil && c.clock.Now().Sub(c.lastErrAt) < recentErrorWindow:
		h.State = usecase.StateDegraded
	default:
		h.State = usecase.StateOK
	}
	return h
}

// retryAfter разбирает заголовок Retry-After: секунды или HTTP-дата.
// Нет заголовка — ноль, пауза берётся из политики повторов.
func retryAfter(header string, now time.Time) time.Duration {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(header); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}

// odataMessage достаёт текст ошибки из ответа 1С: {"odata.error": {"message": {"value": …}}}.
func odataMessage(raw []byte) string {
	var body struct {
		Error struct {
			Message struct {
				Value string `json:"value"`
			} `json:"message"`
		} `json:"odata.error"`
	}
	if err := json.Unmarshal(raw, &body); err == nil && body.Error.Message.Value != "" {
		return body.Error.Message.Value
	}
	text := strings.TrimSpace(string(raw))
	if len(text) > 300 {
		text = text[:300] + "…"
	}
	return text
}

// entityPath строит путь к набору или объекту: кириллица экранируется,
// а скобки и апострофы ключа остаются как есть, в том виде, в каком их ждёт 1С.
func entityPath(set, key string) string {
	segments := strings.Split(set, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	path := "/" + strings.Join(segments, "/")
	if key != "" {
		path += "(guid'" + key + "')"
	}
	return path
}

// query собирает строку запроса. Ключи вида $skip не экранируются:
// url.Values превратил бы их в %24skip, а 1С ждёт буквальный «$».
func query(skip int, filter string) string {
	q := "$format=json"
	if skip > 0 {
		q += "&$skip=" + strconv.Itoa(skip)
	}
	if filter != "" {
		q += "&$filter=" + strings.ReplaceAll(url.QueryEscape(filter), "+", "%20")
	}
	return q
}

// filterByGUID — условие «поле равно ссылке».
func filterByGUID(field, ref string) string {
	return fmt.Sprintf("%s eq guid'%s'", field, ref)
}
