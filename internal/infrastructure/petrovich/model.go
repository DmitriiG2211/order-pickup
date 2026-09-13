// Package petrovich склоняет ФИО по официальным правилам проекта petrovich
// (github.com/petrovich/petrovich-rules, MIT, файл rules.json рядом).
//
// Готовые порты на Go (an112chuh/Petrovich-Go и его форки) при совпадении
// правила «оставить как есть» не останавливаются, а ищут следующее —
// и мужская «Седых» превращается в «Седыха». Правила при этом верные,
// поэтому берём их как есть, а выбор правила пишем сами, по эталонной
// реализации petrovich-js.
package petrovich

import _ "embed"

//go:embed rules.json
var rulesJSON []byte

// Индексы падежей в полях mods правил.
const genitiveCase = 0

// tagFirstWord помечает правила, которые действуют только на первую часть
// двойного имени: «Бонч» в «Бонч-Бруевич» не склоняется, а «Ван» сам по себе — да.
const tagFirstWord = "first_word"

type rule struct {
	Gender string   `json:"gender"`
	Test   []string `json:"test"`
	Mods   []string `json:"mods"`
	Tags   []string `json:"tags"`
}

type ruleSet struct {
	Exceptions []rule `json:"exceptions"`
	Suffixes   []rule `json:"suffixes"`
}

type rules struct {
	Lastname   ruleSet `json:"lastname"`
	Firstname  ruleSet `json:"firstname"`
	Middlename ruleSet `json:"middlename"`
}

// Inflector реализует person.NameInflector.
type Inflector struct {
	rules rules
}
