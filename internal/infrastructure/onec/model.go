// Package onec — антикоррупционный слой к 1С:УТ, опубликованной через OData.
//
// Здесь живут имена полей 1С, её коды ответов и все её неприятности: страница
// по три записи, временные 503, 429, блокировка учётки. Наружу выходят только
// доменные объекты и ошибки usecase.UpstreamError с конкретным кодом.
package onec

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"orderissue/internal/usecase"

	"github.com/sony/gobreaker/v2"
	"golang.org/x/time/rate"
)

// Config — настройки клиента. Числа обоснованы в ADR 0007 и 0008.
type Config struct {
	BaseURL  string
	User     string
	Password string

	// AttemptTimeout — сколько ждём ответа на одну попытку.
	AttemptTimeout time.Duration
	// MaxAttempts — попыток на один запрос. При доле сбоев 1/30 четыре попытки
	// дают отказ запроса 1 на 810 000, а экрана из ~20 запросов — 1 на 40 000.
	MaxAttempts int
	// BackoffBase — первая пауза; дальше удваивается: 1, 2, 4 с (±20%).
	BackoffBase time.Duration
	// RatePerSecond — свой лимит частоты, чтобы реже получать 429. 0 — без лимита.
	RatePerSecond float64
	// MaxConcurrent — одновременных запросов в 1С.
	MaxConcurrent int
	// BreakerFailures — сколько запросов подряд должны исчерпать повторы,
	// чтобы 1С признали недоступной.
	BreakerFailures uint32
	// BreakerOpenFor — сколько не отправляем запросы после этого.
	BreakerOpenFor time.Duration
	// LockBackoff — пауза после 403: блокировка учётки у 1С длится 5 минут.
	LockBackoff time.Duration
}

// DefaultConfig — рабочие значения для эмулятора из задания.
func DefaultConfig() Config {
	return Config{
		AttemptTimeout:  5 * time.Second,
		MaxAttempts:     4,
		BackoffBase:     time.Second,
		RatePerSecond:   5,
		MaxConcurrent:   2,
		BreakerFailures: 5,
		BreakerOpenFor:  30 * time.Second,
		LockBackoff:     5 * time.Minute,
	}
}

type authState int

const (
	authUnknown authState = iota
	authConfirmed
	authRejected
	authLocked
)

// recentErrorWindow — сколько после ошибки считаем связь «деградировавшей».
const recentErrorWindow = time.Minute

var (
	utf8BOM = []byte{0xEF, 0xBB, 0xBF}
	// errAttemptTimeout помечает таймаут одной попытки, в отличие от общего дедлайна.
	errAttemptTimeout = errors.New("1С не ответила за время одной попытки")
)

// Client — HTTP-клиент OData с повторами, лимитами и размыкателями.
type Client struct {
	cfg     Config
	http    *http.Client
	clock   usecase.Clock
	logger  *slog.Logger
	limiter *rate.Limiter
	slots   chan struct{}
	breaker *gobreaker.CircuitBreaker[[]byte]

	// probe не пускает запросы параллельно, пока логин не подтверждён:
	// с неверным паролем в 1С уйдёт одна неудачная попытка входа, а не десять.
	probe sync.Mutex

	mu            sync.Mutex
	auth          authState
	lockedUntil   time.Time
	pauseUntil    time.Time // общий стоп после 429: ждут все запросы, не только упавший
	breakerOpened time.Time
	lastErr       *usecase.UpstreamError
	lastErrAt     time.Time
}

// call — один логический запрос к 1С.
type call struct {
	op     string // по-русски, для ошибки: «чтение строк заказа»
	method string
	path   string // относительный путь с уже экранированными сегментами
	query  string
	body   []byte
}

// Наборы OData, с которыми работает сервис.
const (
	setOrders       = "Document_ЗаказКлиента"
	setOrderLines   = "Document_ЗаказКлиента_Товары"
	setProducts     = "Catalog_Номенклатура"
	setVatRates     = "Catalog_СтавкиНДС"
	setWarehouses   = "Catalog_Склады"
	setPartners     = "Catalog_Партнеры"
	setOnHand       = "AccumulationRegister_ТоварыНаСкладах/Balance"
	setReserved     = "AccumulationRegister_ТоварыКОтгрузке/Balance"
	setShipments    = "Document_РеализацияТоваровУслуг"
	setShipmentRows = "Document_РеализацияТоваровУслуг_Товары"

	dateLayout = "2006-01-02T15:04:05"
	emptyRef   = "00000000-0000-0000-0000-000000000000"
	// maxPages защищает от бесконечного цикла, если 1С сломает пагинацию иначе,
	// чем повтором страницы.
	maxPages = 10000
)

// Gateway реализует порты сценариев поверх OData 1С.
type Gateway struct {
	client *Client
}

// --- DTO: поля так, как их называет 1С ---

type namedDTO struct {
	RefKey      string `json:"Ref_Key"`
	Description string `json:"Description"`
}

type productDTO struct {
	namedDTO
	Article string `json:"Артикул"`
}

type vatRateDTO struct {
	namedDTO
	Rate json.Number `json:"Ставка"`
}

