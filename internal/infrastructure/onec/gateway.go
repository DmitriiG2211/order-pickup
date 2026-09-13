package onec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"orderissue/internal/domain/money"
	"orderissue/internal/domain/order"
	"orderissue/internal/domain/shipment"
	"orderissue/internal/domain/stock"
	"orderissue/internal/domain/vat"
	"orderissue/internal/usecase"
)

func NewGateway(c *Client) *Gateway {
	return &Gateway{client: c}
}

var (
	_ usecase.OrderReader        = (*Gateway)(nil)
	_ usecase.CatalogReader      = (*Gateway)(nil)
	_ usecase.StockReader        = (*Gateway)(nil)
	_ usecase.ShipmentRepository = (*Gateway)(nil)
)

// ListOrders читает шапки всех заказов.
func (g *Gateway) ListOrders(ctx context.Context) ([]order.Order, error) {
	dtos, err := getAll[orderDTO](ctx, g.client, "чтение списка заказов", setOrders, "")
	if err != nil {
		return nil, err
	}
	orders := make([]order.Order, 0, len(dtos))
	for _, d := range dtos {
		o, err := d.toDomain()
		if err != nil {
			return nil, badResponse("чтение списка заказов", err)
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// GetOrder читает заказ и его строки. Строки берём отдельной коллекцией
// с пагинацией, а не из вложенной табличной части: на вложенной части нельзя
// проверить, что 1С не обрезала её тем же ограничением страницы.
func (g *Gateway) GetOrder(ctx context.Context, ref order.Ref) (order.Order, error) {
	head, err := getOne[orderDTO](ctx, g.client, "чтение заказа", setOrders, string(ref))
	if err != nil {
		return order.Order{}, err
	}
	o, err := head.toDomain()
	if err != nil {
		return order.Order{}, badResponse("чтение заказа", err)
	}

	lines, err := getAll[orderLineDTO](ctx, g.client, "чтение строк заказа", setOrderLines, filterByGUID("Ref_Key", string(ref)))
	if err != nil {
		return order.Order{}, err
	}
	for _, l := range lines {
		line, err := l.toDomain()
		if err != nil {
			return order.Order{}, badResponse("чтение строк заказа", err)
		}
		o.Lines = append(o.Lines, line)
	}
	slices.SortFunc(o.Lines, func(a, b order.Line) int { return a.Number - b.Number })
	return o, nil
}

// Products читает карточки номенклатуры по ключам; отсутствующих в 1С в ответе нет.
func (g *Gateway) Products(ctx context.Context, refs []order.ProductRef) (map[order.ProductRef]order.Product, error) {
	out := map[order.ProductRef]order.Product{}
	for _, ref := range distinct(refs) {
		d, err := getOne[productDTO](ctx, g.client, "чтение номенклатуры", setProducts, string(ref))
		if errors.Is(err, usecase.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[ref] = order.Product{Ref: ref, Article: d.Article, Name: d.Description}
	}
	return out, nil
}

// VatRates читает ставки НДС по ключам.
func (g *Gateway) VatRates(ctx context.Context, refs []vat.RateRef) (map[vat.RateRef]vat.Rate, error) {
	out := map[vat.RateRef]vat.Rate{}
	for _, ref := range distinct(refs) {
		d, err := getOne[vatRateDTO](ctx, g.client, "чтение ставки НДС", setVatRates, string(ref))
		if errors.Is(err, usecase.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		percent, err := money.ParseDecimal(d.Rate.String())
		if err != nil {
			return nil, badResponse("чтение ставки НДС", err)
		}
		rate, err := vat.NewRate(d.Description, percent)
		if err != nil {
			return nil, badResponse("чтение ставки НДС", err)
		}
		out[ref] = rate
	}
	return out, nil
}

// Warehouse читает склад.
func (g *Gateway) Warehouse(ctx context.Context, ref order.WarehouseRef) (order.Warehouse, error) {
	d, err := getOne[namedDTO](ctx, g.client, "чтение склада", setWarehouses, string(ref))
	if err != nil {
		return order.Warehouse{}, err
	}
	return order.Warehouse{Ref: ref, Name: d.Description}, nil
}

// Customer читает партнёра — клиента заказа.
func (g *Gateway) Customer(ctx context.Context, ref order.PartnerRef) (order.Customer, error) {
	d, err := getOne[partnerDTO](ctx, g.client, "чтение клиента", setPartners, string(ref))
	if err != nil {
		return order.Customer{}, err
	}
	return order.Customer{Ref: ref, Name: d.Description, IsPerson: d.IsPerson}, nil
}

// Balances читает физические остатки склада.
func (g *Gateway) Balances(ctx context.Context, warehouse order.WarehouseRef) ([]stock.Balance, error) {
	const op = "чтение остатков склада"
	dtos, err := getAll[balanceDTO](ctx, g.client, op, setOnHand, filterByGUID("Склад_Key", string(warehouse)))
	if err != nil {
		return nil, err
	}
	out := make([]stock.Balance, 0, len(dtos))
	for _, d := range dtos {
		onHand, err := money.ParseDecimal(d.OnHand.String())
		if err != nil {
			return nil, badResponse(op, err)
		}
		out = append(out, stock.Balance{
			Item:      stock.Item{Product: order.ProductRef(d.ProductKey), Characteristic: order.CharacteristicRef(d.CharacteristicKey)},
			Warehouse: order.WarehouseRef(d.WarehouseKey),
			OnHand:    onHand,
		})
	}
	return out, nil
}

// Reservations читает резервы склада. Товар обещан документу, пока он «в резерве»
// или уже «к отгрузке», поэтому ресурсы складываются.
func (g *Gateway) Reservations(ctx context.Context, warehouse order.WarehouseRef) ([]stock.Reservation, error) {
	const op = "чтение резервов склада"
	dtos, err := getAll[reservationDTO](ctx, g.client, op, setReserved, filterByGUID("Склад_Key", string(warehouse)))
	if err != nil {
		return nil, err
	}
	out := make([]stock.Reservation, 0, len(dtos))
	for _, d := range dtos {
		reserved, err := money.ParseDecimal(d.Reserved.String())
		if err != nil {
			return nil, badResponse(op, err)
		}
		toShip, err := money.ParseDecimal(d.ToShip.String())
		if err != nil {
			return nil, badResponse(op, err)
		}
		out = append(out, stock.Reservation{
			Item:      stock.Item{Product: order.ProductRef(d.ProductKey), Characteristic: order.CharacteristicRef(d.CharacteristicKey)},
			Warehouse: order.WarehouseRef(d.WarehouseKey),
			Document:  d.Document,
			Quantity:  reserved.Add(toShip),
		})
	}
	return out, nil
}

// FindByOrder читает все реализации по заказу — свои и чужие.
func (g *Gateway) FindByOrder(ctx context.Context, ref order.Ref) ([]shipment.Recorded, error) {
	const op = "поиск черновиков реализации по заказу"
	dtos, err := getAll[shipmentDTO](ctx, g.client, op, setShipments, filterByGUID("ЗаказКлиента_Key", string(ref)))
	if err != nil {
		return nil, err
	}
	out := make([]shipment.Recorded, 0, len(dtos))
	for _, d := range dtos {
		r, err := d.toDomain(nil)
		if err != nil {
			return nil, badResponse(op, err)
		}
		out = append(out, r)
	}
	return out, nil
}

// Create отправляет черновик одним POST. Что вернула 1С, Create не проверяет:
// сверку делает сценарий, перечитав документ через Get.
func (g *Gateway) Create(ctx context.Context, d shipment.Draft) (shipment.Recorded, error) {
	const op = "создание черновика реализации"
	body, err := json.Marshal(newShipmentPayload(d))
	if err != nil {
		return shipment.Recorded{}, fmt.Errorf("%s: %w", op, err)
	}
	raw, err := g.client.post(ctx, op, entityPath(setShipments, ""), body)
	if err != nil {
		return shipment.Recorded{}, err
	}
	var created shipmentDTO
	if err := decode(raw, &created); err != nil {
		return shipment.Recorded{}, badResponse(op, err)
	}
	return created.toDomain(nil)
}

// Get перечитывает реализацию и её строки коллекцией с пагинацией.
func (g *Gateway) Get(ctx context.Context, ref string) (shipment.Recorded, error) {
	const op = "проверка записанного черновика реализации"
	head, err := getOne[shipmentDTO](ctx, g.client, op, setShipments, ref)
	if err != nil {
		return shipment.Recorded{}, err
	}
	lines, err := getAll[shipmentLineDTO](ctx, g.client, op, setShipmentRows, filterByGUID("Ref_Key", ref))
	if err != nil {
		return shipment.Recorded{}, err
	}
	r, err := head.toDomain(lines)
	if err != nil {
		return shipment.Recorded{}, badResponse(op, err)
	}
	return r, nil
}

// getOne читает объект по ключу.
func getOne[T any](ctx context.Context, c *Client, op, set, key string) (T, error) {
	var out T
	raw, err := c.get(ctx, op, entityPath(set, key), query(0, ""))
	if err != nil {
		return out, err
	}
	if err := decode(raw, &out); err != nil {
		return out, badResponse(op, err)
	}
	return out, nil
}

// getAll читает коллекцию целиком. 1С отдаёт по три записи и не сообщает,
// что есть ещё, поэтому листаем со $skip, пока не придёт пустая страница.
func getAll[T any](ctx context.Context, c *Client, op, set, filter string) ([]T, error) {
	var (
		all      []T
		previous []byte
	)
	for page, skip := 0, 0; page < maxPages; page++ {
		raw, err := c.get(ctx, op, entityPath(set, ""), query(skip, filter))
		if err != nil {
			return nil, err
		}
		var body struct {
			Value []json.RawMessage `json:"value"`
		}
		if err := decode(raw, &body); err != nil {
			return nil, badResponse(op, err)
		}
		if len(body.Value) == 0 {
			return all, nil
		}
		// Та же страница второй раз — 1С игнорирует $skip, иначе зациклимся.
		if bytes.Equal(raw, previous) {
			return nil, &usecase.UpstreamError{Code: usecase.UpstreamPaginationStuck, Operation: op}
		}
		previous = raw

		for _, item := range body.Value {
			var v T
			if err := decode(item, &v); err != nil {
				return nil, badResponse(op, err)
			}
			all = append(all, v)
		}
		skip += len(body.Value)
	}
	return nil, &usecase.UpstreamError{Code: usecase.UpstreamPaginationStuck, Operation: op}
}

func decode(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // числа не проходят через float64, см. ADR 0005
	return dec.Decode(v)
}

func badResponse(op string, err error) error {
	return &usecase.UpstreamError{Code: usecase.UpstreamBadResponse, Operation: op, Err: err, UpstreamMessage: err.Error()}
}

func distinct[T comparable](items []T) []T {
	seen := make(map[T]bool, len(items))
	out := make([]T, 0, len(items))
	for _, it := range items {
		if !seen[it] {
			seen[it] = true
			out = append(out, it)
		}
	}
	return out
}

func (d orderDTO) toDomain() (order.Order, error) {
	date, err := time.Parse(dateLayout, d.Date)
	if err != nil {
		return order.Order{}, fmt.Errorf("дата заказа %s: %w", d.Number, err)
	}
	return order.Order{
		Ref:              order.Ref(d.RefKey),
		Number:           d.Number,
		Date:             date,
		Posted:           d.Posted,
		DeletionMark:     d.DeletionMark,
		Status:           d.Status,
		PriceIncludesVAT: d.PriceIncludesVAT,
		Warehouse:        order.WarehouseRef(d.WarehouseKey),
		Customer:         order.PartnerRef(d.PartnerKey),
		Accounting: order.Accounting{
			Counterparty: d.CounterpartyKey,
			Organization: d.OrganizationKey,
			Currency:     d.CurrencyKey,
			Manager:      d.ManagerKey,
			VATTaxation:  d.VATTaxation,
		},
	}, nil
}

func (d orderLineDTO) toDomain() (order.Line, error) {
	number, err := strconv.Atoi(d.LineNumber)
	if err != nil {
		return order.Line{}, fmt.Errorf("номер строки %q: %w", d.LineNumber, err)
	}
	qty, err := money.ParseDecimal(d.Quantity.String())
	if err != nil {
		return order.Line{}, fmt.Errorf("строка %d, количество: %w", number, err)
	}
	price, err := money.ParseDecimal(d.Price.String())
	if err != nil {
		return order.Line{}, fmt.Errorf("строка %d, цена: %w", number, err)
	}
	return order.Line{
		Number:         number,
		Product:        order.ProductRef(d.ProductKey),
		Characteristic: order.CharacteristicRef(d.CharacteristicKey),
		Quantity:       qty,
		Price:          price,
		VatRate:        vat.RateRef(d.VatRateKey),
		Warehouse:      order.WarehouseRef(d.WarehouseKey),
		Cancelled:      d.Cancelled,
	}, nil
}

func (d shipmentDTO) toDomain(lines []shipmentLineDTO) (shipment.Recorded, error) {
	date, err := time.Parse(dateLayout, d.Date)
	if err != nil {
		return shipment.Recorded{}, fmt.Errorf("дата реализации %s: %w", d.Number, err)
	}
	r := shipment.Recorded{
		Ref: d.RefKey, Number: d.Number, Date: date, Posted: d.Posted, DeletionMark: d.DeletionMark,
		OrderRef: order.Ref(d.OrderKey), Comment: d.Comment,
	}
	slices.SortFunc(lines, func(a, b shipmentLineDTO) int {
		na, _ := strconv.Atoi(a.LineNumber)
		nb, _ := strconv.Atoi(b.LineNumber)
		return na - nb
	})
	for _, l := range lines {
		line, err := l.toDomain()
		if err != nil {
			return shipment.Recorded{}, err
		}
		r.Lines = append(r.Lines, line)
	}
	return r, nil
}

func (d shipmentLineDTO) toDomain() (shipment.RecordedLine, error) {
	var (
		fields = []json.Number{d.Quantity, d.Price, d.Amount, d.VAT, d.WithVAT}
		parsed = make([]money.Decimal, len(fields))
	)
	for i, f := range fields {
		v, err := money.ParseDecimal(f.String())
		if err != nil {
			return shipment.RecordedLine{}, fmt.Errorf("строка %s реализации: %w", d.LineNumber, err)
		}
		parsed[i] = v
	}
	amounts := make([]money.Amount, 3)
	for i, v := range parsed[2:] {
		a, err := money.AmountFromDecimal(v)
		if err != nil {
			return shipment.RecordedLine{}, fmt.Errorf("строка %s реализации, сумма: %w", d.LineNumber, err)
		}
		amounts[i] = a
	}
	return shipment.RecordedLine{
		Product:  order.ProductRef(d.ProductKey),
		Quantity: parsed[0],
		Price:    parsed[1],
		VatRate:  vat.RateRef(d.VatRateKey),
		Sums:     vat.Line{Amount: amounts[0], VAT: amounts[1], WithVAT: amounts[2]},
	}, nil
}

func newShipmentPayload(d shipment.Draft) shipmentPayload {
	o := d.Order
	p := shipmentPayload{
		Date:              d.Date.Format(dateLayout),
		OrderKey:          string(o.Ref),
		PartnerKey:        string(o.Customer),
		CounterpartyKey:   o.Accounting.Counterparty,
		OrganizationKey:   o.Accounting.Organization,
		CurrencyKey:       o.Accounting.Currency,
		WarehouseKey:      string(o.Warehouse),
		ManagerKey:        o.Accounting.Manager,
		ResponsibleKey:    o.Accounting.Manager,
		PriceIncludesVAT:  o.PriceIncludesVAT,
		VATTaxation:       o.Accounting.VATTaxation,
		BusinessOperation: "РеализацияКлиенту",
		Comment:           d.Comment,
	}
	for i, l := range d.Lines {
		p.Lines = append(p.Lines, shipmentLinePayload{
			LineNumber:        strconv.Itoa(i + 1),
			RowCode:           l.Number,
			ProductKey:        string(l.Product),
			CharacteristicKey: string(l.Characteristic),
			PackageKey:        emptyRef,
			Quantity:          json.Number(l.Quantity.String()),
			PackageQuantity:   json.Number(l.Quantity.String()),
			Price:             json.Number(l.Price.String()),
			Amount:            json.Number(l.Sums.Amount.String()),
			VatRateKey:        string(l.VatRate),
			VAT:               json.Number(l.Sums.VAT.String()),
			WithVAT:           json.Number(l.Sums.WithVAT.String()),
			WarehouseKey:      string(l.Warehouse),
			OrderKey:          string(o.Ref),
		})
	}
	return p
}
