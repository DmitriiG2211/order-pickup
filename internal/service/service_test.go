package service_test

import (
	"time"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/order"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/service"
	"orderissue/internal/usecase"
	"orderissue/internal/usecase/usecasetest"
)

var fixedNow = time.Date(2026, 9, 13, 11, 0, 0, 0, time.UTC)

// harness — фейки и сервис, собранные для одного заказа демо-базы.
type harness struct {
	orders    *usecasetest.Orders
	catalog   *usecasetest.Catalog
	stock     *usecasetest.Stock
	shipments *usecasetest.Shipments
	renderer  *usecasetest.Renderer
	inflector *usecasetest.Inflector
	svc       *service.Service
}

func newHarness(ref order.Ref) *harness {
	o := domaintest.Order(ref)
	h := &harness{
		orders: &usecasetest.Orders{Byref: map[order.Ref]order.Order{ref: o}},
		catalog: &usecasetest.Catalog{
			ByProduct:   domaintest.Products(),
			ByVatRate:   domaintest.VatRates(),
			ByWarehouse: domaintest.Warehouses(),
			ByCustomer:  domaintest.Customers(),
		},
		stock:     &usecasetest.Stock{BalanceRows: domaintest.Balances(), ReservationRows: domaintest.Reservations()},
		shipments: usecasetest.NewShipments(),
		renderer:  &usecasetest.Renderer{Result: []byte("%PDF-fake")},
		inflector: &usecasetest.Inflector{},
	}
	h.svc = service.New(usecase.Deps{
		Orders: h.orders, Catalog: h.catalog, Stock: h.stock, Shipments: h.shipments,
		Renderer: h.renderer, Inflector: h.inflector, Health: &usecasetest.Health{},
		Clock: usecasetest.Clock{Fixed: fixedNow}, ReadTimeout: time.Second, WriteTimeout: time.Second,
	})
	return h
}

func vorontsovInput() pickup.ReceiverInput {
	return pickup.ReceiverInput{
		LastName: "Воронцов", FirstName: "Пётр", MiddleName: "Аркадьевич", Gender: "м",
		Phone: "8 (999) 123-45-67", Email: "p.vorontsov@example.com",
	}
}
