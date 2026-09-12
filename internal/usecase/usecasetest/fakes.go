// Package usecasetest — фейки портов usecase для тестов сценариев.
// Никакой сети: поведение 1С задаётся полями структур, а не HTTP.
package usecasetest

import (
	"context"
	"time"

	"orderissue/internal/domain/order"
	"orderissue/internal/domain/person"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/domain/shipment"
	"orderissue/internal/domain/stock"
	"orderissue/internal/domain/vat"
	"orderissue/internal/usecase"
)

// Orders — фейковый OrderReader. Err, если задан, возвращается всегда.
type Orders struct {
	Byref map[order.Ref]order.Order
	Err   error
	// Calls считает обращения — тесты проверяют, что при ошибке ввода
	// сценарий вообще не сходил в 1С.
	Calls int
}

func (f *Orders) ListOrders(context.Context) ([]order.Order, error) {
	f.Calls++
	if f.Err != nil {
		return nil, f.Err
	}
	out := make([]order.Order, 0, len(f.Byref))
	for _, o := range f.Byref {
		out = append(out, o)
	}
	return out, nil
}

func (f *Orders) GetOrder(_ context.Context, ref order.Ref) (order.Order, error) {
	f.Calls++
	if f.Err != nil {
		return order.Order{}, f.Err
	}
	o, ok := f.Byref[ref]
	if !ok {
		return order.Order{}, usecase.ErrNotFound
	}
	return o, nil
}

// Catalog — фейковый CatalogReader.
type Catalog struct {
	ByProduct    map[order.ProductRef]order.Product
	ByVatRate    map[vat.RateRef]vat.Rate
	ByWarehouse  map[order.WarehouseRef]order.Warehouse
	ByCustomer   map[order.PartnerRef]order.Customer
	Err          error
	WarehouseErr error
}

func (f *Catalog) Products(_ context.Context, refs []order.ProductRef) (map[order.ProductRef]order.Product, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	out := map[order.ProductRef]order.Product{}
	for _, r := range refs {
		if p, ok := f.ByProduct[r]; ok {
			out[r] = p
		}
	}
	return out, nil
}

func (f *Catalog) VatRates(_ context.Context, refs []vat.RateRef) (map[vat.RateRef]vat.Rate, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	out := map[vat.RateRef]vat.Rate{}
	for _, r := range refs {
		if v, ok := f.ByVatRate[r]; ok {
			out[r] = v
		}
	}
	return out, nil
}

func (f *Catalog) Warehouse(_ context.Context, ref order.WarehouseRef) (order.Warehouse, error) {
	if f.WarehouseErr != nil {
		return order.Warehouse{}, f.WarehouseErr
	}
	if f.Err != nil {
		return order.Warehouse{}, f.Err
	}
	w, ok := f.ByWarehouse[ref]
	if !ok {
		return order.Warehouse{}, usecase.ErrNotFound
	}
	return w, nil
}

func (f *Catalog) Customer(_ context.Context, ref order.PartnerRef) (order.Customer, error) {
	if f.Err != nil {
		return order.Customer{}, f.Err
	}
	c, ok := f.ByCustomer[ref]
	if !ok {
		return order.Customer{}, usecase.ErrNotFound
	}
	return c, nil
}

// Stock — фейковый StockReader.
type Stock struct {
	BalanceRows     []stock.Balance
	ReservationRows []stock.Reservation
	Err             error
}

func (f *Stock) Balances(context.Context, order.WarehouseRef) ([]stock.Balance, error) {
	return f.BalanceRows, f.Err
}

func (f *Stock) Reservations(context.Context, order.WarehouseRef) ([]stock.Reservation, error) {
	return f.ReservationRows, f.Err
}

// Shipments — фейковый ShipmentRepository с управляемым сценарием POST.
type Shipments struct {
	Existing map[order.Ref][]shipment.Recorded
	ByRef    map[string]shipment.Recorded

	// CreateResponses — ответы Create по очереди; последний повторяется.
	// nil-элемент означает «создать по-настоящему».
	CreateResponses []error
	CreateCalls     int
	FindCalls       int

	// nextRef — что выдаёт «1С» новому черновику.
	nextRef int
}

