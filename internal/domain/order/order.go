package order

import "strings"

// ParseRef проверяет, что строка — GUID заказа. Номер документа («00ДМ-000101»)
// сюда не подходит: задание просит именно Ref_Key.
func ParseRef(s string) (Ref, error) {
	normalized := strings.ToLower(strings.TrimSpace(s))
	if !guidPattern.MatchString(normalized) {
		return "", ErrInvalidRef
	}
	return Ref(normalized), nil
}

// ActiveLines — строки без отменённых. Отменённая строка не выдаётся
// и не входит ни в документ, ни в итоги.
func (o Order) ActiveLines() []Line {
	active := make([]Line, 0, len(o.Lines))
	for _, l := range o.Lines {
		if !l.Cancelled {
			active = append(active, l)
		}
	}
	return active
}

// BlockReasons перечисляет, почему заказ нельзя выдавать. Пустой список — можно.
// Помеченный на удаление заказ клиенту не отдают, а непроведённый ещё не
// дошёл до склада: резервов под него нет, и выдача разошлась бы с учётом.
func (o Order) BlockReasons() []BlockReason {
	var reasons []BlockReason
	if o.DeletionMark {
		reasons = append(reasons, BlockedByDeletionMark)
	}
	if !o.Posted {
		reasons = append(reasons, BlockedNotPosted)
	}
	return reasons
}

// Issuable сообщает, можно ли выдавать заказ.
func (o Order) Issuable() bool {
	return len(o.BlockReasons()) == 0
}
