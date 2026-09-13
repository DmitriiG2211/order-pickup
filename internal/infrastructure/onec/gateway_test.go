package onec_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/order"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/domain/shipment"
	"orderissue/internal/domain/stock"
	"orderissue/internal/domain/vat"
	"orderissue/internal/infrastructure/onec"
	"orderissue/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixture — маленький OData-сервер поверх данных демо-базы (internal/domain/domaintest):
// отдаёт коллекции постранично по 3 записи, как настоящая 1С, и понимает
// единственный вид $filter, которым пользуется Gateway — "Поле eq guid'…'".
// Этого достаточно, чтобы проверить сам Gateway (разбор DTO, пагинацию,
// идемпотентную запись); поведение чужой 1С (503/429/защёлка/размыкатель)
// уже отдельно проверено в client_test.go на подставных ответах.
const pageSize = 3

type fixture struct {
	orders       map[string]order.Order
	products     map[order.ProductRef]order.Product
	vatRates     map[vat.RateRef]vat.Rate
	warehouses   map[order.WarehouseRef]order.Warehouse
	balances     []stock.Balance
	reservations []stock.Reservation

	shipments   []map[string]any
	nextNumber  int
	ambiguous   int // столько следующих POST создадут документ, но ответят 503
	createCalls atomic.Int32
}

func newFixture() *fixture {
	orders := map[string]order.Order{}
	for _, ref := range []order.Ref{domaintest.Order101, domaintest.Order102, domaintest.Order103, domaintest.Order104, domaintest.Order105} {
		orders[string(ref)] = domaintest.Order(ref)
	}
	return &fixture{
		orders: orders, products: domaintest.Products(), vatRates: domaintest.VatRates(),
		warehouses: domaintest.Warehouses(), balances: domaintest.Balances(), reservations: domaintest.Reservations(),
		nextNumber: 200,
	}
}

var keyPattern = regexp.MustCompile(`\(guid'([0-9a-fA-F-]{36})'\)`)

func (f *fixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	key := ""
	if loc := keyPattern.FindStringSubmatchIndex(path); loc != nil {
		key, path = path[loc[2]:loc[3]], path[:loc[0]]
	}
	skip, _ := strconv.Atoi(r.URL.Query().Get("$skip"))
	_, filterValue := parseEqFilter(r.URL.Query().Get("$filter"))

	switch {
	case path == "/Document_ЗаказКлиента" && key != "":
		o, ok := f.orders[key]
		if !ok {
			writeNotFound(w)
			return
		}
		writeOne(w, orderJSON(o))
	case path == "/Document_ЗаказКлиента" && r.Method == http.MethodGet:
		all := make([]map[string]any, 0, len(f.orders))
		for _, ref := range []order.Ref{domaintest.Order101, domaintest.Order102, domaintest.Order103, domaintest.Order104, domaintest.Order105} {
			all = append(all, orderJSON(f.orders[string(ref)]))
		}
		writePage(w, all, skip)
	case path == "/Document_ЗаказКлиента_Товары":
		o, ok := f.orders[filterValue]
		if !ok {
			writePage(w, nil, skip)
			return
		}
		lines := make([]map[string]any, 0, len(o.Lines))
		for _, l := range o.Lines {
			lines = append(lines, orderLineJSON(l))
		}
		writePage(w, lines, skip)
	case path == "/Catalog_СтавкиНДС" && key != "":
		rate, ok := f.vatRates[vat.RateRef(key)]
		if !ok {
			writeNotFound(w)
			return
		}
		writeOne(w, map[string]any{"Ref_Key": key, "Description": rate.Name, "Ставка": rate.Percent.String()})
	case path == "/Catalog_Склады" && key != "":
		wh, ok := f.warehouses[order.WarehouseRef(key)]
		if !ok {
			writeNotFound(w)
			return
		}
		writeOne(w, map[string]any{"Ref_Key": key, "Description": wh.Name})
	case path == "/AccumulationRegister_ТоварыНаСкладах/Balance":
		rows := make([]map[string]any, 0)
		for _, b := range f.balances {
			if string(b.Warehouse) == filterValue {
				rows = append(rows, map[string]any{
					"Номенклатура_Key": string(b.Item.Product), "Характеристика_Key": string(b.Item.Characteristic),
					"Склад_Key": string(b.Warehouse), "ВНаличииBalance": b.OnHand.String(),
				})
			}
		}
		writePage(w, rows, skip)
	case path == "/AccumulationRegister_ТоварыКОтгрузке/Balance":
		rows := make([]map[string]any, 0)
		for _, res := range f.reservations {
			if string(res.Warehouse) == filterValue {
				rows = append(rows, map[string]any{
					"Номенклатура_Key": string(res.Item.Product), "Характеристика_Key": string(res.Item.Characteristic),
					"Склад_Key": string(res.Warehouse), "ДокументОтгрузки": res.Document,
					"ВРезервеBalance": res.Quantity.String(), "КОтгрузкеBalance": "0",
				})
			}
		}
		writePage(w, rows, skip)
	case path == "/Document_РеализацияТоваровУслуг" && key != "":
		for _, s := range f.shipments {
			if s["Ref_Key"] == key {
				writeOne(w, withoutLines(s))
				return
			}
		}
		writeNotFound(w)
	case path == "/Document_РеализацияТоваровУслуг" && r.Method == http.MethodPost:
		f.createShipment(w, r)
	case path == "/Document_РеализацияТоваровУслуг":
		rows := make([]map[string]any, 0)
		for _, s := range f.shipments {
			if s["ЗаказКлиента_Key"] == filterValue {
				rows = append(rows, withoutLines(s))
			}
		}
		writePage(w, rows, skip)
	case path == "/Document_РеализацияТоваровУслуг_Товары":
		var rows []map[string]any
		for _, s := range f.shipments {
			if s["Ref_Key"] == filterValue {
				rows, _ = s["_lines"].([]map[string]any)
			}
		}
		writePage(w, rows, skip)
	default:
		http.Error(w, "unhandled path in test fixture: "+path, http.StatusNotFound)
	}
}

