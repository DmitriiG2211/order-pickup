// Package shipment — черновик реализации товаров и услуг по заказу:
// «отметиться в 1С» после выдачи.
package shipment

import (
	"time"

	"orderissue/internal/domain/money"
	"orderissue/internal/domain/order"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/domain/vat"
)

// Draft — черновик, который отправляем в 1С.
type Draft struct {
	Order   order.Order
	Date    time.Time
	Comment string
	Lines   []pickup.PricedLine
}

// Recorded — черновик, как его прочитали из 1С.
type Recorded struct {
	Ref          string
	Number       string
	Date         time.Time
	Posted       bool
	DeletionMark bool
	OrderRef     order.Ref
	Comment      string
	Lines        []RecordedLine
}

// RecordedLine — строка записанного черновика.
type RecordedLine struct {
	Product  order.ProductRef
	Quantity money.Decimal
	Price    money.Decimal
	VatRate  vat.RateRef
	Sums     vat.Line
}

// Mismatch — расхождение записанного черновика с тем, что мы отправляли.
type Mismatch struct {
	Field string // что разошлось
	Line  int    // номер строки; 0 — шапка документа
	Want  string
	Got   string
}
