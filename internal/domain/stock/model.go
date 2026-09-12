// Package stock — чем склад располагает под конкретный заказ.
package stock

import (
	"orderissue/internal/domain/money"
	"orderissue/internal/domain/order"
)

// Item — товар с характеристикой: остатки в 1С ведутся в этом разрезе.
type Item struct {
	Product        order.ProductRef
	Characteristic order.CharacteristicRef
}

// Balance — сколько товара физически лежит на складе
// (регистр «ТоварыНаСкладах», ресурс «ВНаличии»).
type Balance struct {
	Item      Item
	Warehouse order.WarehouseRef
	OnHand    money.Decimal
}

// Reservation — сколько товара на складе обещано документу отгрузки
// (регистр «ТоварыКОтгрузке»).
type Reservation struct {
	Item      Item
	Warehouse order.WarehouseRef
	Document  string // Ref документа отгрузки; не обязательно заказ
	Quantity  money.Decimal
}

// Position — картина по одному товару заказа.
//
// Главное число — OnHand: кладовщик отдаёт товар с полки, и ему важно, сколько
// там реально лежит. «Свободный» остаток (наличие минус все резервы) вычел бы
// резерв этого же заказа из него самого и показал бы ноль там, где товар есть.
// Чужие резервы показываем рядом, чтобы было видно, что отдаём чужое.
// Подробно — в ADR 0003.
type Position struct {
	OnHand           money.Decimal // лежит на складе заказа
	Needed           money.Decimal // нужно по всем строкам заказа с этим товаром
	Shortage         money.Decimal // не хватает: max(0, Needed − OnHand)
	ReservedByOthers money.Decimal // обещано другим документам на этом складе
}
