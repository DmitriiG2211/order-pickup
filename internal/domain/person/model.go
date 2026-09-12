// Package person — ФИО и пол человека, который приехал за товаром.
package person

// Gender — грамматический род. От него зависит склонение: «Седых Веры»,
// но «Седых Павла»; «Синицыной Ольги», но «Синицына Олега».
type Gender int

const (
	Male Gender = iota + 1
	Female
)

// FullName — фамилия, имя и отчество. Отчества может не быть.
type FullName struct {
	Last   string
	First  string
	Middle string
}

// NameInflector склоняет ФИО. Реализация — infrastructure/petrovich:
// правила склонения — внешние данные, домену нужен только результат.
type NameInflector interface {
	// Genitive возвращает ФИО в родительном падеже: «выдать кого?».
	Genitive(name FullName, gender Gender) FullName
}
