package petrovich

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"orderissue/internal/domain/person"
)

var _ person.NameInflector = (*Inflector)(nil)

// New загружает вшитые правила. Ошибка означает битый rules.json в сборке.
func New() (*Inflector, error) {
	var r rules
	if err := json.Unmarshal(rulesJSON, &r); err != nil {
		return nil, fmt.Errorf("petrovich: разбор rules.json: %w", err)
	}
	return &Inflector{rules: r}, nil
}

// Genitive возвращает ФИО в родительном падеже.
func (i *Inflector) Genitive(name person.FullName, gender person.Gender) person.FullName {
	g := genderName(gender)
	return person.FullName{
		Last:   inflect(name.Last, i.rules.Lastname, g),
		First:  inflect(name.First, i.rules.Firstname, g),
		Middle: inflect(name.Middle, i.rules.Middlename, g),
	}
}

func genderName(g person.Gender) string {
	if g == person.Female {
		return "female"
	}
	return "male"
}

// inflect склоняет каждую часть двойного имени отдельно.
func inflect(value string, set ruleSet, gender string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "-")
	for idx, part := range parts {
		isFirstWord := idx == 0 && len(parts) > 1
		if r, ok := findRule(part, set, gender, isFirstWord); ok {
			parts[idx] = apply(part, r.Mods[genitiveCase])
		}
	}
	return strings.Join(parts, "-")
}

// findRule ищет сначала среди исключений (слово целиком), потом среди
// окончаний. Первое подходящее правило — окончательное, даже если оно
// велит оставить слово как есть: в этом и была ошибка Go-портов.
func findRule(word string, set ruleSet, gender string, isFirstWord bool) (rule, bool) {
	lower := strings.ToLower(word)
	for _, r := range set.Exceptions {
		if applicable(r, gender, isFirstWord) && slices.Contains(r.Test, lower) {
			return r, true
		}
	}
	for _, r := range set.Suffixes {
		if !applicable(r, gender, isFirstWord) {
			continue
		}
		for _, suffix := range r.Test {
			if strings.HasSuffix(lower, suffix) {
				return r, true
			}
		}
	}
	return rule{}, false
}

func applicable(r rule, gender string, isFirstWord bool) bool {
	if r.Gender != "androgynous" && r.Gender != gender {
		return false
	}
	if slices.Contains(r.Tags, tagFirstWord) && !isFirstWord {
		return false
	}
	return true
}

// apply применяет модификатор: "." — без изменений; каждый "-" в начале
// отрезает одну букву с конца, остальное дописывается («--ьва»: Лев → Льва).
func apply(word, mod string) string {
	if mod == "." {
		return word
	}
	cut := len(mod) - len(strings.TrimLeft(mod, "-"))
	runes := []rune(word)
	if cut > len(runes) {
		cut = len(runes)
	}
	return string(runes[:len(runes)-cut]) + mod[cut:]
}
