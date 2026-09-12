package pickup_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"orderissue/internal/domain/person"
	"orderissue/internal/domain/pickup"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validInput() pickup.ReceiverInput {
	return pickup.ReceiverInput{
		LastName:   "Иванов",
		FirstName:  "Иван",
		MiddleName: "Иванович",
		Phone:      "8 (999) 123-45-67",
		Email:      "i.ivanov@example.com",
	}
}

func fieldErrors(t *testing.T, err error) pickup.FieldErrors {
	t.Helper()
	var errs pickup.FieldErrors
	require.ErrorAs(t, err, &errs)
	return errs
}

func TestNewReceiver_ExampleFromTask_NormalizesPhoneAndDetectsGender(t *testing.T) {
	// Arrange: пример из задания, пол не передан
	in := validInput()

	// Act
	r, err := pickup.NewReceiver(in)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, person.FullName{Last: "Иванов", First: "Иван", Middle: "Иванович"}, r.Name)
	assert.Equal(t, person.Male, r.Gender)
	assert.Equal(t, "+79991234567", r.Phone)
	assert.Equal(t, "i.ivanov@example.com", r.Email)
}

func TestNewReceiver_ExplicitGender_WinsOverPatronymic(t *testing.T) {
	// Arrange: проверка задания всегда передаёт пол — ему и доверяем
	in := validInput()
	in.Gender = "ж"

	// Act
	r, err := pickup.NewReceiver(in)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, person.Female, r.Gender)
}

func TestNewReceiver_NoGenderAndNoPatronymic_AsksForGender(t *testing.T) {
	// Arrange: определить пол не из чего, а по имени угадывать ненадёжно
	in := validInput()
	in.MiddleName = ""

	// Act
	_, err := pickup.NewReceiver(in)

	// Assert
	errs := fieldErrors(t, err)
	assert.Equal(t, []pickup.Field{pickup.FieldGender}, keys(errs))
	assert.Contains(t, errs[pickup.FieldGender], "отчества нет")
}

func TestNewReceiver_NoGenderAndUnrecognizedPatronymic_AsksForGender(t *testing.T) {
	// Arrange
	in := validInput()
	in.MiddleName = "Хасан"

	// Act
	_, err := pickup.NewReceiver(in)

	// Assert
	errs := fieldErrors(t, err)
	assert.Contains(t, errs[pickup.FieldGender], "«Хасан»")
}

func TestNewReceiver_SeveralInvalidFields_ReportsEveryFieldAtOnce(t *testing.T) {
	// Arrange: три ошибки в одной отправке формы
	in := validInput()
	in.LastName = ""
	in.Phone = "999 123 45 6"
	in.Email = "ivanov.example.com"

	// Act
	_, err := pickup.NewReceiver(in)

	// Assert
	errs := fieldErrors(t, err)
	assert.Equal(t, []pickup.Field{pickup.FieldEmail, pickup.FieldLastName, pickup.FieldPhone}, keys(errs))
	assert.Equal(t, "Укажите фамилию", errs[pickup.FieldLastName])
	assert.Contains(t, errs[pickup.FieldPhone], "введено 9")
	assert.Equal(t, "Нет символа «@»", errs[pickup.FieldEmail])
}

func TestNewReceiver_CommonPhoneNotations_NormalizeToPlus7(t *testing.T) {
	for _, phone := range []string{
		"8 (999) 123-45-67",
		"+7 999 123 45 67",
		"+7(999)123-45-67",
		"79991234567",
		"9991234567",
	} {
		t.Run(phone, func(t *testing.T) {
			// Arrange
			in := validInput()
			in.Phone = phone

			// Act
			r, err := pickup.NewReceiver(in)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, "+79991234567", r.Phone)
		})
	}
}

