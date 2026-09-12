// Package domaintest — данные демо-базы 1С в виде доменных объектов для тестов.
package domaintest

import (
	"time"

	"orderissue/internal/domain/money"
	"orderissue/internal/domain/order"
)

// Order возвращает заказ демо-базы: domaintest.Order(domaintest.Order101).
func Order(ref order.Ref) order.Order {
	o, ok := Orders()[ref]
	if !ok {
		panic("domaintest: нет заказа " + string(ref))
	}
	return o
}

// Decimal разбирает число для тестов и падает на опечатке в самом тесте.
func Decimal(s string) money.Decimal {
	return d(s)
}

func d(s string) money.Decimal {
	v, err := money.ParseDecimal(s)
	if err != nil {
		panic(err)
	}
	return v
}

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		panic(err)
	}
	return t
}
