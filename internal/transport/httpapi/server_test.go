package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/order"
	"orderissue/internal/domain/person"
	"orderissue/internal/service"
	"orderissue/internal/usecase"
	"orderissue/internal/usecase/usecasetest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testServer(t *testing.T) (http.Handler, *usecasetest.Orders) {
	t.Helper()
	return testServerFor(t, map[order.Ref]order.Order{domaintest.Order101: domaintest.Order(domaintest.Order101)})
}

// testServerFor собирает сервер над своим набором заказов — нужно тестам,
// которым важен конкретный заказ (например, заблокированный 105-й).
func testServerFor(t *testing.T, byRef map[order.Ref]order.Order) (http.Handler, *usecasetest.Orders) {
	t.Helper()
	orders := &usecasetest.Orders{Byref: byRef}
	catalog := &usecasetest.Catalog{
		ByProduct: domaintest.Products(), ByVatRate: domaintest.VatRates(),
		ByWarehouse: domaintest.Warehouses(), ByCustomer: domaintest.Customers(),
	}
	implementation := service.New(usecase.Deps{
		Orders: orders, Catalog: catalog,
		Stock:     &usecasetest.Stock{BalanceRows: domaintest.Balances(), ReservationRows: domaintest.Reservations()},
		Shipments: usecasetest.NewShipments(), Renderer: &usecasetest.Renderer{Result: []byte("%PDF-fake")},
		Inflector: &usecasetest.Inflector{Result: person.FullName{Last: "Воронцова", First: "Петра", Middle: "Аркадьевича"}}, Health: &usecasetest.Health{},
		Clock:       usecasetest.Clock{Fixed: time.Date(2026, 9, 13, 11, 0, 0, 0, time.UTC)},
		ReadTimeout: time.Second, WriteTimeout: time.Second,
	})
	return NewServer(implementation), orders
}

func validRequest() []byte {
	return []byte(`{"orderRef":"2da145f0-d5fc-11f1-a0b3-48df371887e9","customer":{"lastName":"Воронцов","firstName":"Пётр","middleName":"Аркадьевич","gender":"м","phone":"8 (999) 123-45-67","email":"p.vorontsov@example.com"}}`)
}

func TestRoot_ServesPage(t *testing.T) {
	h, _ := testServer(t)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "Выдача заказа")
}