type partnerDTO struct {
	namedDTO
	IsPerson bool `json:"ЭтоФизическоеЛицо"`
}

type orderDTO struct {
	RefKey           string `json:"Ref_Key"`
	Number           string `json:"Number"`
	Date             string `json:"Date"`
	Posted           bool   `json:"Posted"`
	DeletionMark     bool   `json:"DeletionMark"`
	Status           string `json:"Статус"`
	PriceIncludesVAT bool   `json:"ЦенаВключаетНДС"`
	WarehouseKey     string `json:"Склад_Key"`
	PartnerKey       string `json:"Партнер_Key"`
	CounterpartyKey  string `json:"Контрагент_Key"`
	OrganizationKey  string `json:"Организация_Key"`
	CurrencyKey      string `json:"Валюта_Key"`
	ManagerKey       string `json:"Менеджер_Key"`
	VATTaxation      string `json:"НалогообложениеНДС"`
}

type orderLineDTO struct {
	LineNumber        string      `json:"LineNumber"`
	ProductKey        string      `json:"Номенклатура_Key"`
	CharacteristicKey string      `json:"Характеристика_Key"`
	Quantity          json.Number `json:"Количество"`
	Price             json.Number `json:"Цена"`
	VatRateKey        string      `json:"СтавкаНДС_Key"`
	WarehouseKey      string      `json:"Склад_Key"`
	Cancelled         bool        `json:"Отменено"`
}

type balanceDTO struct {
	ProductKey        string      `json:"Номенклатура_Key"`
	CharacteristicKey string      `json:"Характеристика_Key"`
	WarehouseKey      string      `json:"Склад_Key"`
	OnHand            json.Number `json:"ВНаличииBalance"`
}

type reservationDTO struct {
	ProductKey        string      `json:"Номенклатура_Key"`
	CharacteristicKey string      `json:"Характеристика_Key"`
	WarehouseKey      string      `json:"Склад_Key"`
	Document          string      `json:"ДокументОтгрузки"`
	Reserved          json.Number `json:"ВРезервеBalance"`
	ToShip            json.Number `json:"КОтгрузкеBalance"`
}

type shipmentDTO struct {
	RefKey       string `json:"Ref_Key"`
	Number       string `json:"Number"`
	Date         string `json:"Date"`
	Posted       bool   `json:"Posted"`
	DeletionMark bool   `json:"DeletionMark"`
	OrderKey     string `json:"ЗаказКлиента_Key"`
	Comment      string `json:"Комментарий"`
}

type shipmentLineDTO struct {
	LineNumber string      `json:"LineNumber"`
	ProductKey string      `json:"Номенклатура_Key"`
	Quantity   json.Number `json:"Количество"`
	Price      json.Number `json:"Цена"`
	VatRateKey string      `json:"СтавкаНДС_Key"`
	Amount     json.Number `json:"Сумма"`
	VAT        json.Number `json:"СуммаНДС"`
	WithVAT    json.Number `json:"СуммаСНДС"`
}

// shipmentPayload — тело POST реализации. Набор полей повторяет черновик,
// который эмулятор принимает; клиентский Ref_Key не передаём — 1С его игнорирует.
type shipmentPayload struct {
	Date              string                `json:"Date"`
	Posted            bool                  `json:"Posted"`
	DeletionMark      bool                  `json:"DeletionMark"`
	OrderKey          string                `json:"ЗаказКлиента_Key"`
	PartnerKey        string                `json:"Партнер_Key"`
	CounterpartyKey   string                `json:"Контрагент_Key"`
	OrganizationKey   string                `json:"Организация_Key"`
	CurrencyKey       string                `json:"Валюта_Key"`
	WarehouseKey      string                `json:"Склад_Key"`
	ManagerKey        string                `json:"Менеджер_Key"`
	ResponsibleKey    string                `json:"Ответственный_Key"`
	PriceIncludesVAT  bool                  `json:"ЦенаВключаетНДС"`
	VATTaxation       string                `json:"НалогообложениеНДС"`
	BusinessOperation string                `json:"ХозяйственнаяОперация"`
	Agreed            bool                  `json:"Согласован"`
	Comment           string                `json:"Комментарий"`
	Lines             []shipmentLinePayload `json:"Товары"`
}

type shipmentLinePayload struct {
	LineNumber        string      `json:"LineNumber"`
	RowCode           int         `json:"КодСтроки"`
	ProductKey        string      `json:"Номенклатура_Key"`
	CharacteristicKey string      `json:"Характеристика_Key"`
	PackageKey        string      `json:"Упаковка_Key"`
	Quantity          json.Number `json:"Количество"`
	PackageQuantity   json.Number `json:"КоличествоУпаковок"`
	Price             json.Number `json:"Цена"`
	Amount            json.Number `json:"Сумма"`
	VatRateKey        string      `json:"СтавкаНДС_Key"`
	VAT               json.Number `json:"СуммаНДС"`
	WithVAT           json.Number `json:"СуммаСНДС"`
	WarehouseKey      string      `json:"Склад_Key"`
	OrderKey          string      `json:"ЗаказКлиента_Key"`
}
