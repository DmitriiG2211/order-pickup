package person

import "strings"

// String собирает ФИО через пробел, пропуская пустое отчество.
func (n FullName) String() string {
	parts := []string{n.Last, n.First}
	if n.Middle != "" {
		parts = append(parts, n.Middle)
	}
	return strings.Join(parts, " ")
}

// GenderFromPatronymic определяет пол по отчеству. Задание называет отчество
// надёжным признаком, и это так: окончание у отчества задаёт пол однозначно.
// По имени пол не угадываем: Саша, Женя, Валя бывают обоих полов.
func GenderFromPatronymic(middle string) (Gender, bool) {
	fields := strings.Fields(strings.ToLower(middle))
	if len(fields) == 0 {
		return 0, false
	}
	// Для «Гасан оглы» решает последнее слово.
	last := fields[len(fields)-1]
	switch {
	case strings.HasSuffix(last, "ич"), last == "оглы", last == "улы", last == "уулу":
		return Male, true
	case strings.HasSuffix(last, "на"), last == "кызы", last == "гызы":
		return Female, true
	default:
		return 0, false
	}
}
