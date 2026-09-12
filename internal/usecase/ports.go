package usecase

import (
	"context"
	"time"

	"orderissue/internal/domain/order"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/domain/shipment"
	"orderissue/internal/domain/stock"
	"orderissue/internal/domain/vat"
)

// OrderReader читает заказы клиентов.
type OrderReader interface {
	// ListOrders — шапки всех заказов, без строк.
	ListOrders(ctx context.Context) ([]order.Order, error)
	// GetOrder — заказ со всеми строками. Нет такого заказа — ErrNotFound.
	GetOrder(ctx context.Context, ref order.Ref) (order.Order, error)
}

// CatalogReader читает справочники по ключам из заказа, не целиком.
// Ключа нет в 1С — его просто нет в ответе, решение принимает домен.
type CatalogReader interface {
	Products(ctx context.Context, refs []order.ProductRef) (map[order.ProductRef]order.Product, error)
	VatRates(ctx context.Context, refs []vat.RateRef) (map[vat.RateRef]vat.Rate, error)
	Warehouse(ctx context.Context, ref order.WarehouseRef) (order.Warehouse, error)
	Customer(ctx context.Context, ref order.PartnerRef) (order.Customer, error)
}

// StockReader читает остатки и резервы по складу.
type StockReader interface {
	Balances(ctx context.Context, warehouse order.WarehouseRef) ([]stock.Balance, error)
	Reservations(ctx context.Context, warehouse order.WarehouseRef) ([]stock.Reservation, error)
}

// ShipmentRepository пишет и читает черновики реализации.
type ShipmentRepository interface {
	// FindByOrder — все реализации по заказу, включая чужие.
	FindByOrder(ctx context.Context, ref order.Ref) ([]shipment.Recorded, error)
	// Create — один запрос без повторов; при неизвестном исходе — *UpstreamError с OutcomeUnknown.
	Create(ctx context.Context, d shipment.Draft) (shipment.Recorded, error)
	// Get перечитывает реализацию по ключу, который выдала 1С.
	Get(ctx context.Context, ref string) (shipment.Recorded, error)
}

// DocumentRenderer превращает документ на выдачу в PDF.
type DocumentRenderer interface {
	Render(doc pickup.Document) ([]byte, error)
}

// Clock — время и ожидание, подменяемые в тестах.
type Clock interface {
	Now() time.Time
	// Sleep ждёт d или отмены ctx.
	Sleep(ctx context.Context, d time.Duration) error
}

// HealthReporter сообщает состояние связи с 1С.
type HealthReporter interface {
	Health() IntegrationHealth
}
