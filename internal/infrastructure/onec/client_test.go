package onec_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/shipment"
	"orderissue/internal/infrastructure/onec"
	"orderissue/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClock не спит по-настоящему, а запоминает паузы и двигает время.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
}

func newFakeClock() *fakeClock { return &fakeClock{now: time.Now()} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	c.mu.Lock()
	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)
	c.mu.Unlock()
	return ctx.Err()
}

func (c *fakeClock) pauses() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration(nil), c.sleeps...)
}

// scripted отвечает по очереди заданными ответами; последний повторяется.
type scripted struct {
	mu        sync.Mutex
	responses []func(http.ResponseWriter, *http.Request)
	calls     atomic.Int32
	lastURI   string
}

func (s *scripted) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	n := int(s.calls.Add(1)) - 1
	s.mu.Lock()
	s.lastURI = r.RequestURI
	respond := s.responses[min(n, len(s.responses)-1)]
	s.mu.Unlock()
	respond(w, r)
}

func status(code int, message string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
		_, _ = fmt.Fprintf(w, `{"odata.error":{"code":"","message":{"lang":"ru","value":%q}}}`, message)
	}
}

func warehouseJSON(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(`{"Ref_Key":"e9f93b98-b2d3-11f1-a0b5-48df371887e9","Description":"Центральный склад"}`))
}

func rateLimited(retryAfter string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", retryAfter)
		status(http.StatusTooManyRequests, "Превышена частота обращений к сервису")(w, r)
	}
}

func testConfig(baseURL string) onec.Config {
	cfg := onec.DefaultConfig()
	cfg.BaseURL, cfg.User, cfg.Password = baseURL, "user", "password"
	cfg.RatePerSecond = 0
	return cfg
}

func newGateway(t *testing.T, handler http.Handler, clock *fakeClock, mutate func(*onec.Config)) (*onec.Gateway, *onec.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg := testConfig(srv.URL + "/odata")
	if mutate != nil {
		mutate(&cfg)
	}
	client, err := onec.NewClient(cfg, clock, nil)
	require.NoError(t, err)
	return onec.NewGateway(client), client
}

func upstreamErr(t *testing.T, err error) *usecase.UpstreamError {
	t.Helper()
	var e *usecase.UpstreamError
	require.ErrorAs(t, err, &e)
	return e
}

func TestNewClient_EmptyPassword_Refused(t *testing.T) {
	// Arrange: пустой пароль 1С засчитала бы как неудачный вход
	cfg := testConfig("http://1c.local")
	cfg.Password = ""

	// Act
	_, err := onec.NewClient(cfg, newFakeClock(), nil)

	// Assert
	require.Error(t, err)
}

func TestRead_TwoTransient503ThenSuccess_RetriesWithGrowingPauses(t *testing.T) {
	// Arrange
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){
		status(503, "Сервис временно недоступен"), status(503, "Сервис временно недоступен"), warehouseJSON,
	}}
	clock := newFakeClock()
	gw, _ := newGateway(t, h, clock, nil)

	// Act
	wh, err := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Центральный склад", wh.Name)
	assert.EqualValues(t, 3, h.calls.Load())
	pauses := clock.pauses()
	require.Len(t, pauses, 2)
	assert.InDelta(t, time.Second, pauses[0], float64(200*time.Millisecond), "первая пауза 1 с ±20%%")
	assert.InDelta(t, 2*time.Second, pauses[1], float64(400*time.Millisecond), "вторая пауза 2 с ±20%%")
}

func TestRead_Always503_GivesUpAfterFourAttemptsWithSpecificError(t *testing.T) {
	// Arrange
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){status(503, "Сервис временно недоступен")}}
	gw, _ := newGateway(t, h, newFakeClock(), nil)

	// Act
	_, err := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)

	// Assert
	e := upstreamErr(t, err)
	assert.Equal(t, usecase.UpstreamUnavailable, e.Code)
	assert.Equal(t, 4, e.Attempts)
	assert.Equal(t, 503, e.UpstreamStatus)
	assert.Equal(t, "Сервис временно недоступен", e.UpstreamMessage)
	assert.Equal(t, "чтение склада", e.Operation)
	assert.EqualValues(t, 4, h.calls.Load())
}

func TestRead_RateLimitedWithRetryAfter_WaitsAtLeastAsAsked(t *testing.T) {
	// Arrange: 1С просит подождать 3 секунды — это дольше первой паузы в 1 с
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){rateLimited("3"), warehouseJSON}}
	clock := newFakeClock()
	gw, _ := newGateway(t, h, clock, nil)

	// Act
	_, err := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)

	// Assert
	require.NoError(t, err)
	require.NotEmpty(t, clock.pauses())
	assert.GreaterOrEqual(t, clock.pauses()[0], 3*time.Second)
}

