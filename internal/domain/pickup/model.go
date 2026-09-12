// Package pickup — выдача заказа при самовывозе: кто приехал за товаром
// и документ на выдачу.
package pickup

import (
	"time"

	"orderissue/internal/domain/order"
	"orderissue/internal/domain/person"
	"orderissue/internal/domain/stock"
	"orderissue/internal/domain/vat"
)

// Field — поле формы получателя. Значения совпадают с именами полей
// в контракте задания, чтобы ошибка указывала ровно на то поле, которое видит человек.
type Field string

const (
	FieldLastName   Field = "lastName"
	FieldFirstName  Field = "firstName"
	FieldMiddleName Field = "middleName"
	FieldGender     Field = "gender"
	FieldPhone      Field = "phone"
	FieldEmail      Field = "email"
)

const maxNameLength = 100

// FieldErrors — все ошибки ввода сразу, по полям. Показывать по одной
// заставило бы человека отправлять форму столько раз, сколько в ней ошибок.
type FieldErrors map[Field]string

// ReceiverInput — данные формы как их ввёл человек.
type ReceiverInput struct {
	LastName   string
	FirstName  string
	MiddleName string
	Gender     string
	Phone      string
	Email      string
}

// Receiver — проверенный получатель. Живёт только в запросе:
// по заданию его данные нигде не сохраняются.
type Receiver struct {
	Name   person.FullName
	Gender person.Gender
	Phone  string // +79991234567
	Email  string
}

// References — справочные данные 1С, без которых строку не посчитать.
type References struct {
	Products  map[order.ProductRef]order.Product
	VatRates  map[vat.RateRef]vat.Rate
	Warehouse order.Warehouse
}

// MissingReferenceError — строка заказа ссылается на то, чего нет в справочнике.
// Считать такую строку с нулевой ставкой или пустым наименованием нельзя:
// документ молча разошёлся бы с 1С.
type MissingReferenceError struct {
	Kind       string // «ставка НДС», «номенклатура»
	Ref        string
	LineNumber int
}

// PricedLine — строка заказа с посчитанными суммами.
type PricedLine struct {
	order.Line
	Article string
	Name    string
	VatName string
	Sums    vat.Line
}

// Document — документ на выдачу: то, что уходит в PDF и в JSON для проверки.
type Document struct {
	OrderRef         order.Ref
	OrderNumber      string
	OrderDate        time.Time
	WarehouseName    string
	PriceIncludesVAT bool
	ReceiverGenitive person.FullName
	Phone            string
	Email            string
	Lines            []PricedLine
	Totals           vat.Line
	BlockReasons     []order.BlockReason
}

// SheetLine — строка экрана заказа: суммы и остаток.
type SheetLine struct {
	PricedLine
	Stock stock.Position
}

// Sheet — экран заказа для кладовщика, до ввода получателя.
type Sheet struct {
	Order         order.Order
	WarehouseName string
	Customer      order.Customer
	Lines         []SheetLine
	Totals        vat.Line
	// SuggestedReceiver — ФИО из карточки клиента-физлица, чтобы не вводить
	// руками: задание само говорит, что подходящее ФИО уже есть в карточке.
	SuggestedReceiver *SuggestedReceiver
}

// SuggestedReceiver — подсказка для формы получателя.
type SuggestedReceiver struct {
	Name   person.FullName
	Gender person.Gender // 0, если по отчеству не определить
}