func TestNewReceiver_InvalidPhones_ExplainWhatIsWrong(t *testing.T) {
	cases := []struct {
		name    string
		phone   string
		message string
	}{
		{name: "не хватает цифр", phone: "8 999 123 45", message: "введено 9"},
		{name: "лишняя цифра", phone: "8 999 123 45 678", message: "введено 12"},
		{name: "иностранный номер", phone: "+44 20 7946 0958", message: "только российские"},
		{name: "буква в номере", phone: "8 999 12З 45 67", message: "«З»"},
		{name: "плюс в середине", phone: "89+991234567", message: "только в начале"},
		{name: "пусто", phone: "  ", message: "Укажите телефон"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			in := validInput()
			in.Phone = tc.phone

			// Act
			_, err := pickup.NewReceiver(in)

			// Assert
			assert.Contains(t, fieldErrors(t, err)[pickup.FieldPhone], tc.message)
		})
	}
}

func TestNewReceiver_InvalidEmails_ExplainWhatIsWrong(t *testing.T) {
	cases := []struct {
		name    string
		email   string
		message string
	}{
		{name: "без собаки", email: "ivanov.example.com", message: "Нет символа «@»"},
		{name: "две собаки", email: "ivanov@mail@example.com", message: "должен быть один"},
		{name: "без имени", email: "@example.com", message: "Нет имени"},
		{name: "без домена", email: "ivanov@", message: "Нет домена"},
		{name: "домен без точки", email: "ivanov@example", message: "нет точки"},
		{name: "две точки подряд", email: "ivanov@example..com", message: "в точках"},
		{name: "пробел внутри", email: "i ivanov@example.com", message: "пробелов"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			in := validInput()
			in.Email = tc.email

			// Act
			_, err := pickup.NewReceiver(in)

			// Assert
			assert.Contains(t, fieldErrors(t, err)[pickup.FieldEmail], tc.message)
		})
	}
}

func TestNewReceiver_DigitInLastName_PointsAtThatCharacter(t *testing.T) {
	// Arrange
	in := validInput()
	in.LastName = "Иванов2"

	// Act
	_, err := pickup.NewReceiver(in)

	// Assert
	assert.Contains(t, fieldErrors(t, err)[pickup.FieldLastName], "«2»")
}

func TestNewReceiver_CompoundNames_AcceptedWithSpacesCollapsed(t *testing.T) {
	// Arrange: двойная фамилия, тюркское отчество из двух слов, лишние пробелы
	in := validInput()
	in.LastName = "  Римский-Корсаков "
	in.FirstName = "Николай"
	in.MiddleName = "Гасан   оглы"

	// Act
	r, err := pickup.NewReceiver(in)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Римский-Корсаков", r.Name.Last)
	assert.Equal(t, "Гасан оглы", r.Name.Middle)
	assert.Equal(t, person.Male, r.Gender)
}

func TestNewReceiver_NameStartingWithHyphen_Rejected(t *testing.T) {
	// Arrange
	in := validInput()
	in.FirstName = "-Иван"

	// Act
	_, err := pickup.NewReceiver(in)

	// Assert
	assert.Contains(t, fieldErrors(t, err)[pickup.FieldFirstName], "начинаться и заканчиваться буквой")
}

func TestNewReceiver_NameLongerThanLimit_ReportsActualLength(t *testing.T) {
	// Arrange
	in := validInput()
	in.LastName = strings.Repeat("я", 101)

	// Act
	_, err := pickup.NewReceiver(in)

	// Assert
	assert.Contains(t, fieldErrors(t, err)[pickup.FieldLastName], "сейчас 101")
}

func TestNewReceiver_UnknownGenderValue_ShowsWhatWasReceived(t *testing.T) {
	// Arrange
	in := validInput()
	in.Gender = "male"

	// Act
	_, err := pickup.NewReceiver(in)

	// Assert
	assert.Contains(t, fieldErrors(t, err)[pickup.FieldGender], "«male»")
}

func TestFieldErrorsError_ForLogs_ContainsFieldNamesButNoUserInput(t *testing.T) {
	// Arrange: текст ошибки почты содержит домен из ввода
	in := validInput()
	in.Email = "ivanov@secret-domain"

	_, err := pickup.NewReceiver(in)
	require.Error(t, err)

	// Act
	logLine := err.Error()

	// Assert
	assert.Contains(t, logLine, "email")
	assert.NotContains(t, logLine, "secret-domain", "данные получателя не должны попадать в лог")
}

func keys(errs pickup.FieldErrors) []pickup.Field {
	return slices.Sorted(maps.Keys(errs))
}
