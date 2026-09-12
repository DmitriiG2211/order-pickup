package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"orderissue/internal/domain/order"
	"orderissue/internal/domain/shipment"
)

// UpstreamCode — что именно пошло не так при обращении к 1С. Коды уходят
// клиенту как есть: по общему «ошибка» кладовщик не поймёт, ждать или звонить.
type UpstreamCode string

const (
	UpstreamTimeout         UpstreamCode = "ONEC_TIMEOUT"
	UpstreamUnavailable     UpstreamCode = "ONEC_UNAVAILABLE"
	UpstreamRateLimited     UpstreamCode = "ONEC_RATE_LIMITED"
	UpstreamCircuitOpen     UpstreamCode = "ONEC_CIRCUIT_OPEN"
	UpstreamNetwork         UpstreamCode = "ONEC_NETWORK"
	UpstreamAuthRejected    UpstreamCode = "ONEC_AUTH_REJECTED"
	UpstreamAccountLocked   UpstreamCode = "ONEC_ACCOUNT_LOCKED"
	UpstreamRequestRejected UpstreamCode = "ONEC_REQUEST_REJECTED"
	UpstreamBadResponse     UpstreamCode = "ONEC_BAD_RESPONSE"
	UpstreamPaginationStuck UpstreamCode = "ONEC_PAGINATION_STUCK"
)

// WriteOutcome — известно ли, применился ли запрос на запись.
type WriteOutcome int

const (
	// OutcomeNotApplied — 1С отказала до обработки (429, 4xx): повтор безопасен или бессмыслен, но дубля точно нет.
	OutcomeNotApplied WriteOutcome = iota
	// OutcomeUnknown — ответ не получен (503, таймаут, обрыв): документ мог создаться.
	OutcomeUnknown
)

// UpstreamError — ошибка обращения к 1С со всеми подробностями.
type UpstreamError struct {
	Code UpstreamCode
	// Operation — что делали, по-русски: «чтение строк заказа».
	Operation string
	Attempts  int
	Elapsed   time.Duration
	// RetryAfter — через сколько имеет смысл повторить; 0 — не подсказываем.
	RetryAfter      time.Duration
	UpstreamStatus  int
	UpstreamMessage string
	Outcome         WriteOutcome
	Err             error
}

func (e *UpstreamError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s", e.Operation, e.describe())
	if e.Attempts > 1 {
		fmt.Fprintf(&b, " (попыток: %d за %s)", e.Attempts, e.Elapsed.Round(100*time.Millisecond))
	}
	if e.UpstreamMessage != "" {
		fmt.Fprintf(&b, "; 1С: %s", e.UpstreamMessage)
	}
	return b.String()
}

func (e *UpstreamError) Unwrap() error { return e.Err }

func (e *UpstreamError) describe() string {
	switch e.Code {
	case UpstreamTimeout:
		return "1С не ответила вовремя"
	case UpstreamUnavailable:
		return fmt.Sprintf("1С вернула %d", e.UpstreamStatus)
	case UpstreamRateLimited:
		return "1С просит реже обращаться (429)"
	case UpstreamCircuitOpen:
		return "1С признана недоступной после серии сбоев, запросы временно не отправляются"
	case UpstreamNetwork:
		return "нет связи с 1С"
	case UpstreamAuthRejected:
		return "1С отклонила логин или пароль; запросы остановлены, чтобы не заблокировать учётную запись"
	case UpstreamAccountLocked:
		return "учётная запись 1С заблокирована"
	case UpstreamRequestRejected:
		return fmt.Sprintf("1С отклонила запрос (%d)", e.UpstreamStatus)
	case UpstreamBadResponse:
		return "1С вернула ответ, который не удалось разобрать"
	case UpstreamPaginationStuck:
		return "1С игнорирует $skip и отдаёт одну и ту же страницу"
	default:
		return string(e.Code)
	}
}

var (
	ErrOrderNotFound         = errors.New("заказ не найден в 1С")
	ErrShipmentDraftNotFound = errors.New("черновика реализации по заказу ещё нет")
	// ErrNotFound — порт не нашёл объект по ключу; сценарий решает, что это значит.
	ErrNotFound = errors.New("объект не найден в 1С")
)

// OrderNotIssuableError — заказ нельзя выдавать.
type OrderNotIssuableError struct {
	Number  string
	Reasons []order.BlockReason
}

func (e *OrderNotIssuableError) Error() string {
	return fmt.Sprintf("заказ %s нельзя выдавать: %v", e.Number, e.Reasons)
}

// ShipmentRejectedError — 1С отказалась создавать черновик по существу.
type ShipmentRejectedError struct {
	Upstream *UpstreamError
}

func (e *ShipmentRejectedError) Error() string {
	return "1С отклонила черновик реализации: " + e.Upstream.UpstreamMessage
}

func (e *ShipmentRejectedError) Unwrap() error { return e.Upstream }

// ShipmentNotConfirmedError — черновик в 1С есть, но не совпадает с отправленным.
type ShipmentNotConfirmedError struct {
	Ref        string
	Number     string
	Mismatches []shipment.Mismatch
}

func (e *ShipmentNotConfirmedError) Error() string {
	parts := make([]string, 0, len(e.Mismatches))
	for _, m := range e.Mismatches {
		parts = append(parts, m.String())
	}
	return fmt.Sprintf("черновик %s записан, но не совпал с отправленным: %s", e.Number, strings.Join(parts, "; "))
}
