// Package money — точные числа и суммы: без float, чтобы не терять копейку
// на округлении (0.1+0.2 ≠ 0.3 в float64).
package money

import (
	"errors"
	"fmt"
)

const (
	maxScale         = 6 // знаков после запятой; у цен в 1С два, у количества до трёх
	maxIntegerDigits = 9 // цифр в целой части — сумма тысяч таких чисел влезает в int64
)

var (
	ErrInvalidDecimal  = errors.New("не число")
	ErrTooPrecise      = fmt.Errorf("больше %d знаков после запятой", maxScale)
	ErrOutOfRange      = fmt.Errorf("больше %d цифр в целой части", maxIntegerDigits)
	ErrAmountOverflow  = errors.New("сумма не помещается в разрядность")
	ErrZeroDenominator = errors.New("деление на ноль")
)

// Decimal — точное число coef / 10^scale, нормализованное (1.50 = 1.5).
type Decimal struct {
	coef  int64
	scale int
}

// Zero — ноль, удобен как начальное значение для сумм.
var Zero = Decimal{}

// Amount — сумма в целых копейках; в рубли переводится только при форматировании.
type Amount int64
