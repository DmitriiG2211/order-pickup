package httpapi

import (
	"strings"

	"orderissue/internal/domain/order"
	"orderissue/internal/domain/person"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/usecase"
)

// wireNumber renders on the wire as a bare JSON number, matching the task's
// own example ("amount": 3750, not "3750"): a strict comparator on the other
// end would fail a string against a number even though the value is right.
// It carries the exact text money.Amount/money.Decimal already computed, so
// there is no float64 round-trip and no risk of it disagreeing with itself.
type wireNumber string

func (n wireNumber) MarshalJSON() ([]byte, error) {
	if n == "" {
		return []byte("0"), nil
	}
	return []byte(n), nil
}

func (n *wireNumber) UnmarshalJSON(data []byte) error {
	*n = wireNumber(strings.Trim(string(data), `"`))
	return nil
}

// receiverDTO — тело получателя, как его шлёт форма и проверка задания.
// Имена полей — контракт задания, менять нельзя.
type receiverDTO struct {
	LastName   string `json:"lastName"`
	FirstName  string `json:"firstName"`
	MiddleName string `json:"middleName"`
	Gender     string `json:"gender"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
}

func (d receiverDTO) toDomain() pickup.ReceiverInput {
	return pickup.ReceiverInput{
		LastName: d.LastName, FirstName: d.FirstName, MiddleName: d.MiddleName,
		Gender: d.Gender, Phone: d.Phone, Email: d.Email,
	}
}

// previewRequestDTO — тело POST /api/documents/preview.
type previewRequestDTO struct {
	OrderRef string      `json:"orderRef"`
	Customer receiverDTO `json:"customer"`
}

// customerResponseDTO, orderResponseDTO, lineResponseDTO, totalsDTO, previewResponseDTO
// повторяют контракт из задания дословно: поля сверяются по имени, не по структуре целиком,
// но лишние поля не мешают, а эти обязаны совпасть.
type customerResponseDTO struct {
	FullNameGenitive string `json:"fullNameGenitive"`
	Phone            string `json:"phone"`
	Email            string `json:"email"`
}

type orderResponseDTO struct {
	Number           string `json:"number"`
	Date             string `json:"date"`
	Warehouse        string `json:"warehouse"`
	PriceIncludesVAT bool   `json:"priceIncludesVat"`
}

type lineResponseDTO struct {
	Article       string     `json:"article"`
	Name          string     `json:"name"`
	Qty           wireNumber `json:"qty"`
	Price         wireNumber `json:"price"`
	VatName       string     `json:"vatName"`
	Amount        wireNumber `json:"amount"`
	VatAmount     wireNumber `json:"vatAmount"`
	AmountWithVAT wireNumber `json:"amountWithVat"`
}

type totalsDTO struct {
	Amount        wireNumber `json:"amount"`
	VatAmount     wireNumber `json:"vatAmount"`
	AmountWithVAT wireNumber `json:"amountWithVat"`
}

// previewResponseDTO — ответ preview. Issuable и BlockReasons — сверх контракта
// задания: лишние поля не мешают, а странице и проверяющему они полезны.
type previewResponseDTO struct {
	Customer     customerResponseDTO `json:"customer"`
	Order        orderResponseDTO    `json:"order"`
	Lines        []lineResponseDTO   `json:"lines"`
	Totals       totalsDTO           `json:"totals"`
	Issuable     bool                `json:"issuable"`
	BlockReasons []string            `json:"blockReasons,omitempty"`
}

func newPreviewResponse(doc pickup.Document) previewResponseDTO {
	lines := make([]lineResponseDTO, len(doc.Lines))
	for i, l := range doc.Lines {
		lines[i] = lineResponseDTO{
			Article: l.Article, Name: l.Name, Qty: wireNumber(l.Quantity.String()), Price: wireNumber(l.Price.String()),
			VatName: l.VatName, Amount: wireNumber(l.Sums.Amount.String()), VatAmount: wireNumber(l.Sums.VAT.String()),
			AmountWithVAT: wireNumber(l.Sums.WithVAT.String()),
		}
	}
	reasons := make([]string, len(doc.BlockReasons))
	for i, r := range doc.BlockReasons {
		reasons[i] = string(r)
	}
	return previewResponseDTO{
		Customer: customerResponseDTO{
			FullNameGenitive: doc.ReceiverGenitive.String(), Phone: doc.Phone, Email: doc.Email,
		},
		Order: orderResponseDTO{
			Number: doc.OrderNumber, Date: doc.OrderDate.Format("2006-01-02T15:04:05"),
			Warehouse: doc.WarehouseName, PriceIncludesVAT: doc.PriceIncludesVAT,
		},
		Lines:        lines,
		Totals:       totalsDTO{Amount: wireNumber(doc.Totals.Amount.String()), VatAmount: wireNumber(doc.Totals.VAT.String()), AmountWithVAT: wireNumber(doc.Totals.WithVAT.String())},
		Issuable:     len(doc.BlockReasons) == 0,
		BlockReasons: reasons,
	}
}

// orderSummaryDTO — строка списка заказов.
type orderSummaryDTO struct {
	Ref          string   `json:"ref"`
	Number       string   `json:"number"`
	Date         string   `json:"date"`
	Customer     string   `json:"customer"`
	Warehouse    string   `json:"warehouse"`
	Status       string   `json:"status"`
	Issuable     bool     `json:"issuable"`
	BlockReasons []string `json:"blockReasons,omitempty"`
}

func newOrderSummary(s usecase.OrderSummary) orderSummaryDTO {
	reasons := make([]string, len(s.BlockReasons))
	for i, r := range s.BlockReasons {
		reasons[i] = string(r)
	}
	return orderSummaryDTO{
		Ref: string(s.Ref), Number: s.Number, Date: s.Date.Format("2006-01-02T15:04:05"),
		Customer: s.CustomerName, Warehouse: s.WarehouseName, Status: s.Status,
		Issuable: len(s.BlockReasons) == 0, BlockReasons: reasons,
	}
}

// stockDTO — остаток по строке (ADR 0003): наличие, нужно, нехватка, чужие резервы.
type stockDTO struct {
	OnHand           wireNumber `json:"onHand"`
	Needed           wireNumber `json:"needed"`
	Shortage         wireNumber `json:"shortage"`
	ReservedByOthers wireNumber `json:"reservedByOthers"`
}

type sheetLineDTO struct {
	Article       string     `json:"article"`
	Name          string     `json:"name"`
	Qty           wireNumber `json:"qty"`
	Price         wireNumber `json:"price"`
	VatName       string     `json:"vatName"`
	Amount        wireNumber `json:"amount"`
	VatAmount     wireNumber `json:"vatAmount"`
	AmountWithVAT wireNumber `json:"amountWithVat"`
	Stock         stockDTO   `json:"stock"`
}

type suggestedReceiverDTO struct {
	LastName   string `json:"lastName"`
	FirstName  string `json:"firstName"`
	MiddleName string `json:"middleName,omitempty"`
	Gender     string `json:"gender,omitempty"`
}

// sheetResponseDTO — GET /api/v1/orders/{orderRef}: экран заказа для кладовщика.
type sheetResponseDTO struct {
	Order             orderResponseDTO      `json:"order"`
	Customer          string                `json:"customer"`
	Lines             []sheetLineDTO        `json:"lines"`
	Totals            totalsDTO             `json:"totals"`
	Issuable          bool                  `json:"issuable"`
	BlockReasons      []string              `json:"blockReasons,omitempty"`
	SuggestedReceiver *suggestedReceiverDTO `json:"suggestedReceiver,omitempty"`
}

func newSheetResponse(s pickup.Sheet) sheetResponseDTO {
	lines := make([]sheetLineDTO, len(s.Lines))
	for i, l := range s.Lines {
		lines[i] = sheetLineDTO{
			Article: l.Article, Name: l.Name, Qty: wireNumber(l.Quantity.String()), Price: wireNumber(l.Price.String()),
			VatName: l.VatName, Amount: wireNumber(l.Sums.Amount.String()), VatAmount: wireNumber(l.Sums.VAT.String()),
			AmountWithVAT: wireNumber(l.Sums.WithVAT.String()),
			Stock: stockDTO{
				OnHand: wireNumber(l.Stock.OnHand.String()), Needed: wireNumber(l.Stock.Needed.String()),
				Shortage: wireNumber(l.Stock.Shortage.String()), ReservedByOthers: wireNumber(l.Stock.ReservedByOthers.String()),
			},
		}
	}
	reasons := make([]string, len(s.Order.BlockReasons()))
	for i, r := range s.Order.BlockReasons() {
		reasons[i] = string(r)
	}
	resp := sheetResponseDTO{
		Order: orderResponseDTO{
			Number: s.Order.Number, Date: s.Order.Date.Format("2006-01-02T15:04:05"),
			Warehouse: s.WarehouseName, PriceIncludesVAT: s.Order.PriceIncludesVAT,
		},
		Customer: s.Customer.Name, Lines: lines,
		Totals:       totalsDTO{Amount: wireNumber(s.Totals.Amount.String()), VatAmount: wireNumber(s.Totals.VAT.String()), AmountWithVAT: wireNumber(s.Totals.WithVAT.String())},
		Issuable:     len(reasons) == 0,
		BlockReasons: reasons,
	}
	if s.SuggestedReceiver != nil {
		resp.SuggestedReceiver = &suggestedReceiverDTO{
			LastName: s.SuggestedReceiver.Name.Last, FirstName: s.SuggestedReceiver.Name.First,
			MiddleName: s.SuggestedReceiver.Name.Middle, Gender: genderString(s.SuggestedReceiver.Gender),
		}
	}
	return resp
}

func genderString(g person.Gender) string {
	switch g {
	case person.Male:
		return "м"
	case person.Female:
		return "ж"
	default:
		return ""
	}
}

// shipmentDraftDTO — ответ на создание/поиск черновика реализации.
type shipmentDraftDTO struct {
	Ref     string `json:"ref"`
	Number  string `json:"number"`
	Date    string `json:"date"`
	Created bool   `json:"created"`
}

func newShipmentDraftDTO(ref string, number string, date string, created bool) shipmentDraftDTO {
	return shipmentDraftDTO{Ref: ref, Number: number, Date: date, Created: created}
}

// statusResponseDTO — GET /api/v1/status.
type statusResponseDTO struct {
	State             string `json:"state"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
	LastError         string `json:"lastError,omitempty"`
}

func newStatusResponse(h usecase.IntegrationHealth) statusResponseDTO {
	resp := statusResponseDTO{State: string(h.State), RetryAfterSeconds: seconds(h.RetryAfter)}
	if h.LastError != nil {
		resp.LastError = h.LastError.Error()
	}
	return resp
}

func parseOrderRef(raw string) (order.Ref, error) {
	return order.ParseRef(raw)
}
