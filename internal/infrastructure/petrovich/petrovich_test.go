package petrovich_test

import (
	"testing"

	"orderissue/internal/domain/person"
	"orderissue/internal/infrastructure/petrovich"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newInflector(t *testing.T) *petrovich.Inflector {
	t.Helper()
	inf, err := petrovich.New()
	require.NoError(t, err)
	return inf
}

func name(last, first, middle string) person.FullName {
	return person.FullName{Last: last, First: first, Middle: middle}
}

func TestGenitive_ClientsFromTask_MatchReference(t *testing.T) {
	// Arrange: четыре клиента из базы, подобранные заданием нарочно
	cases := []struct {
		name   person.FullName
		gender person.Gender
		want   string
	}{
		{name("Воронцов", "Пётр", "Аркадьевич"), person.Male, "Воронцова Петра Аркадьевича"},
		{name("Синицына", "Ольга", "Львовна"), person.Female, "Синицыной Ольги Львовны"},
		{name("Коваленко", "Тарас", "Игоревич"), person.Male, "Коваленко Тараса Игоревича"},
		{name("Седых", "Вера", "Павловна"), person.Female, "Седых Веры Павловны"},
	}
	inf := newInflector(t)

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			// Act
			got := inf.Genitive(tc.name, tc.gender)

			// Assert
			assert.Equal(t, tc.want, got.String())
		})
	}
}

func TestGenitive_MaleSurnameEndingInYkh_StaysUnchanged(t *testing.T) {
	// Arrange: на этом случае Go-порты petrovich дают «Седыха»
	inf := newInflector(t)

	// Act
	got := inf.Genitive(name("Седых", "Павел", "Андреевич"), person.Male)

	// Assert
	assert.Equal(t, "Седых Павла Андреевича", got.String())
}

func TestGenitive_SurnameWithoutGenderMarker_DependsOnGender(t *testing.T) {
	// Arrange: «Шевчук» склоняется у мужчины и не склоняется у женщины
	inf := newInflector(t)

	// Act
	male := inf.Genitive(name("Шевчук", "Юрий", "Юлианович"), person.Male)
	female := inf.Genitive(name("Шевчук", "Анна", "Петровна"), person.Female)

	// Assert
	assert.Equal(t, "Шевчука Юрия Юлиановича", male.String())
	assert.Equal(t, "Шевчук Анны Петровны", female.String())
}

func TestGenitive_CompoundSurname_InflectsEachPart(t *testing.T) {
	// Arrange
	inf := newInflector(t)

	// Act
	got := inf.Genitive(name("Римский-Корсаков", "Николай", "Андреевич"), person.Male)

	// Assert
	assert.Equal(t, "Римского-Корсакова Николая Андреевича", got.String())
}

func TestGenitive_FirstWordException_AppliesOnlyToFirstPartOfCompound(t *testing.T) {
	// Arrange: «Бонч» в двойной фамилии не склоняется; правило помечено first_word
	inf := newInflector(t)

	// Act
	got := inf.Genitive(name("Бонч-Бруевич", "Владимир", "Дмитриевич"), person.Male)

	// Assert
	assert.Equal(t, "Бонч-Бруевича Владимира Дмитриевича", got.String())
}

func TestGenitive_NamesWithIrregularStems_UseExceptions(t *testing.T) {
	// Arrange: беглые гласные и чередования, которые не выводятся из окончания
	cases := []struct {
		name   person.FullName
		gender person.Gender
		want   string
	}{
		{name("Толстой", "Лев", "Николаевич"), person.Male, "Толстого Льва Николаевича"},
		{name("Никитин", "Илья", "Сергеевич"), person.Male, "Никитина Ильи Сергеевича"},
		{name("Толстая", "Любовь", "Ильинична"), person.Female, "Толстой Любови Ильиничны"},
	}
	inf := newInflector(t)

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			// Act
			got := inf.Genitive(tc.name, tc.gender)

			// Assert
			assert.Equal(t, tc.want, got.String())
		})
	}
}

func TestGenitive_TurkicPatronymic_KeepsParticleUnchanged(t *testing.T) {
	// Arrange
	inf := newInflector(t)

	// Act
	got := inf.Genitive(name("Мамедов", "Эльдар", "Гасан оглы"), person.Male)

	// Assert
	assert.Equal(t, "Мамедова Эльдара Гасан оглы", got.String())
}

func TestGenitive_WithoutMiddleName_ReturnsEmptyMiddleName(t *testing.T) {
	// Arrange
	inf := newInflector(t)

	// Act
	got := inf.Genitive(name("Иванов", "Иван", ""), person.Male)

	// Assert
	assert.Equal(t, "Иванова Ивана", got.String())
}
