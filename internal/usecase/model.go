// Package usecase — контракт между HTTP и бизнес-логикой: сценарии, порты,
// DTO и ошибки. Реализация контракта — internal/service.
package usecase

import (
	"time"

	"orderissue/internal/domain/order"
	"orderissue/internal/domain/person"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/domain/shipment"
)

// Deps — порты, нужные конкретной реализации API (internal/service).
type Deps struct {
	Orders       OrderReader
	Catalog      CatalogReader
	Stock        StockReader
	Shipments    ShipmentRepository
	Renderer     DocumentRenderer
	Inflector    person.NameInflector
	Health       HealthReporter
	Clock        Clock
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// OrderSummary — строка списка заказов.
type OrderSummary struct {
	Ref           order.Ref
	Number        string
	Date          time.Time
	CustomerName  string
	WarehouseName string
	Status        string
	BlockReasons  []order.BlockReason
}

// DocumentRequest — заказ и данные получателя.
type DocumentRequest struct {
	OrderRef order.Ref
	Receiver pickup.ReceiverInput
}

// ShipmentResult — записанный черновик и признак, создан он этим вызовом или уже был.
type ShipmentResult struct {
	Draft   shipment.Recorded
	Created bool
}

// IntegrationState — состояние связи с 1С для баннера на странице.
type IntegrationState string

const (
	StateOK           IntegrationState = "ok"
	StateDegraded     IntegrationState = "degraded"
	StateUnavailable  IntegrationState = "unavailable"
	StateAuthRejected IntegrationState = "auth_rejected"
	StateLocked       IntegrationState = "account_locked"
)

// IntegrationHealth — что знаем о 1С прямо сейчас, без запроса в неё.
type IntegrationHealth struct {
	State      IntegrationState
	RetryAfter time.Duration
	LastError  *UpstreamError
	LastErrAt  time.Time
}
