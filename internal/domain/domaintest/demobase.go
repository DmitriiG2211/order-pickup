// Данные демо-базы 1С, выгруженные 2026-09-13: те же заказы, строки, остатки
// и резервы, что лежат в эмуляторе. Правьте вместе с фикстурами мока 1С.

package domaintest

import (
	"orderissue/internal/domain/order"
	"orderissue/internal/domain/stock"
	"orderissue/internal/domain/vat"
)

// Склады демо-базы.
const (
	WarehouseCentral  order.WarehouseRef = "e9f93b98-b2d3-11f1-a0b5-48df371887e9" // Центральный склад
	WarehouseNorth    order.WarehouseRef = "940498cc-5af6-11f1-a0b4-48df371887e9" // Северный склад
	WarehouseShowcase order.WarehouseRef = "e4bdeca3-a52f-11f1-a0b1-48df371887e9" // Витрина
)

// Товары демо-базы.
const (
	Product0001 order.ProductRef = "0faf93e1-ce2c-11f1-a0b2-48df371887e9" // ДМ-0001 Бумага офисная А4, 500 листов
	Product0002 order.ProductRef = "3fd3bf0e-91d3-11f1-a0b3-48df371887e9" // ДМ-0002 Ручка шариковая синяя
	Product0003 order.ProductRef = "05fb7a62-4619-11f1-a0b1-48df371887e9" // ДМ-0003 Папка-регистратор 75 мм
	Product0004 order.ProductRef = "4a522d9c-6904-11f1-a0b5-48df371887e9" // ДМ-0004 Степлер металлический №24
	Product0005 order.ProductRef = "d60690f7-dc78-11f1-a0b0-48df371887e9" // ДМ-0005 Скотч упаковочный 48 мм
)

// Заказы демо-базы.
const (
	Order101 order.Ref = "2da145f0-d5fc-11f1-a0b3-48df371887e9" // 00ДМ-000101
	Order102 order.Ref = "d0bc7651-c6dc-11f1-a0b1-48df371887e9" // 00ДМ-000102
	Order103 order.Ref = "d11793df-f4ef-11f1-a0b4-48df371887e9" // 00ДМ-000103
	Order104 order.Ref = "481ef429-7105-11f1-a0b5-48df371887e9" // 00ДМ-000104
	Order105 order.Ref = "ce6ca2b8-31c8-11f1-a0b1-48df371887e9" // 00ДМ-000105
)

// Warehouses — справочник складов.
func Warehouses() map[order.WarehouseRef]order.Warehouse {
	return map[order.WarehouseRef]order.Warehouse{
		"e9f93b98-b2d3-11f1-a0b5-48df371887e9": {Ref: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Name: "Центральный склад"},
		"940498cc-5af6-11f1-a0b4-48df371887e9": {Ref: "940498cc-5af6-11f1-a0b4-48df371887e9", Name: "Северный склад"},
		"e4bdeca3-a52f-11f1-a0b1-48df371887e9": {Ref: "e4bdeca3-a52f-11f1-a0b1-48df371887e9", Name: "Витрина"},
	}
}

// Products — справочник номенклатуры.
func Products() map[order.ProductRef]order.Product {
	return map[order.ProductRef]order.Product{
		"0faf93e1-ce2c-11f1-a0b2-48df371887e9": {Ref: "0faf93e1-ce2c-11f1-a0b2-48df371887e9", Article: "ДМ-0001", Name: "Бумага офисная А4, 500 листов"},
		"3fd3bf0e-91d3-11f1-a0b3-48df371887e9": {Ref: "3fd3bf0e-91d3-11f1-a0b3-48df371887e9", Article: "ДМ-0002", Name: "Ручка шариковая синяя"},
		"05fb7a62-4619-11f1-a0b1-48df371887e9": {Ref: "05fb7a62-4619-11f1-a0b1-48df371887e9", Article: "ДМ-0003", Name: "Папка-регистратор 75 мм"},
		"4a522d9c-6904-11f1-a0b5-48df371887e9": {Ref: "4a522d9c-6904-11f1-a0b5-48df371887e9", Article: "ДМ-0004", Name: "Степлер металлический №24"},
		"d60690f7-dc78-11f1-a0b0-48df371887e9": {Ref: "d60690f7-dc78-11f1-a0b0-48df371887e9", Article: "ДМ-0005", Name: "Скотч упаковочный 48 мм"},
	}
}

