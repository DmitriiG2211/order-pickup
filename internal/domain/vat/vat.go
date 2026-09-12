package vat

import (
	"errors"
	"fmt"
	"strings"

	"orderissue/internal/domain/money"
)

var (
	ErrInvalidPercent    = errors.New("ставка НДС должна быть от 0 до 100")
	ErrNegativeLineInput = errors.New("количество и цена не могут быть отрицательными")
)

var hundred = money.NewInt(100)

func NewRate(name string, percent money.Decimal) (Rate, error) {
	if percent.Sign() < 0 || percent.Cmp(money.NewInt(100)) >= 0 {
		return Rate{}, fmt.Errorf("%s: %w", name, ErrInvalidPercent)
	}
	// Description из OData иногда приходит с пробелом на конце.
	return Rate{Name: strings.TrimSpace(name), Percent: percent}, nil
}

// MethodFor выбирает способ по флагу заказа «ЦенаВключаетНДС».
// Название ставки роли не играет: «22/122» в заказе без флага начисляется
// сверху — так в эталоне задания (заказ 00ДМ-000101, строка ДМ-0002).
func MethodFor(priceIncludesVAT bool) Method {
	if priceIncludesVAT {
		return Included
	}
	return AddedOnTop
}

// CalculateLine считает строку: сначала сумму с округлением до копейки,
// затем НДС от уже округлённой суммы. Обратный порядок даёт расхождение
// на копейку (см. тест про цену 2,2451).
func CalculateLine(quantity, price money.Decimal, rate Rate, method Method) (Line, error) {
	if quantity.Sign() < 0 || price.Sign() < 0 {
		return Line{}, ErrNegativeLineInput
	}

	amount, err := money.Multiply(quantity, price)
	if err != nil {
		return Line{}, fmt.Errorf("сумма строки: %w", err)
	}

	switch method {
	case Included:
		tax, err := money.Share(amount, rate.Percent, hundred.Add(rate.Percent))
		if err != nil {
			return Line{}, fmt.Errorf("НДС в сумме: %w", err)
		}
		return Line{Amount: amount, VAT: tax, WithVAT: amount}, nil
	default:
		tax, err := money.Share(amount, rate.Percent, hundred)
		if err != nil {
			return Line{}, fmt.Errorf("НДС сверху: %w", err)
		}
		return Line{Amount: amount, VAT: tax, WithVAT: amount + tax}, nil
	}
}

// Total — итоги документа: сумма уже округлённых строк. Пересчёт НДС от общей
// суммы дал бы другое число, и задание прямо это запрещает.
func Total(lines []Line) Line {
	var total Line
	for _, l := range lines {
		total.Amount += l.Amount
		total.VAT += l.VAT
		total.WithVAT += l.WithVAT
	}
	return total
}