func (f *fixture) createShipment(w http.ResponseWriter, r *http.Request) {
	f.createCalls.Add(1)
	dec := json.NewDecoder(r.Body)
	dec.UseNumber() // сохраняем точное десятичное представление, без прохода через float64
	var body map[string]any
	if err := dec.Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	orderRef, _ := body["ЗаказКлиента_Key"].(string)
	if _, ok := f.orders[orderRef]; !ok {
		http.Error(w, `{"odata.error":{"message":{"value":"Не заполнено поле «Заказ клиента»"}}}`, http.StatusBadRequest)
		return
	}

	f.nextNumber++
	ref := fmt.Sprintf("00000000-0000-0000-0000-%012d", f.nextNumber)
	header := map[string]any{}
	for k, v := range body {
		if k != "Товары" {
			header[k] = v
		}
	}
	header["Ref_Key"] = ref
	header["Number"] = fmt.Sprintf("00УТ-%06d", f.nextNumber)
	if _, ok := header["Posted"]; !ok {
		header["Posted"] = false
	}
	if _, ok := header["DeletionMark"]; !ok {
		header["DeletionMark"] = false
	}
	rawLines, _ := body["Товары"].([]any)
	lines := make([]map[string]any, 0, len(rawLines))
	for i, raw := range rawLines {
		l, _ := raw.(map[string]any)
		row := map[string]any{}
		for k, v := range l {
			row[k] = v
		}
		row["Ref_Key"] = ref
		row["LineNumber"] = strconv.Itoa(i + 1)
		lines = append(lines, row)
	}
	header["_lines"] = lines
	f.shipments = append(f.shipments, header)

	if f.ambiguous > 0 {
		f.ambiguous--
		http.Error(w, `{"odata.error":{"message":{"value":"Сервис временно недоступен"}}}`, http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(withoutLines(header))
}

// withoutLines убирает служебное поле _lines перед тем, как отдать документ
// наружу: Gateway его не ждёт, строки читаются отдельным набором.
func withoutLines(s map[string]any) map[string]any {
	clean := make(map[string]any, len(s))
	for k, v := range s {
		if k != "_lines" {
			clean[k] = v
		}
	}
	return clean
}

// parseEqFilter понимает единственный вид фильтра, которым пользуется Gateway:
// "Поле eq guid'значение'" или "Поле eq 'значение'".
func parseEqFilter(raw string) (field, value string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	parts := strings.SplitN(raw, " eq ", 2)
	if len(parts) != 2 {
		return "", ""
	}
	v := strings.TrimPrefix(parts[1], "guid'")
	v = strings.TrimSuffix(v, "'")
	return parts[0], v
}

func writePage(w http.ResponseWriter, all []map[string]any, skip int) {
	page := []map[string]any{}
	if skip < len(all) {
		page = all[skip:min(skip+pageSize, len(all))]
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"value": page})
}

func writeOne(w http.ResponseWriter, v map[string]any) {
	_ = json.NewEncoder(w).Encode(v)
}

func writeNotFound(w http.ResponseWriter) {
	http.Error(w, `{"odata.error":{"message":{"value":"Данные не найдены."}}}`, http.StatusNotFound)
}

func orderJSON(o order.Order) map[string]any {
	return map[string]any{
		"Ref_Key": string(o.Ref), "Number": o.Number, "Date": o.Date.Format("2006-01-02T15:04:05"),
		"Posted": o.Posted, "DeletionMark": o.DeletionMark, "Статус": o.Status,
		"ЦенаВключаетНДС": o.PriceIncludesVAT, "Склад_Key": string(o.Warehouse), "Партнер_Key": string(o.Customer),
		"Контрагент_Key": o.Accounting.Counterparty, "Организация_Key": o.Accounting.Organization,
		"Валюта_Key": o.Accounting.Currency, "Менеджер_Key": o.Accounting.Manager,
		"НалогообложениеНДС": o.Accounting.VATTaxation,
	}
}

func orderLineJSON(l order.Line) map[string]any {
	return map[string]any{
		"LineNumber": strconv.Itoa(l.Number), "Номенклатура_Key": string(l.Product),
		"Характеристика_Key": string(l.Characteristic), "Количество": l.Quantity.String(),
		"Цена": l.Price.String(), "СтавкаНДС_Key": string(l.VatRate),
		"Склад_Key": string(l.Warehouse), "Отменено": l.Cancelled,
	}
}

func newFixtureGateway(t *testing.T, f *fixture) *onec.Gateway {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	cfg := onec.DefaultConfig()
	cfg.BaseURL, cfg.User, cfg.Password, cfg.RatePerSecond = srv.URL, "test", "test", 0
	client, err := onec.NewClient(cfg, newFakeClock(), nil)
	require.NoError(t, err)
	return onec.NewGateway(client)
}

func draftFor(t *testing.T, gw *onec.Gateway) shipment.Draft {
	t.Helper()
	o, err := gw.GetOrder(context.Background(), domaintest.Order101)
	require.NoError(t, err)
	lines, _, err := pickup.PriceLines(o, pickup.References{Products: domaintest.Products(), VatRates: domaintest.VatRates()})
	require.NoError(t, err)
	return shipment.NewDraft(o, lines, time.Date(2026, 9, 13, 11, 0, 0, 0, time.UTC))
}

func TestGetOrder_Order101_ReadsAllFourLinesAcrossPages(t *testing.T) {
	// Arrange: страница фикстуры — три записи, у заказа четыре строки
	gw := newFixtureGateway(t, newFixture())

	// Act
	o, err := gw.GetOrder(context.Background(), domaintest.Order101)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, domaintest.Order(domaintest.Order101), o, "совпадает с данными демо-базы до поля")
}