func TestRead_RetryAfterBeyondDeadline_FailsFastWithRateLimited(t *testing.T) {
	// Arrange: ждать 10 секунд при дедлайне 2 секунды бессмысленно
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){rateLimited("10")}}
	clock := newFakeClock()
	gw, _ := newGateway(t, h, clock, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Act
	_, err := gw.Warehouse(ctx, domaintest.WarehouseCentral)

	// Assert
	e := upstreamErr(t, err)
	assert.Equal(t, usecase.UpstreamRateLimited, e.Code)
	assert.Equal(t, 10*time.Second, e.RetryAfter)
	assert.Empty(t, clock.pauses(), "не ждали впустую")
}

func TestRead_WrongPassword_LatchesAndSendsNothingMore(t *testing.T) {
	// Arrange
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){status(401, "Ошибка аутентификации")}}
	gw, client := newGateway(t, h, newFakeClock(), nil)

	// Act
	_, first := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)
	_, second := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)

	// Assert
	assert.Equal(t, usecase.UpstreamAuthRejected, upstreamErr(t, first).Code)
	assert.Equal(t, usecase.UpstreamAuthRejected, upstreamErr(t, second).Code)
	assert.EqualValues(t, 1, h.calls.Load(), "каждая попытка входа приближает блокировку общей учётки")
	assert.Equal(t, usecase.StateAuthRejected, client.Health().State)
}

func TestRead_TenConcurrentCallsWithWrongPassword_OnlyOneReachesServer(t *testing.T) {
	// Arrange: десять вкладок открылись одновременно, а пароль в .env неверный
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(20 * time.Millisecond)
			status(401, "Ошибка аутентификации")(w, r)
		},
	}}
	gw, _ := newGateway(t, h, newFakeClock(), nil)

	// Act
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = gw.Warehouse(context.Background(), domaintest.WarehouseCentral)
		}()
	}
	wg.Wait()

	// Assert
	assert.EqualValues(t, 1, h.calls.Load())
}

func TestRead_AccountLocked_NoRetryAndNoRequestsUntilLockExpires(t *testing.T) {
	// Arrange
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){status(403, "Учётная запись заблокирована")}}
	gw, client := newGateway(t, h, newFakeClock(), nil)

	// Act
	_, first := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)
	_, second := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)

	// Assert
	e := upstreamErr(t, first)
	assert.Equal(t, usecase.UpstreamAccountLocked, e.Code)
	assert.Equal(t, 5*time.Minute, e.RetryAfter)
	assert.Equal(t, usecase.UpstreamAccountLocked, upstreamErr(t, second).Code)
	assert.EqualValues(t, 1, h.calls.Load())
	assert.Equal(t, usecase.StateLocked, client.Health().State)
}

func TestRead_BadRequest_NotRetriedAndCarriesMessageFrom1C(t *testing.T) {
	// Arrange
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){status(400, "Ошибка при разборе опции запроса $filter")}}
	gw, _ := newGateway(t, h, newFakeClock(), nil)

	// Act
	_, err := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)

	// Assert
	e := upstreamErr(t, err)
	assert.Equal(t, usecase.UpstreamRequestRejected, e.Code)
	assert.Equal(t, "Ошибка при разборе опции запроса $filter", e.UpstreamMessage)
	assert.EqualValues(t, 1, h.calls.Load())
}

func TestRead_NotFound_ReturnsErrNotFoundWithoutRetry(t *testing.T) {
	// Arrange
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){status(404, "Данные не найдены.")}}
	gw, _ := newGateway(t, h, newFakeClock(), nil)

	// Act
	_, err := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)

	// Assert
	require.ErrorIs(t, err, usecase.ErrNotFound)
	assert.EqualValues(t, 1, h.calls.Load())
}

func TestRead_1CHangs_EachAttemptTimesOutAndReportsTimeout(t *testing.T) {
	// Arrange: 1С не отвечает дольше таймаута попытки
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){
		func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
			warehouseJSON(w, r)
		},
	}}
	gw, _ := newGateway(t, h, newFakeClock(), func(c *onec.Config) {
		c.AttemptTimeout = 30 * time.Millisecond
		c.MaxAttempts = 2
	})

	// Act
	_, err := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)

	// Assert
	e := upstreamErr(t, err)
	assert.Equal(t, usecase.UpstreamTimeout, e.Code)
	assert.Equal(t, 2, e.Attempts, "таймаут попытки повторяется, в отличие от общего дедлайна")
}

func TestRead_ResponseStartsWithBOM_DecodedNormally(t *testing.T) {
	// Arrange: 1С отдаёт JSON с BOM
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(append([]byte{0xEF, 0xBB, 0xBF}, `{"Description":"Центральный склад"}`...))
		},
	}}
	gw, _ := newGateway(t, h, newFakeClock(), nil)

	// Act
	wh, err := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Центральный склад", wh.Name)
}