func NewShipments() *Shipments {
	return &Shipments{Existing: map[order.Ref][]shipment.Recorded{}, ByRef: map[string]shipment.Recorded{}}
}

func (f *Shipments) FindByOrder(_ context.Context, ref order.Ref) ([]shipment.Recorded, error) {
	f.FindCalls++
	return f.Existing[ref], nil
}

func (f *Shipments) Create(_ context.Context, d shipment.Draft) (shipment.Recorded, error) {
	idx := f.CreateCalls
	f.CreateCalls++

	var respErr error
	if idx < len(f.CreateResponses) {
		respErr = f.CreateResponses[idx]
	} else if len(f.CreateResponses) > 0 {
		respErr = f.CreateResponses[len(f.CreateResponses)-1]
	}

	f.nextRef++
	ref := refFor(f.nextRef)
	recorded := shipment.Recorded{
		Ref: ref, Number: numberFor(f.nextRef), Date: d.Date,
		OrderRef: d.Order.Ref, Comment: d.Comment,
	}
	for _, l := range d.Lines {
		recorded.Lines = append(recorded.Lines, shipment.RecordedLine{
			Product: l.Product, Quantity: l.Quantity, Price: l.Price, VatRate: l.VatRate, Sums: l.Sums,
		})
	}

	if respErr != nil {
		var upErr *usecase.UpstreamError
		if asUpstream(respErr, &upErr) && upErr.Outcome == usecase.OutcomeUnknown {
			// «Создалось, но ответ потерялся»: документ реально появляется в хранилище.
			f.ByRef[ref] = recorded
			f.Existing[d.Order.Ref] = append(f.Existing[d.Order.Ref], recorded)
		}
		return shipment.Recorded{}, respErr
	}

	f.ByRef[ref] = recorded
	f.Existing[d.Order.Ref] = append(f.Existing[d.Order.Ref], recorded)
	return recorded, nil
}

func (f *Shipments) Get(_ context.Context, ref string) (shipment.Recorded, error) {
	r, ok := f.ByRef[ref]
	if !ok {
		return shipment.Recorded{}, usecase.ErrNotFound
	}
	return r, nil
}

func asUpstream(err error, target **usecase.UpstreamError) bool {
	e, ok := err.(*usecase.UpstreamError) //nolint:errorlint // фейк сравнивает конкретный тип, не цепочку
	if ok {
		*target = e
	}
	return ok
}

func refFor(n int) string    { return "generated-ref-" + itoa(n) }
func numberFor(n int) string { return "00УТ-0002" + itoa(n) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// Renderer — фейковый DocumentRenderer.
type Renderer struct {
	Result []byte
	Err    error
	Calls  int
}

func (f *Renderer) Render(pickup.Document) ([]byte, error) {
	f.Calls++
	return f.Result, f.Err
}

// Inflector — фейк склонения: запоминает вызов и отдаёт заданный результат
// (или ФИО как есть, если Result не задан). Настоящее склонение проверяется
// в пакете petrovich.
type Inflector struct {
	Result    person.FullName
	GotName   person.FullName
	GotGender person.Gender
	Called    bool
}

func (f *Inflector) Genitive(n person.FullName, g person.Gender) person.FullName {
	f.Called, f.GotName, f.GotGender = true, n, g
	if f.Result != (person.FullName{}) {
		return f.Result
	}
	return n
}

var _ person.NameInflector = (*Inflector)(nil)

// Health — фейковый HealthReporter.
type Health struct {
	State usecase.IntegrationHealth
}

func (f *Health) Health() usecase.IntegrationHealth { return f.State }

// Clock — управляемые часы: Now фиксировано, Sleep не ждёт по-настоящему.
type Clock struct {
	Fixed time.Time
}

func (c Clock) Now() time.Time { return c.Fixed }

func (c Clock) Sleep(ctx context.Context, _ time.Duration) error { return ctx.Err() }