// VatRates — справочник ставок НДС, все 9 штук.
func VatRates() map[vat.RateRef]vat.Rate {
	return map[vat.RateRef]vat.Rate{
		"401fa06c-cdaa-11f1-a0b0-48df371887e9": {Name: "Без НДС", Percent: d("0")},
		"e4addfac-9a93-11f1-a0b1-48df371887e9": {Name: "0%", Percent: d("0")},
		"37ed6e41-6d57-11f1-a0b0-48df371887e9": {Name: "10%", Percent: d("10")},
		"de224f60-eed1-11f1-a0b5-48df371887e9": {Name: "10/110", Percent: d("10")},
		"a567a20d-d3b6-11f1-a0b0-48df371887e9": {Name: "20%", Percent: d("20")},
		"753672ef-00eb-11f1-a0b5-48df371887e9": {Name: "20/120", Percent: d("20")},
		"2ede2bcf-8851-11f1-a0b1-48df371887e9": {Name: "22%", Percent: d("22")},
		"d866b397-abf8-11f1-a0b1-48df371887e9": {Name: "22/122", Percent: d("22")},
		"33e4dcc4-064a-11f1-a0b0-48df371887e9": {Name: "13%", Percent: d("13")},
	}
}

// Customers — клиенты (партнёры).
func Customers() map[order.PartnerRef]order.Customer {
	return map[order.PartnerRef]order.Customer{
		"f9ca61d9-dfc6-11f1-a0b1-48df371887e9": {Ref: "f9ca61d9-dfc6-11f1-a0b1-48df371887e9", Name: "Воронцов Пётр Аркадьевич", IsPerson: true},
		"e2898db9-4a83-11f1-a0b2-48df371887e9": {Ref: "e2898db9-4a83-11f1-a0b2-48df371887e9", Name: "Синицына Ольга Львовна", IsPerson: true},
		"e06a2ff9-a3fe-11f1-a0b0-48df371887e9": {Ref: "e06a2ff9-a3fe-11f1-a0b0-48df371887e9", Name: "Коваленко Тарас Игоревич", IsPerson: true},
		"67453d28-1958-11f1-a0b0-48df371887e9": {Ref: "67453d28-1958-11f1-a0b0-48df371887e9", Name: "Седых Вера Павловна", IsPerson: true},
		"8dbf0bf8-cf02-11f1-a0b1-48df371887e9": {Ref: "8dbf0bf8-cf02-11f1-a0b1-48df371887e9", Name: "ООО «Ромашка»", IsPerson: false},
	}
}

