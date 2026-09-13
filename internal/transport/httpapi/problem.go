// Package httpapi — хендлеры HTTP API поверх сценариев usecase.
//
// Здесь и только здесь ошибки usecase и domain превращаются в HTTP-ответы
// по RFC 9457 (application/problem+json) с конкретным кодом (ADR 0011).
// Хендлер не принимает решений — вызывает сценарий и переводит результат.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"orderissue/internal/domain/order"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/usecase"
)

// Problem — тело ошибки по RFC 9457.
type Problem struct {
	Status            int               `json:"status"`
	Code              string            `json:"code"`
	Title             string            `json:"title"`
	Detail            string            `json:"detail,omitempty"`
	Operation         string            `json:"operation,omitempty"`
	Attempts          int               `json:"attempts,omitempty"`
	RetryAfterSeconds int               `json:"retryAfterSeconds,omitempty"`
	Upstream          *UpstreamProblem  `json:"upstream,omitempty"`
	Errors            map[string]string `json:"errors,omitempty"`
	Mismatches        []string          `json:"mismatches,omitempty"`
}

// UpstreamProblem — что именно ответила 1С, для разбора без парсинга PDF или логов.
type UpstreamProblem struct {
	Status  int    `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

// writeProblem пишет Problem как application/problem+json.
func writeProblem(w http.ResponseWriter, p Problem) {
	w.Header().Set("Content-Type", "application/problem+json;charset=utf-8")
	if p.RetryAfterSeconds > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(p.RetryAfterSeconds))
	}
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// writeError разбирает ошибку сценария на конкретный код problem+json.
// Порядок проверок важен: конкретные типы раньше общих врапперов.
func writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, order.ErrInvalidRef) {
		writeProblem(w, Problem{Status: http.StatusBadRequest, Code: "INVALID_ORDER_REF", Title: "Неверная ссылка на заказ", Detail: err.Error()})
		return
	}
	if fe, ok := as[pickup.FieldErrors](err); ok {
		out := make(map[string]string, len(fe))
		for f, msg := range fe {
			out[string(f)] = msg
		}
		writeProblem(w, Problem{Status: http.StatusUnprocessableEntity, Code: "VALIDATION_FAILED", Title: "Проверьте поля", Errors: out})
		return
	}
	if errors.Is(err, usecase.ErrOrderNotFound) {
		writeProblem(w, Problem{Status: http.StatusNotFound, Code: "ORDER_NOT_FOUND", Title: "Заказ не найден", Detail: err.Error()})
		return
	}
	if errors.Is(err, usecase.ErrShipmentDraftNotFound) {
		writeProblem(w, Problem{Status: http.StatusNotFound, Code: "SHIPMENT_DRAFT_NOT_FOUND", Title: "Черновика ещё нет", Detail: err.Error()})
		return
	}
	if e, ok := as[*usecase.OrderNotIssuableError](err); ok {
		writeProblem(w, Problem{
			Status: http.StatusConflict, Code: "ORDER_NOT_ISSUABLE",
			Title: "Заказ нельзя выдавать", Detail: reasonsText(e.Reasons),
		})
		return
	}
	if e, ok := as[*pickup.MissingReferenceError](err); ok {
		writeProblem(w, Problem{
			Status: http.StatusBadGateway, Code: "ONEC_DATA_INCONSISTENT",
			Title: "Данные 1С не согласованы", Detail: e.Error(),
		})
		return
	}
	if e, ok := as[*usecase.ShipmentRejectedError](err); ok {
		writeProblem(w, Problem{
			Status: http.StatusBadGateway, Code: "SHIPMENT_REJECTED",
			Title: "1С отклонила черновик реализации", Detail: e.Error(),
			Upstream: &UpstreamProblem{Status: e.Upstream.UpstreamStatus, Message: e.Upstream.UpstreamMessage},
		})
		return
	}
	if e, ok := as[*usecase.ShipmentNotConfirmedError](err); ok {
		texts := make([]string, len(e.Mismatches))
		for i, m := range e.Mismatches {
			texts[i] = m.String()
		}
		writeProblem(w, Problem{
			Status: http.StatusBadGateway, Code: "SHIPMENT_NOT_CONFIRMED",
			Title: "Черновик записан, но не совпал с отправленным", Detail: e.Error(), Mismatches: texts,
		})
		return
	}
	if e, ok := as[*usecase.UpstreamError](err); ok {
		writeUpstreamProblem(w, e)
		return
	}
	writeProblem(w, Problem{Status: http.StatusInternalServerError, Code: "INTERNAL", Title: "Внутренняя ошибка сервиса"})
}

// as — errors.As без предварительного объявления переменной результата.
func as[T error](err error) (T, bool) {
	var target T
	ok := errors.As(err, &target)
	return target, ok
}

func writeUpstreamProblem(w http.ResponseWriter, e *usecase.UpstreamError) {
	status, code, title := upstreamStatusAndCode(e.Code)
	writeProblem(w, Problem{
		Status: status, Code: code, Title: title, Detail: e.Error(),
		Operation: e.Operation, Attempts: e.Attempts, RetryAfterSeconds: seconds(e.RetryAfter),
		Upstream: upstreamDetail(e),
	})
}

func upstreamStatusAndCode(code usecase.UpstreamCode) (status int, apiCode, title string) {
	switch code {
	case usecase.UpstreamTimeout:
		return http.StatusGatewayTimeout, "ONEC_TIMEOUT", "1С не ответила"
	case usecase.UpstreamUnavailable:
		return http.StatusServiceUnavailable, "ONEC_UNAVAILABLE", "1С недоступна"
	case usecase.UpstreamRateLimited:
		return http.StatusServiceUnavailable, "ONEC_RATE_LIMITED", "1С просит подождать"
	case usecase.UpstreamCircuitOpen:
		return http.StatusServiceUnavailable, "ONEC_CIRCUIT_OPEN", "1С временно признана недоступной"
	case usecase.UpstreamNetwork:
		return http.StatusBadGateway, "ONEC_NETWORK", "Нет связи с 1С"
	case usecase.UpstreamAuthRejected:
		return http.StatusServiceUnavailable, "ONEC_AUTH_REJECTED", "1С отклонила логин или пароль"
	case usecase.UpstreamAccountLocked:
		return http.StatusServiceUnavailable, "ONEC_ACCOUNT_LOCKED", "Учётная запись 1С заблокирована"
	case usecase.UpstreamRequestRejected:
		return http.StatusBadGateway, "ONEC_REQUEST_REJECTED", "1С отклонила запрос"
	case usecase.UpstreamPaginationStuck:
		return http.StatusBadGateway, "ONEC_PAGINATION_STUCK", "1С зациклила постраничное чтение"
	default:
		return http.StatusBadGateway, "ONEC_BAD_RESPONSE", "Не удалось разобрать ответ 1С"
	}
}

func upstreamDetail(e *usecase.UpstreamError) *UpstreamProblem {
	if e.UpstreamStatus == 0 && e.UpstreamMessage == "" {
		return nil
	}
	return &UpstreamProblem{Status: e.UpstreamStatus, Message: e.UpstreamMessage}
}

func reasonsText(reasons []order.BlockReason) string {
	s := ""
	for i, r := range reasons {
		if i > 0 {
			s += ", "
		}
		switch r {
		case order.BlockedByDeletionMark:
			s += "заказ помечен на удаление"
		case order.BlockedNotPosted:
			s += "заказ не проведён"
		default:
			s += string(r)
		}
	}
	return s
}

// seconds округляет вверх: даже 400 мс паузы стоит показать как «подождите 1 с»,
// а не спрятать за нулём.
func seconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	if s := d / time.Second; s > 0 {
		if d%time.Second == 0 {
			return int(s)
		}
		return int(s) + 1
	}
	return 1
}
