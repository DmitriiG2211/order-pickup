package person_test

import (
	"testing"

	"orderissue/internal/domain/person"

	"github.com/stretchr/testify/assert"
)

func TestGenderFromPatronymic_KnownEndings_DetectsGender(t *testing.T) {
	cases := []struct {
		name       string
		patronymic string
		want       person.Gender
	}{
		{name: "мужское на -вич", patronymic: "Аркадьевич", want: person.Male},
		{name: "мужское на -ич без -в-", patronymic: "Ильич", want: person.Male},
		{name: "женское на -вна", patronymic: "Львовна", want: person.Female},
		{name: "женское на -ична", patronymic: "Ильинична", want: person.Female},
		{name: "тюркское мужское", patronymic: "Гасан оглы", want: person.Male},
		{name: "тюркское женское", patronymic: "Гасан кызы", want: person.Female},
		{name: "регистр не важен", patronymic: "ПАВЛОВНА", want: person.Female},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got, ok := person.GenderFromPatronymic(tc.patronymic)

			// Assert
			assert.True(t, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGenderFromPatronymic_EmptyOrUnrecognized_ReportsUnknown(t *testing.T) {
	for _, patronymic := range []string{"", "   ", "Хасан"} {
		t.Run(patronymic, func(t *testing.T) {
			// Act
			_, ok := person.GenderFromPatronymic(patronymic)

			// Assert
			assert.False(t, ok, "по такому отчеству пол угадывать нельзя")
		})
	}
}

func TestFullNameString_WithoutMiddleName_OmitsTrailingSpace(t *testing.T) {
	// Arrange: отчества может не быть, «Иванов Иван » с пробелом в документе недопустим
	name := person.FullName{Last: "Иванов", First: "Иван"}

	// Act
	got := name.String()

	// Assert
	assert.Equal(t, "Иванов Иван", got)
}
