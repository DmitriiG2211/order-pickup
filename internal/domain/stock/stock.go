package stock

import "orderissue/internal/domain/order"

// HasShortage сообщает, что товара на складе меньше, чем нужно по заказу.
func (p Position) HasShortage() bool {
	return p.Shortage.Sign() > 0
}

// Positions считает позицию для каждого товара заказа на складе заказа.
// Остатки других складов не учитываются: задание спрашивает про склад заказа.
func Positions(o order.Order, balances []Balance, reservations []Reservation) map[Item]Position {
	positions := map[Item]Position{}

	// Один товар может стоять в нескольких строках (заказ 00ДМ-000103, ручка):
	// сравнивать остаток нужно с их суммой, а не с каждой строкой отдельно.
	for _, l := range o.ActiveLines() {
		item := Item{Product: l.Product, Characteristic: l.Characteristic}
		p := positions[item]
		p.Needed = p.Needed.Add(l.Quantity)
		positions[item] = p
	}

	for _, b := range balances {
		p, ok := positions[b.Item]
		if !ok || b.Warehouse != o.Warehouse {
			continue
		}
		p.OnHand = p.OnHand.Add(b.OnHand)
		positions[b.Item] = p
	}

	for _, r := range reservations {
		p, ok := positions[r.Item]
		if !ok || r.Warehouse != o.Warehouse || r.Document == string(o.Ref) {
			continue
		}
		p.ReservedByOthers = p.ReservedByOthers.Add(r.Quantity)
		positions[r.Item] = p
	}

	for item, p := range positions {
		if gap := p.Needed.Sub(p.OnHand); gap.Sign() > 0 {
			p.Shortage = gap
		}
		positions[item] = p
	}
	return positions
}