func TestGetOrder_UnknownRef_ReturnsErrNotFound(t *testing.T) {
	// Arrange
	gw := newFixtureGateway(t, newFixture())

	// Act
	_, err := gw.GetOrder(context.Background(), order.Ref("00000000-0000-0000-0000-000000000001"))

	// Assert
	require.ErrorIs(t, err, usecase.ErrNotFound)
}

func TestListOrders_FiveOrdersOnTwoPages_ReadsAllHeaders(t *testing.T) {
	// Arrange
	gw := newFixtureGateway(t, newFixture())

	// Act
	orders, err := gw.ListOrders(context.Background())

	// Assert
	require.NoError(t, err)
	numbers := make([]string, 0, len(orders))
	for _, o := range orders {
		numbers = append(numbers, o.Number)
	}
	assert.Equal(t, []string{"00ДМ-000101", "00ДМ-000102", "00ДМ-000103", "00ДМ-000104", "00ДМ-000105"}, numbers)
}

func TestVatRates_KnownAndUnknownKeys_ReturnsOnlyKnownWithPercentFromCatalog(t *testing.T) {
	// Arrange: ставка 13% и ключ, которого нет в 1С
	gw := newFixtureGateway(t, newFixture())
	nonStandard := vat.RateRef("33e4dcc4-064a-11f1-a0b0-48df371887e9")
	missing := vat.RateRef("00000000-0000-0000-0000-000000000009")

	// Act
	rates, err := gw.VatRates(context.Background(), []vat.RateRef{nonStandard, missing, nonStandard})

	// Assert
	require.NoError(t, err)
	require.Len(t, rates, 1)
	assert.Equal(t, "13%", rates[nonStandard].Name)
	assert.Equal(t, "13", rates[nonStandard].Percent.String())
}

