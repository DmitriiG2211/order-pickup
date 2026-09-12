package usecase

import (
	"context"

	"orderissue/internal/domain/order"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/domain/shipment"
)

// OrderCatalog — список заказов и состав одного заказа.
type OrderCatalog interface {
	ListOrders(ctx context.Context) ([]OrderSummary, error)
	GetOrder(ctx context.Context, ref order.Ref) (pickup.Sheet, error)
}

// DocumentComposer — документ на выдачу: как данные и как PDF.
type DocumentComposer interface {
	PreviewDocument(ctx context.Context, req DocumentRequest) (pickup.Document, error)
	PrintDocument(ctx context.Context, req DocumentRequest) ([]byte, pickup.Document, error)
}

// ShipmentRegistrar — отметка о выдаче в 1С: создать или найти черновик реализации.
type ShipmentRegistrar interface {
	EnsureShipmentDraft(ctx context.Context, ref order.Ref) (ShipmentResult, error)
	FindShipmentDraft(ctx context.Context, ref order.Ref) (shipment.Recorded, error)
}

// StatusReporter — известное состояние связи с 1С, для баннера на странице.
type StatusReporter interface {
	ServiceStatus() IntegrationHealth
}

// API — полный контракт сценариев, которым пользуется транспорт. Реализация —
// internal/service.Service; транспорт зависит только от этого интерфейса.
type API interface {
	OrderCatalog
	DocumentComposer
	ShipmentRegistrar
	StatusReporter
}
