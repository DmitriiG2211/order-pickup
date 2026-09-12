// Package vat — расчёт строки документа и НДС по правилам задания.
package vat

import "orderissue/internal/domain/money"

// RateRef — ссылка на ставку в справочнике 1С (GUID).
type RateRef string

// Rate — ставка НДС из справочника 1С «СтавкиНДС».
//
// Процент берём из поля «Ставка», а не из названия: у «22%» и «22/122» он
// одинаковый, 22, а в справочнике встречаются и нетиповые ставки вроде 13%.
type Rate struct {
	Name    string
	Percent money.Decimal
}

// Method — как НДС соотносится с суммой строки.
type Method int

const (
	AddedOnTop Method = iota + 1 // цена без НДС, налог начисляется сверху
	Included                     // цена уже содержит НДС, налог выделяется из суммы
)

// Line — суммы одной строки.
type Line struct {
	// Amount — количество × цена, округлённое до копейки. Когда цена включает
	// НДС, это сумма с налогом внутри, а не база без налога.
	Amount  money.Amount
	VAT     money.Amount
	WithVAT money.Amount
}
