package pickup

import (
	"fmt"
	"strings"

	"orderissue/internal/domain/order"
	"orderissue/internal/domain/person"
	"orderissue/internal/domain/stock"
	"orderissue/internal/domain/vat"
)

func (e *MissingReferenceError) Error() string {
	return fmt.Sprintf("строка %d: %s %s не найдена в справочнике 1С", e.LineNumber, e.Kind, e.Ref)
}

// ComposeDocument собирает документ на выдачу. Помеченный на удаление или
// непроведённый заказ тоже собирается — решение, печатать ли его, принимает
// сценарий, а причины лежат в BlockReasons.
func ComposeDocument(o order.Order, refs References, r Receiver, inflector person.NameInflector) (Document, error) {
	lines, totals, err := PriceLines(o, refs)
	if err != nil {
		return Document{}, err
	}
	return Document{
		OrderRef:         o.Ref,
		OrderNumber:      o.Number,
		OrderDate:        o.Date,
		WarehouseName:    refs.Warehouse.Name,
		PriceIncludesVAT: o.PriceIncludesVAT,
		ReceiverGenitive: inflector.Genitive(r.Name, r.Gender),
		Phone:            r.Phone,
		Email:            r.Email,
		Lines:            lines,
		Totals:           totals,
		BlockReasons:     o.BlockReasons(),
	}, nil
}

// ComposeSheet собирает экран заказа.
func ComposeSheet(o order.Order, refs References, customer order.Customer, positions map[stock.Item]stock.Position) (Sheet, error) {
	priced, totals, err := PriceLines(o, refs)
	if err != nil {
		return Sheet{}, err
	}
	lines := make([]SheetLine, 0, len(priced))
	for _, pl := range priced {
		item := stock.Item{Product: pl.Product, Characteristic: pl.Characteristic}
		lines = append(lines, SheetLine{PricedLine: pl, Stock: positions[item]})
	}
	return Sheet{
		Order:             o,
		WarehouseName:     refs.Warehouse.Name,
		Customer:          customer,
		Lines:             lines,
		Totals:            totals,
		SuggestedReceiver: SuggestReceiver(customer),
	}, nil
}

// SuggestReceiver разбирает наименование клиента-физлица на ФИО.
// У организации («ООО «Ромашка»») и у наименования не из 2–3 слов подсказки нет.
func SuggestReceiver(c order.Customer) *SuggestedReceiver {
	if !c.IsPerson {
		return nil
	}
	words := strings.Fields(c.Name)
	if len(words) < 2 || len(words) > 3 {
		return nil
	}
	s := &SuggestedReceiver{Name: person.FullName{Last: words[0], First: words[1]}}
	if len(words) == 3 {
		s.Name.Middle = words[2]
		s.Gender, _ = person.GenderFromPatronymic(words[2])
	}
	return s
}

// PriceLines считает активные строки заказа и итоги по правилам задания.
func PriceLines(o order.Order, refs References) ([]PricedLine, vat.Line, error) {
	method := vat.MethodFor(o.PriceIncludesVAT)
	active := o.ActiveLines()
	lines := make([]PricedLine, 0, len(active))
	sums := make([]vat.Line, 0, len(active))

	for _, l := range active {
		product, ok := refs.Products[l.Product]
		if !ok {
			return nil, vat.Line{}, &MissingReferenceError{Kind: "номенклатура", Ref: string(l.Product), LineNumber: l.Number}
		}
		rate, ok := refs.VatRates[l.VatRate]
		if !ok {
			return nil, vat.Line{}, &MissingReferenceError{Kind: "ставка НДС", Ref: string(l.VatRate), LineNumber: l.Number}
		}
		s, err := vat.CalculateLine(l.Quantity, l.Price, rate, method)
		if err != nil {
			return nil, vat.Line{}, fmt.Errorf("строка %d: %w", l.Number, err)
		}
		lines = append(lines, PricedLine{Line: l, Article: product.Article, Name: product.Name, VatName: rate.Name, Sums: s})
		sums = append(sums, s)
	}
	return lines, vat.Total(sums), nil
}
