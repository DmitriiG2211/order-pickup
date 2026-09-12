package money

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// NewInt возвращает целое число.
func NewInt(n int64) Decimal {
	return Decimal{coef: n}
}

// ParseDecimal разбирает десятичную запись числа так, как её отдаёт JSON:
// "1250", "990.5", "-3", "0.125", "2E-05".
func ParseDecimal(s string) (Decimal, error) {
	mantissa, exponent, err := splitExponent(s)
	if err != nil {
		return Decimal{}, fmt.Errorf("%q: %w", s, err)
	}

	negative := strings.HasPrefix(mantissa, "-")
	mantissa = strings.TrimPrefix(mantissa, "-")

	intPart, fracPart, hasPoint := strings.Cut(mantissa, ".")
	if intPart == "" || (hasPoint && fracPart == "") || !isDigits(intPart) || !isDigits(fracPart) {
		return Decimal{}, fmt.Errorf("%q: %w", s, ErrInvalidDecimal)
	}

	digits := strings.TrimLeft(intPart+fracPart, "0")
	scale := len(fracPart) - exponent
	for scale < 0 { // экспонента больше дробной части: 1.5e2 = 150
		digits += "0"
		scale++
	}
	for scale > 0 && strings.HasSuffix(digits, "0") {
		digits = digits[:len(digits)-1]
		scale--
	}

	if digits == "" {
		return Zero, nil
	}
	if scale > maxScale {
		return Decimal{}, fmt.Errorf("%q: %w", s, ErrTooPrecise)
	}
	if len(digits)-scale > maxIntegerDigits {
		return Decimal{}, fmt.Errorf("%q: %w", s, ErrOutOfRange)
	}

	// После проверок выше в digits не больше 15 цифр, int64 их вмещает.
	coef, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return Decimal{}, fmt.Errorf("%q: %w", s, ErrOutOfRange)
	}
	if negative {
		coef = -coef
	}
	return Decimal{coef: coef, scale: scale}, nil
}

// Sign возвращает -1, 0 или 1.
func (d Decimal) Sign() int {
	switch {
	case d.coef < 0:
		return -1
	case d.coef > 0:
		return 1
	default:
		return 0
	}
}

// Cmp сравнивает числа: -1, если d < o; 0, если равны; 1, если d > o.
func (d Decimal) Cmp(o Decimal) int {
	s := max(d.scale, o.scale)
	return d.bigAt(s).Cmp(o.bigAt(s))
}

// Add возвращает d + o.
func (d Decimal) Add(o Decimal) Decimal {
	s := max(d.scale, o.scale)
	return fromBig(new(big.Int).Add(d.bigAt(s), o.bigAt(s)), s)
}

// Sub возвращает d - o.
func (d Decimal) Sub(o Decimal) Decimal {
	s := max(d.scale, o.scale)
	return fromBig(new(big.Int).Sub(d.bigAt(s), o.bigAt(s)), s)
}

// String печатает число без лишних нулей: "1250", "990.5", "0.125".
func (d Decimal) String() string {
	return formatScaled(d.coef, d.scale, 0)
}

// bigAt возвращает коэффициент, приведённый к масштабу s (s >= d.scale).
func (d Decimal) bigAt(s int) *big.Int {
	return new(big.Int).Mul(big.NewInt(d.coef), pow10(s-d.scale))
}

// fromBig собирает Decimal обратно. При ограничениях ParseDecimal переполнения
// быть не может, поэтому это нарушение инварианта, а не ошибка ввода.
func fromBig(coef *big.Int, scale int) Decimal {
	if !coef.IsInt64() {
		panic("money: переполнение Decimal, нарушены ограничения ParseDecimal")
	}
	value := coef.Int64()
	for scale > 0 && value%10 == 0 {
		value /= 10
		scale--
	}
	if value == 0 {
		return Zero
	}
	return Decimal{coef: value, scale: scale}
}

func splitExponent(s string) (mantissa string, exponent int, err error) {
	idx := strings.IndexAny(s, "eE")
	if idx < 0 {
		return s, 0, nil
	}
	exponent, convErr := strconv.Atoi(s[idx+1:])
	if convErr != nil || exponent > 30 || exponent < -30 {
		return "", 0, ErrInvalidDecimal
	}
	return s[:idx], exponent, nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func pow10(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// formatScaled печатает coef / 10^scale без нулей в конце дробной части.
// Если дробная часть есть, дополняет её нулями до minFraction знаков.
func formatScaled(coef int64, scale, minFraction int) string {
	sign := ""
	if coef < 0 {
		sign = "-"
		coef = -coef
	}
	digits := strconv.FormatInt(coef, 10)
	if scale == 0 {
		return sign + digits
	}
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale-len(digits)+1) + digits
	}
	intPart, fracPart := digits[:len(digits)-scale], strings.TrimRight(digits[len(digits)-scale:], "0")
	if fracPart == "" {
		return sign + intPart
	}
	if len(fracPart) < minFraction {
		fracPart += strings.Repeat("0", minFraction-len(fracPart))
	}
	return sign + intPart + "." + fracPart
}
