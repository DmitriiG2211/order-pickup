package shipment

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"orderissue/internal/domain/order"
	"orderissue/internal/domain/pickup"
)

// MarkerFor — метка в комментарии черновика, по которой мы узнаём свой черновик.
//
// Своего ключа 1С не принимает: переданный Ref_Key эмулятор выбрасывает и
// выдаёт свой, а повторный POST создаёт дубль (проверено на живой базе).
// База общая, чужие черновики по тому же заказу видны. Поэтому черновик
// помечается ключом заказа: один заказ — один наш черновик, сколько бы раз
// ни нажали кнопку и сколько бы раз ни повторили запрос после сбоя.
func MarkerFor(ref order.Ref) string {
	return "[pickup-service заказ " + string(ref) + "]"
}

// NewDraft готовит черновик по заказу. Получателя функция не принимает:
// его данные по заданию в 1С не пишутся, и сигнатура это гарантирует.
func NewDraft(o order.Order, lines []pickup.PricedLine, now time.Time) Draft {
	return Draft{
		Order:   o,
		Date:    now,
		Comment: fmt.Sprintf("Выдача со склада по заказу %s %s", o.Number, MarkerFor(o.Ref)),
		Lines:   lines,
	}
}

// FindOurs ищет наш черновик среди черновиков по заказу: с нашим маркером
// и без пометки на удаление. Если наших несколько — например, дубль остался
// от запуска до появления защиты, — возвращает самый ранний: он и был отметкой.
func FindOurs(ref order.Ref, candidates []Recorded) (Recorded, bool) {
	marker := MarkerFor(ref)
	var ours []Recorded
	for _, c := range candidates {
		if c.OrderRef == ref && !c.DeletionMark && strings.Contains(c.Comment, marker) {
			ours = append(ours, c)
		}
	}
	if len(ours) == 0 {
		return Recorded{}, false
	}
	return slices.MinFunc(ours, func(a, b Recorded) int {
		if c := a.Date.Compare(b.Date); c != 0 {
			return c
		}
		return strings.Compare(a.Number, b.Number)
	}), true
}

func (m Mismatch) String() string {
	where := "шапка"
	if m.Line > 0 {
		where = "строка " + strconv.Itoa(m.Line)
	}
	return fmt.Sprintf("%s, %s: ожидали %s, в 1С %s", where, m.Field, m.Want, m.Got)
}

// Verify сверяет черновик, перечитанный из 1С, с отправленным. Ответ 201
// на POST говорит только, что 1С что-то записала; что именно — видно лишь
// при чтении. Возвращает все расхождения, а не первое: так проще разбираться.
func Verify(d Draft, r Recorded) []Mismatch {
	var mm []Mismatch
	add := func(field string, line int, want, got string) {
		if want != got {
			mm = append(mm, Mismatch{Field: field, Line: line, Want: want, Got: got})
		}
	}

	add("заказ", 0, string(d.Order.Ref), string(r.OrderRef))
	if !strings.Contains(r.Comment, MarkerFor(d.Order.Ref)) {
		mm = append(mm, Mismatch{Field: "маркер в комментарии", Want: MarkerFor(d.Order.Ref), Got: r.Comment})
	}
	add("количество строк", 0, strconv.Itoa(len(d.Lines)), strconv.Itoa(len(r.Lines)))

	for i := range min(len(d.Lines), len(r.Lines)) {
		want, got, n := d.Lines[i], r.Lines[i], i+1
		add("товар", n, string(want.Product), string(got.Product))
		add("количество", n, want.Quantity.String(), got.Quantity.String())
		add("цена", n, want.Price.String(), got.Price.String())
		add("ставка НДС", n, string(want.VatRate), string(got.VatRate))
		add("сумма", n, want.Sums.Amount.String(), got.Sums.Amount.String())
		add("НДС", n, want.Sums.VAT.String(), got.Sums.VAT.String())
		add("сумма с НДС", n, want.Sums.WithVAT.String(), got.Sums.WithVAT.String())
	}
	return mm
}
