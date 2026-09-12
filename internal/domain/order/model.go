// Package order — заказ клиента, каким он нужен для выдачи со склада.
package order

import (
	"errors"
	"regexp"
	"time"

	"orderissue/internal/domain/money"
	"orderissue/internal/domain/vat"
)

// Ссылки на объекты 1С — GUID. Отдельный тип на каждый вид объекта не даёт
// перепутать ключ товара с ключом склада: в ответах 1С они выглядят одинаково.
type (
	Ref               string // заказ клиента
	ProductRef        string // номенклатура
	CharacteristicRef string // характеристика номенклатуры
	WarehouseRef      string // склад
	PartnerRef        string // партнёр (клиент)
)

var (
	ErrInvalidRef = errors.New("ожидается GUID вида 2da145f0-d5fc-11f1-a0b3-48df371887e9")
	guidPattern   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// BlockReason — почему заказ нельзя выдавать.
type BlockReason string

const (
	BlockedByDeletionMark BlockReason = "deletion_mark"
	BlockedNotPosted      BlockReason = "not_posted"
)

// Line — строка заказа.
type Line struct {
	Number         int
	Product        ProductRef
	Characteristic CharacteristicRef
	Quantity       money.Decimal
	Price          money.Decimal
	VatRate        vat.RateRef
	Warehouse      WarehouseRef
	Cancelled      bool
}

// Accounting — реквизиты заказа, которые переносятся в черновик реализации.
// Домен их не интерпретирует, только передаёт дальше.
type Accounting struct {
	Counterparty string
	Organization string
	Currency     string
	Manager      string
	VATTaxation  string
}

// Order — заказ клиента.
type Order struct {
	Ref              Ref
	Number           string
	Date             time.Time
	Posted           bool
	DeletionMark     bool
	Status           string
	PriceIncludesVAT bool
	Warehouse        WarehouseRef
	Customer         PartnerRef
	Accounting       Accounting
	Lines            []Line
}

// Product — карточка номенклатуры.
type Product struct {
	Ref     ProductRef
	Article string
	Name    string
}

// Warehouse — склад.
type Warehouse struct {
	Ref  WarehouseRef
	Name string
}

// Customer — клиент заказа (партнёр).
type Customer struct {
	Ref      PartnerRef
	Name     string
	IsPerson bool
}