func TestBalances_CentralWarehouse_ReturnsOnlyItsRowsFromAllPages(t *testing.T) {
	// Arrange
	gw := newFixtureGateway(t, newFixture())

	// Act
	balances, err := gw.Balances(context.Background(), domaintest.WarehouseCentral)

	// Assert
	require.NoError(t, err)
	var paper string
	for _, b := range balances {
		assert.Equal(t, domaintest.WarehouseCentral, b.Warehouse)
		if b.Item.Product == domaintest.Product0001 {
			paper = b.OnHand.String()
		}
	}
	assert.Equal(t, "2", paper)
	assert.Greater(t, len(balances), pageSize, "строк больше одной страницы")
}

func TestReservations_CentralWarehouse_IncludesOrder102ReserveForPaper(t *testing.T) {
	// Arrange
	gw := newFixtureGateway(t, newFixture())

	// Act
	reservations, err := gw.Reservations(context.Background(), domaintest.WarehouseCentral)

	// Assert
	require.NoError(t, err)
	var found bool
	for _, r := range reservations {
		if r.Document == string(domaintest.Order102) && r.Item.Product == domaintest.Product0001 {
			found = true
			assert.Equal(t, "4", r.Quantity.String())
		}
	}
	assert.True(t, found)
}

func TestCreateThenGet_Draft101_WhatWasWrittenMatchesWhatWasSent(t *testing.T) {
	// Arrange
	f := newFixture()
	gw := newFixtureGateway(t, f)
	d := draftFor(t, gw)

	// Act
	created, err := gw.Create(context.Background(), d)
	require.NoError(t, err)
	recorded, err := gw.Get(context.Background(), created.Ref)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, shipment.Verify(d, recorded), "суммы, количества и ставки дошли до 1С без искажений")
	assert.False(t, recorded.Posted, "черновик, а не проведённый документ")
}

func TestFindByOrder_AfterCreate_FindsOurDraftByMarker(t *testing.T) {
	// Arrange
	f := newFixture()
	gw := newFixtureGateway(t, f)
	d := draftFor(t, gw)
	_, err := gw.Create(context.Background(), d)
	require.NoError(t, err)

	// Act
	candidates, err := gw.FindByOrder(context.Background(), domaintest.Order101)

	// Assert
	require.NoError(t, err)
	ours, ok := shipment.FindOurs(domaintest.Order101, candidates)
	require.True(t, ok)
	assert.Equal(t, d.Comment, ours.Comment)
}

func TestCreate_1CCreatesButResponds503_OutcomeUnknownAndDocumentExists(t *testing.T) {
	// Arrange: запись прошла, ответ «потерялся» — ровно случай из ADR 0004
	f := newFixture()
	f.ambiguous = 1
	gw := newFixtureGateway(t, f)
	d := draftFor(t, gw)

	// Act
	_, err := gw.Create(context.Background(), d)
	candidates, findErr := gw.FindByOrder(context.Background(), domaintest.Order101)

	// Assert
	assert.Equal(t, usecase.OutcomeUnknown, upstreamErr(t, err).Outcome)
	require.NoError(t, findErr)
	assert.Len(t, candidates, 1, "документ создан — поиск его находит")
}

func TestCreate_OrderUnknownTo1C_RejectedAndNotApplied(t *testing.T) {
	// Arrange
	gw := newFixtureGateway(t, newFixture())
	d := draftFor(t, gw)
	d.Order.Ref = "00000000-0000-0000-0000-000000000001"

	// Act
	_, err := gw.Create(context.Background(), d)

	// Assert
	e := upstreamErr(t, err)
	assert.Equal(t, usecase.UpstreamRequestRejected, e.Code)
	assert.Equal(t, usecase.OutcomeNotApplied, e.Outcome)
}
