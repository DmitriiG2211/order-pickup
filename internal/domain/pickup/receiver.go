package pickup

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"orderissue/internal/domain/person"
)

// Error перечисляет только имена полей. Тексты ошибок содержат куски ввода
// («в домене «example» нет точки»), а Error() легко попадает в лог —
// данные получателя туда попасть не должны. Тексты отдаются клиенту отдельно.
func (e FieldErrors) Error() string {
	fields := make([]string, 0, len(e))
	for f := range e {
		fields = append(fields, string(f))
	}
	sort.Strings(fields)
	return "неверные данные получателя в полях: " + strings.Join(fields, ", ")
}

// NewReceiver проверяет форму и возвращает получателя или FieldErrors со всеми ошибками.
func NewReceiver(in ReceiverInput) (Receiver, error) {
	errs := FieldErrors{}
	r := Receiver{
		Name: person.FullName{
			Last:   checkName(errs, FieldLastName, in.LastName, "Укажите фамилию"),
			First:  checkName(errs, FieldFirstName, in.FirstName, "Укажите имя"),
			Middle: checkName(errs, FieldMiddleName, in.MiddleName, ""),
		},
		Phone: checkPhone(errs, in.Phone),
		Email: checkEmail(errs, in.Email),
	}
	r.Gender = checkGender(errs, in.Gender, r.Name.Middle)

	if len(errs) > 0 {
		return Receiver{}, errs
	}
	return r, nil
}

// checkName проверяет часть ФИО. Пустое requiredMessage означает, что поле необязательное.
func checkName(errs FieldErrors, field Field, raw, requiredMessage string) string {
	value := strings.Join(strings.Fields(raw), " ")
	if value == "" {
		if requiredMessage != "" {
			errs[field] = requiredMessage
		}
		return ""
	}
	if n := utf8.RuneCountInString(value); n > maxNameLength {
		errs[field] = fmt.Sprintf("Не длиннее %d символов, сейчас %d", maxNameLength, n)
		return ""
	}
	for _, r := range value {
		if !unicode.IsLetter(r) && r != '-' && r != '\'' && r != ' ' {
			errs[field] = fmt.Sprintf("Допустимы буквы, дефис, апостроф и пробел, а здесь есть «%c»", r)
			return ""
		}
	}
	first, _ := utf8.DecodeRuneInString(value)
	last, _ := utf8.DecodeLastRuneInString(value)
	if !unicode.IsLetter(first) || !unicode.IsLetter(last) {
		errs[field] = "Должно начинаться и заканчиваться буквой"
		return ""
	}
	return value
}

func checkGender(errs FieldErrors, raw, middleName string) person.Gender {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "м":
		return person.Male
	case "ж":
		return person.Female
	case "":
		// Пол не пришёл — задание разрешает определить его по отчеству.
	default:
		errs[FieldGender] = fmt.Sprintf("Допустимые значения «м» или «ж», получено «%s»", strings.TrimSpace(raw))
		return 0
	}

	if _, middleInvalid := errs[FieldMiddleName]; middleInvalid {
		return 0 // об ошибке уже сказано у отчества
	}
	if middleName == "" {
		errs[FieldGender] = "Укажите пол: отчества нет, определить его не из чего"
		return 0
	}
	gender, ok := person.GenderFromPatronymic(middleName)
	if !ok {
		errs[FieldGender] = fmt.Sprintf("Укажите пол: по отчеству «%s» его не определить", middleName)
		return 0
	}
	return gender
}

// checkPhone принимает российский номер в любой привычной записи
// («8 (999) 123-45-67», «+7 999 123 45 67», «9991234567») и приводит его к +7XXXXXXXXXX.
func checkPhone(errs FieldErrors, raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		errs[FieldPhone] = "Укажите телефон"
		return ""
	}

	var digits strings.Builder
	for i, r := range value {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r == '+' && i == 0:
		case r == '+':
			errs[FieldPhone] = "«+» может стоять только в начале номера"
			return ""
		case r == ' ' || r == '(' || r == ')' || r == '-':
		default:
			errs[FieldPhone] = fmt.Sprintf("Допустимы цифры, пробелы, скобки, дефис и «+», а здесь есть «%c»", r)
			return ""
		}
	}

	d := digits.String()
	hasPlus := strings.HasPrefix(value, "+")
	switch {
	case len(d) == 11 && d[0] == '7':
		return "+" + d
	case len(d) == 11 && d[0] == '8' && !hasPlus:
		return "+7" + d[1:]
	case len(d) == 10 && !hasPlus:
		return "+7" + d
	case hasPlus && len(d) > 0 && d[0] != '7':
		errs[FieldPhone] = "Поддерживаются только российские номера: после «+» должен идти код 7"
	default:
		errs[FieldPhone] = fmt.Sprintf("Нужно 11 цифр: «+7» или «8» и ещё 10 цифр, а введено %d", len(d))
	}
	return ""
}

func checkEmail(errs FieldErrors, raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		errs[FieldEmail] = "Укажите почту"
		return ""
	}
	if strings.ContainsFunc(value, unicode.IsSpace) {
		errs[FieldEmail] = "В адресе почты не должно быть пробелов"
		return ""
	}

	local, domain, found := strings.Cut(value, "@")
	switch {
	case !found:
		errs[FieldEmail] = "Нет символа «@»"
	case strings.Contains(domain, "@"):
		errs[FieldEmail] = "Символ «@» должен быть один"
	case local == "":
		errs[FieldEmail] = "Нет имени до «@»"
	case domain == "":
		errs[FieldEmail] = "Нет домена после «@»"
	case !strings.Contains(domain, "."):
		errs[FieldEmail] = fmt.Sprintf("В домене «%s» нет точки", domain)
	case strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || strings.Contains(domain, ".."):
		errs[FieldEmail] = fmt.Sprintf("Домен «%s» записан с ошибкой в точках", domain)
	default:
		return value
	}
	return ""
}