// Orders — все пять заказов демо-базы со строками.
func Orders() map[order.Ref]order.Order {
	return map[order.Ref]order.Order{
		"2da145f0-d5fc-11f1-a0b3-48df371887e9": {
			Ref: "2da145f0-d5fc-11f1-a0b3-48df371887e9", Number: "00ДМ-000101", Date: date("2026-08-03T09:14:27"),
			Posted: true, DeletionMark: false, Status: "КОтгрузке", PriceIncludesVAT: false,
			Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Customer: "f9ca61d9-dfc6-11f1-a0b1-48df371887e9",
			Accounting: order.Accounting{Counterparty: "ff3f3d41-d5f7-11f1-a0b3-48df371887e9", Organization: "b176c2bc-f41e-11f1-a0b1-48df371887e9", Currency: "c14937b9-ee66-11f1-a0b0-48df371887e9", Manager: "b983b655-3320-11f1-a0b2-48df371887e9", VATTaxation: "ПродажаОблагаетсяНДС"},
			Lines: []order.Line{
				{Number: 1, Product: "0faf93e1-ce2c-11f1-a0b2-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("3"), Price: d("1250"), VatRate: "2ede2bcf-8851-11f1-a0b1-48df371887e9", Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Cancelled: false},
				{Number: 2, Product: "3fd3bf0e-91d3-11f1-a0b3-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("2"), Price: d("990.5"), VatRate: "d866b397-abf8-11f1-a0b1-48df371887e9", Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Cancelled: false},
				{Number: 3, Product: "05fb7a62-4619-11f1-a0b1-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("10"), Price: d("45"), VatRate: "401fa06c-cdaa-11f1-a0b0-48df371887e9", Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Cancelled: false},
				{Number: 4, Product: "4a522d9c-6904-11f1-a0b5-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("1"), Price: d("15000"), VatRate: "33e4dcc4-064a-11f1-a0b0-48df371887e9", Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Cancelled: false},
			},
		},
		"d0bc7651-c6dc-11f1-a0b1-48df371887e9": {
			Ref: "d0bc7651-c6dc-11f1-a0b1-48df371887e9", Number: "00ДМ-000102", Date: date("2026-08-04T10:14:27"),
			Posted: true, DeletionMark: false, Status: "КОтгрузке", PriceIncludesVAT: true,
			Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Customer: "e2898db9-4a83-11f1-a0b2-48df371887e9",
			Accounting: order.Accounting{Counterparty: "49991005-e828-11f1-a0b0-48df371887e9", Organization: "b176c2bc-f41e-11f1-a0b1-48df371887e9", Currency: "c14937b9-ee66-11f1-a0b0-48df371887e9", Manager: "b983b655-3320-11f1-a0b2-48df371887e9", VATTaxation: "ПродажаОблагаетсяНДС"},
			Lines: []order.Line{
				{Number: 1, Product: "0faf93e1-ce2c-11f1-a0b2-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("4"), Price: d("2400"), VatRate: "2ede2bcf-8851-11f1-a0b1-48df371887e9", Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Cancelled: false},
				{Number: 2, Product: "3fd3bf0e-91d3-11f1-a0b3-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("1"), Price: d("899.99"), VatRate: "a567a20d-d3b6-11f1-a0b0-48df371887e9", Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Cancelled: false},
				{Number: 3, Product: "05fb7a62-4619-11f1-a0b1-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("7"), Price: d("120.35"), VatRate: "37ed6e41-6d57-11f1-a0b0-48df371887e9", Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Cancelled: false},
			},
		},
		"d11793df-f4ef-11f1-a0b4-48df371887e9": {
			Ref: "d11793df-f4ef-11f1-a0b4-48df371887e9", Number: "00ДМ-000103", Date: date("2026-08-05T11:14:27"),
			Posted: true, DeletionMark: false, Status: "КОтгрузке", PriceIncludesVAT: false,
			Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Customer: "e06a2ff9-a3fe-11f1-a0b0-48df371887e9",
			Accounting: order.Accounting{Counterparty: "93c46f09-a2af-11f1-a0b0-48df371887e9", Organization: "b176c2bc-f41e-11f1-a0b1-48df371887e9", Currency: "c14937b9-ee66-11f1-a0b0-48df371887e9", Manager: "b983b655-3320-11f1-a0b2-48df371887e9", VATTaxation: "ПродажаОблагаетсяНДС"},
			Lines: []order.Line{
				{Number: 1, Product: "d60690f7-dc78-11f1-a0b0-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("5"), Price: d("1.15"), VatRate: "2ede2bcf-8851-11f1-a0b1-48df371887e9", Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Cancelled: false},
				{Number: 2, Product: "4a522d9c-6904-11f1-a0b5-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("0.125"), Price: d("8000"), VatRate: "2ede2bcf-8851-11f1-a0b1-48df371887e9", Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Cancelled: false},
				{Number: 3, Product: "3fd3bf0e-91d3-11f1-a0b3-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("2"), Price: d("500"), VatRate: "2ede2bcf-8851-11f1-a0b1-48df371887e9", Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Cancelled: false},
				{Number: 4, Product: "3fd3bf0e-91d3-11f1-a0b3-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("3"), Price: d("450"), VatRate: "2ede2bcf-8851-11f1-a0b1-48df371887e9", Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Cancelled: false},
			},
		},
		"481ef429-7105-11f1-a0b5-48df371887e9": {
			Ref: "481ef429-7105-11f1-a0b5-48df371887e9", Number: "00ДМ-000104", Date: date("2026-08-06T12:14:27"),
			Posted: true, DeletionMark: false, Status: "КОтгрузке", PriceIncludesVAT: false,
			Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Customer: "67453d28-1958-11f1-a0b0-48df371887e9",
			Accounting: order.Accounting{Counterparty: "94840d7a-85c3-11f1-a0b0-48df371887e9", Organization: "b176c2bc-f41e-11f1-a0b1-48df371887e9", Currency: "c14937b9-ee66-11f1-a0b0-48df371887e9", Manager: "b983b655-3320-11f1-a0b2-48df371887e9", VATTaxation: "ПродажаОблагаетсяНДС"},
			Lines: []order.Line{
				{Number: 1, Product: "05fb7a62-4619-11f1-a0b1-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("2"), Price: d("1999"), VatRate: "2ede2bcf-8851-11f1-a0b1-48df371887e9", Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Cancelled: false},
				{Number: 2, Product: "d60690f7-dc78-11f1-a0b0-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("1"), Price: d("640.4"), VatRate: "2ede2bcf-8851-11f1-a0b1-48df371887e9", Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Cancelled: false},
			},
		},
		"ce6ca2b8-31c8-11f1-a0b1-48df371887e9": {
			Ref: "ce6ca2b8-31c8-11f1-a0b1-48df371887e9", Number: "00ДМ-000105", Date: date("2026-08-07T13:14:27"),
			Posted: false, DeletionMark: true, Status: "НеСогласован", PriceIncludesVAT: false,
			Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Customer: "8dbf0bf8-cf02-11f1-a0b1-48df371887e9",
			Accounting: order.Accounting{Counterparty: "05a8ba31-8ab0-11f1-a0b4-48df371887e9", Organization: "b176c2bc-f41e-11f1-a0b1-48df371887e9", Currency: "c14937b9-ee66-11f1-a0b0-48df371887e9", Manager: "b983b655-3320-11f1-a0b2-48df371887e9", VATTaxation: "ПродажаОблагаетсяНДС"},
			Lines: []order.Line{
				{Number: 1, Product: "0faf93e1-ce2c-11f1-a0b2-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000", Quantity: d("1"), Price: d("100"), VatRate: "2ede2bcf-8851-11f1-a0b1-48df371887e9", Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Cancelled: false},
			},
		},
	}
}

// Balances — остатки «ТоварыНаСкладах» по всем складам.
func Balances() []stock.Balance {
	return []stock.Balance{
		{Item: stock.Item{Product: "0faf93e1-ce2c-11f1-a0b2-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", OnHand: d("2")},
		{Item: stock.Item{Product: "3fd3bf0e-91d3-11f1-a0b3-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", OnHand: d("40")},
		{Item: stock.Item{Product: "05fb7a62-4619-11f1-a0b1-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", OnHand: d("128")},
		{Item: stock.Item{Product: "4a522d9c-6904-11f1-a0b5-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", OnHand: d("15")},
		{Item: stock.Item{Product: "d60690f7-dc78-11f1-a0b0-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", OnHand: d("60")},
		{Item: stock.Item{Product: "0faf93e1-ce2c-11f1-a0b2-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", OnHand: d("90")},
		{Item: stock.Item{Product: "3fd3bf0e-91d3-11f1-a0b3-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", OnHand: d("12")},
		{Item: stock.Item{Product: "05fb7a62-4619-11f1-a0b1-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", OnHand: d("30")},
		{Item: stock.Item{Product: "4a522d9c-6904-11f1-a0b5-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", OnHand: d("7")},
		{Item: stock.Item{Product: "d60690f7-dc78-11f1-a0b0-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", OnHand: d("25")},
		{Item: stock.Item{Product: "0faf93e1-ce2c-11f1-a0b2-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e4bdeca3-a52f-11f1-a0b1-48df371887e9", OnHand: d("5")},
		{Item: stock.Item{Product: "05fb7a62-4619-11f1-a0b1-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e4bdeca3-a52f-11f1-a0b1-48df371887e9", OnHand: d("8")},
		{Item: stock.Item{Product: "4a522d9c-6904-11f1-a0b5-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e4bdeca3-a52f-11f1-a0b1-48df371887e9", OnHand: d("3")},
	}
}

// Reservations — резервы «ТоварыКОтгрузке» (ВРезерве + КОтгрузке).
func Reservations() []stock.Reservation {
	return []stock.Reservation{
		{Item: stock.Item{Product: "0faf93e1-ce2c-11f1-a0b2-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Document: "2da145f0-d5fc-11f1-a0b3-48df371887e9", Quantity: d("3")},
		{Item: stock.Item{Product: "3fd3bf0e-91d3-11f1-a0b3-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Document: "2da145f0-d5fc-11f1-a0b3-48df371887e9", Quantity: d("2")},
		{Item: stock.Item{Product: "05fb7a62-4619-11f1-a0b1-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Document: "2da145f0-d5fc-11f1-a0b3-48df371887e9", Quantity: d("10")},
		{Item: stock.Item{Product: "4a522d9c-6904-11f1-a0b5-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Document: "2da145f0-d5fc-11f1-a0b3-48df371887e9", Quantity: d("1")},
		{Item: stock.Item{Product: "0faf93e1-ce2c-11f1-a0b2-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Document: "d0bc7651-c6dc-11f1-a0b1-48df371887e9", Quantity: d("4")},
		{Item: stock.Item{Product: "3fd3bf0e-91d3-11f1-a0b3-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Document: "d0bc7651-c6dc-11f1-a0b1-48df371887e9", Quantity: d("1")},
		{Item: stock.Item{Product: "05fb7a62-4619-11f1-a0b1-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "e9f93b98-b2d3-11f1-a0b5-48df371887e9", Document: "d0bc7651-c6dc-11f1-a0b1-48df371887e9", Quantity: d("7")},
		{Item: stock.Item{Product: "d60690f7-dc78-11f1-a0b0-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Document: "d11793df-f4ef-11f1-a0b4-48df371887e9", Quantity: d("5")},
		{Item: stock.Item{Product: "4a522d9c-6904-11f1-a0b5-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Document: "d11793df-f4ef-11f1-a0b4-48df371887e9", Quantity: d("0.125")},
		{Item: stock.Item{Product: "3fd3bf0e-91d3-11f1-a0b3-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Document: "d11793df-f4ef-11f1-a0b4-48df371887e9", Quantity: d("2")},
		{Item: stock.Item{Product: "3fd3bf0e-91d3-11f1-a0b3-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Document: "d11793df-f4ef-11f1-a0b4-48df371887e9", Quantity: d("3")},
		{Item: stock.Item{Product: "05fb7a62-4619-11f1-a0b1-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Document: "481ef429-7105-11f1-a0b5-48df371887e9", Quantity: d("2")},
		{Item: stock.Item{Product: "d60690f7-dc78-11f1-a0b0-48df371887e9", Characteristic: "00000000-0000-0000-0000-000000000000"}, Warehouse: "940498cc-5af6-11f1-a0b4-48df371887e9", Document: "481ef429-7105-11f1-a0b5-48df371887e9", Quantity: d("1")},
	}
}