func TestRead_RequestLine_KeepsDollarOptionsAndGuidKeyLiteral(t *testing.T) {
	// Arrange: url.Values превратил бы $format в %24format, а скобки ключа — в %28
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){warehouseJSON}}
	gw, _ := newGateway(t, h, newFakeClock(), nil)

	// Act
	_, err := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, h.lastURI, "(guid'e9f93b98-b2d3-11f1-a0b5-48df371887e9')?$format=json")
}

func TestReadAll_1CIgnoresSkip_ReportsPaginationStuckInsteadOfLooping(t *testing.T) {
	// Arrange: сервер на любой $skip отдаёт одну и ту же непустую страницу
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"value":[{"Ref_Key":"a","Number":"1","Date":"2026-08-03T09:14:27"}]}`))
		},
	}}
	gw, _ := newGateway(t, h, newFakeClock(), nil)

	// Act
	_, err := gw.ListOrders(context.Background())

	// Assert
	assert.Equal(t, usecase.UpstreamPaginationStuck, upstreamErr(t, err).Code)
	assert.EqualValues(t, 2, h.calls.Load())
}

func TestWrite_503_NotRetriedBecauseDocumentMayExist(t *testing.T) {
	// Arrange
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){status(503, "Сервис временно недоступен")}}
	gw, _ := newGateway(t, h, newFakeClock(), nil)

	// Act
	_, err := gw.Create(context.Background(), draft(t))

	// Assert
	e := upstreamErr(t, err)
	assert.Equal(t, usecase.UpstreamUnavailable, e.Code)
	assert.Equal(t, usecase.OutcomeUnknown, e.Outcome)
	assert.EqualValues(t, 1, h.calls.Load(), "повторный POST создал бы дубль")
}

func TestWrite_RateLimitedThenCreated_RetriedBecauseNotApplied(t *testing.T) {
	// Arrange: 429 значит, что 1С запрос не обработала — дубля не будет
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){
		rateLimited("1"),
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Ref_Key":"5abb9f85-4592-11f1-a0b5-48df371887e9","Number":"00УТ-000201","Date":"2026-09-13T11:00:00"}`))
		},
	}}
	gw, _ := newGateway(t, h, newFakeClock(), nil)

	// Act
	created, err := gw.Create(context.Background(), draft(t))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "00УТ-000201", created.Number)
	assert.EqualValues(t, 2, h.calls.Load())
}

func TestBreaker_FiveRequestsExhaustRetries_NextFailsFastWithoutRequest(t *testing.T) {
	// Arrange: одна попытка на запрос, чтобы тест был коротким
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){status(503, "Сервис временно недоступен")}}
	gw, client := newGateway(t, h, newFakeClock(), func(c *onec.Config) { c.MaxAttempts = 1 })
	for range 5 {
		_, _ = gw.Warehouse(context.Background(), domaintest.WarehouseCentral)
	}

	// Act
	_, err := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)

	// Assert
	e := upstreamErr(t, err)
	assert.Equal(t, usecase.UpstreamCircuitOpen, e.Code)
	assert.Positive(t, e.RetryAfter)
	assert.EqualValues(t, 5, h.calls.Load(), "лежащую 1С больше не нагружаем")
	assert.Equal(t, usecase.StateUnavailable, client.Health().State)
}

func TestBreaker_AfterOpenPeriod_SuccessfulProbeRestoresRequests(t *testing.T) {
	// Arrange
	var healthy atomic.Bool
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){
		func(w http.ResponseWriter, r *http.Request) {
			if healthy.Load() {
				warehouseJSON(w, r)
				return
			}
			status(503, "Сервис временно недоступен")(w, r)
		},
	}}
	gw, client := newGateway(t, h, newFakeClock(), func(c *onec.Config) {
		c.MaxAttempts = 1
		c.BreakerOpenFor = 30 * time.Millisecond
	})
	for range 5 {
		_, _ = gw.Warehouse(context.Background(), domaintest.WarehouseCentral)
	}
	healthy.Store(true)
	time.Sleep(50 * time.Millisecond)

	// Act
	_, err := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)

	// Assert
	require.NoError(t, err)
	assert.NotEqual(t, usecase.StateUnavailable, client.Health().State)
}

func TestBreaker_NotFoundAnswers_DoNotCountAsOutage(t *testing.T) {
	// Arrange: 404 — нормальный ответ живой 1С
	h := &scripted{responses: []func(http.ResponseWriter, *http.Request){status(404, "Данные не найдены.")}}
	gw, _ := newGateway(t, h, newFakeClock(), nil)

	// Act
	for range 7 {
		_, err := gw.Warehouse(context.Background(), domaintest.WarehouseCentral)
		require.ErrorIs(t, err, usecase.ErrNotFound)
	}

	// Assert
	assert.EqualValues(t, 7, h.calls.Load(), "размыкатель не открылся")
}

func draft(t *testing.T) shipment.Draft {
	t.Helper()
	o := domaintest.Order(domaintest.Order101)
	return shipment.NewDraft(o, nil, time.Date(2026, 9, 13, 11, 0, 0, 0, time.UTC))
}
