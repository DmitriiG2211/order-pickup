package service

import (
	"context"
	"errors"

	"orderissue/internal/domain/order"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/domain/stock"
	"orderissue/internal/domain/vat"
	"orderissue/internal/usecase"
)

// ListOrders — список заказов для выбора. Имя клиента или склада, которого
// нет в 1С, остаётся пустым: из-за одной битой ссылки список не должен пропасть.
func (s *Service) ListOrders(ctx context.Context) ([]usecase.OrderSummary, error) {
	ctx, cancel := s.readContext(ctx)
	defer cancel()

	orders, err := s.d.Orders.ListOrders(ctx)
	if err != nil {
		return nil, err
	}
	customers := map[order.PartnerRef]string{}
	warehouses := map[order.WarehouseRef]string{}

	out := make([]usecase.OrderSummary, 0, len(orders))
	for _, o := range orders {
		if _, ok := customers[o.Customer]; !ok {
			c, err := s.d.Catalog.Customer(ctx, o.Customer)
			if err != nil && !errors.Is(err, usecase.ErrNotFound) {
				return nil, err
			}
			customers[o.Customer] = c.Name
		}
		if _, ok := warehouses[o.Warehouse]; !ok {
			w, err := s.d.Catalog.Warehouse(ctx, o.Warehouse)
			if err != nil && !errors.Is(err, usecase.ErrNotFound) {
				return nil, err
			}
			warehouses[o.Warehouse] = w.Name
		}
		out = append(out, usecase.OrderSummary{
			Ref: o.Ref, Number: o.Number, Date: o.Date, Status: o.Status,
			CustomerName: customers[o.Customer], WarehouseName: warehouses[o.Warehouse],
			BlockReasons: o.BlockReasons(),
		})
	}
	return out, nil
}

// GetOrder — экран заказа: строки с суммами, остатки и подсказка получателя.
func (s *Service) GetOrder(ctx context.Context, ref order.Ref) (pickup.Sheet, error) {
	ctx, cancel := s.readContext(ctx)
	defer cancel()

	o, refs, err := s.loadOrder(ctx, ref)
	if err != nil {
		return pickup.Sheet{}, err
	}
	customer, err := s.d.Catalog.Customer(ctx, o.Customer)
	if err != nil && !errors.Is(err, usecase.ErrNotFound) {
		return pickup.Sheet{}, err
	}
	balances, err := s.d.Stock.Balances(ctx, o.Warehouse)
	if err != nil {
		return pickup.Sheet{}, err
	}
	reservations, err := s.d.Stock.Reservations(ctx, o.Warehouse)
	if err != nil {
		return pickup.Sheet{}, err
	}
	return pickup.ComposeSheet(o, refs, customer, stock.Positions(o, balances, reservations))
}

// loadOrder читает заказ и справочники, нужные для расчёта его строк.
func (s *Service) loadOrder(ctx context.Context, ref order.Ref) (order.Order, pickup.References, error) {
	o, err := s.d.Orders.GetOrder(ctx, ref)
	if errors.Is(err, usecase.ErrNotFound) {
		return order.Order{}, pickup.References{}, usecase.ErrOrderNotFound
	}
	if err != nil {
		return order.Order{}, pickup.References{}, err
	}

	products := make([]order.ProductRef, 0, len(o.Lines))
	rates := make([]vat.RateRef, 0, len(o.Lines))
	for _, l := range o.ActiveLines() {
		products = append(products, l.Product)
		rates = append(rates, l.VatRate)
	}

	refs := pickup.References{}
	if refs.Products, err = s.d.Catalog.Products(ctx, products); err != nil {
		return order.Order{}, pickup.References{}, err
	}
	if refs.VatRates, err = s.d.Catalog.VatRates(ctx, rates); err != nil {
		return order.Order{}, pickup.References{}, err
	}
	refs.Warehouse, err = s.d.Catalog.Warehouse(ctx, o.Warehouse)
	if errors.Is(err, usecase.ErrNotFound) {
		return order.Order{}, pickup.References{}, &pickup.MissingReferenceError{Kind: "склад", Ref: string(o.Warehouse)}
	}
	if err != nil {
		return order.Order{}, pickup.References{}, err
	}
	return o, refs, nil
}
