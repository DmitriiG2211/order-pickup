package money

import (
	"math/big"
	"strings"
)

// AmountFromDecimal переводит рубли в копейки без округления; больше двух знаков — ошибка.
func AmountFromDecimal(d Decimal) (Amount, error) {
	if d.scale > 2 {
		return 0, ErrTooPrecise
	}
	return Amount(d.bigAt(2).Int64()), nil
}

// Multiply — сумма строки (количество × цена) в копейках, округление половины вверх.
func Multiply(a, b Decimal) (Amount, error) {
	// a × b рублей = a.coef × b.coef / 10^(a.scale+b.scale); в копейках ещё × 100.
	numerator := new(big.Int).Mul(big.NewInt(a.coef), big.NewInt(b.coef))
	numerator.Mul(numerator, big.NewInt(100))
	return toAmount(divRoundHalfUp(numerator, pow10(a.scale+b.scale)))
}

// Share — amount × numerator / denominator, округление половины вверх.
// НДС сверху: Share(сумма, ставка, 100). НДС внутри: Share(сумма, ставка, 100+ставка).
func Share(amount Amount, numerator, denominator Decimal) (Amount, error) {
	if denominator.Sign() == 0 {
		return 0, ErrZeroDenominator
	}
	// amount × (n.coef / 10^n.scale) / (d.coef / 10^d.scale)
	// = amount × n.coef × 10^d.scale / (d.coef × 10^n.scale)
	top := new(big.Int).Mul(big.NewInt(int64(amount)), big.NewInt(numerator.coef))
	top.Mul(top, pow10(denominator.scale))
	bottom := new(big.Int).Mul(big.NewInt(denominator.coef), pow10(numerator.scale))
	return toAmount(divRoundHalfUp(top, bottom))
}

// String печатает сумму в рублях: "3750", "435.82" — без копеек, если их нет.
func (a Amount) String() string {
	return formatScaled(int64(a), 2, 2)
}

// Rubles печатает сумму всегда с копейками: "3750.00". Для человека — в PDF и на экране.
func (a Amount) Rubles() string {
	s := a.String()
	if !strings.Contains(s, ".") {
		s += ".00"
	}
	return s
}

// divRoundHalfUp делит с округлением половины от нуля: 1,265 → 1,27; −1,265 → −1,27.
func divRoundHalfUp(numerator, denominator *big.Int) *big.Int {
	quotient, remainder := new(big.Int).QuoRem(numerator, denominator, new(big.Int))
	twiceRemainder := new(big.Int).Abs(remainder)
	twiceRemainder.Mul(twiceRemainder, big.NewInt(2))
	if twiceRemainder.Cmp(new(big.Int).Abs(denominator)) >= 0 {
		if numerator.Sign()*denominator.Sign() < 0 {
			quotient.Sub(quotient, big.NewInt(1))
		} else {
			quotient.Add(quotient, big.NewInt(1))
		}
	}
	return quotient
}

func toAmount(kopecks *big.Int) (Amount, error) {
	if !kopecks.IsInt64() {
		return 0, ErrAmountOverflow
	}
	return Amount(kopecks.Int64()), nil
}
