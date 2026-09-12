package service

import (
	"context"
	"errors"

	"orderissue/internal/domain/order"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/domain/shipment"
	"orderissue/internal/usecase"
)

// EnsureShipmentDraft создаёт ровно один наш черновик реализации по заказу
// и проверяет, что в 1С записано отправленное. Порядок и причины — в ADR 0004.
func (s *Service) EnsureShipmentDraft(ctx context.Context, ref order.Ref) (usecase.ShipmentResult, error) {
	unlock := s.lockOrder(string(ref))
	defer unlock()

	ctx, cancel := s.writeContext(ctx)
	defer cancel()

	o, refs, err := s.loadOrder(ctx, ref)
	if err != nil {
		return usecase.ShipmentResult{}, err
	}
	if !o.Issuable() {
		return usecase.ShipmentResult{}, &usecase.OrderNotIssuableError{Number: o.Number, Reasons: o.BlockReasons()}
	}
	lines, _, err := pickup.PriceLines(o, refs)
	if err != nil {
		return usecase.ShipmentResult{}, err
	}
	draft := shipment.NewDraft(o, lines, s.d.Clock.Now())

	var lastErr error
	for range maxWriteCycles {
		// Сначала ищем: черновик мог остаться от прошлой печати или от
		// предыдущего цикла, где POST прошёл, а ответ потерялся.
		candidates, err := s.d.Shipments.FindByOrder(ctx, o.Ref)
		if err != nil {
			return usecase.ShipmentResult{}, err
		}
		if existing, ok := shipment.FindOurs(o.Ref, candidates); ok {
			recorded, err := s.verify(ctx, draft, existing.Ref)
			return usecase.ShipmentResult{Draft: recorded, Created: false}, err
		}

		created, err := s.d.Shipments.Create(ctx, draft)
		if err == nil {
			recorded, err := s.verify(ctx, draft, created.Ref)
			return usecase.ShipmentResult{Draft: recorded, Created: true}, err
		}

		var upErr *usecase.UpstreamError
		switch {
		case !errors.As(err, &upErr):
			return usecase.ShipmentResult{}, err
		case upErr.Outcome == usecase.OutcomeUnknown:
			// Документ мог создаться. Повторять POST вслепую нельзя — на
			// следующем круге сначала поищем его.
			lastErr = err
			continue
		case upErr.Code == usecase.UpstreamRequestRejected:
			return usecase.ShipmentResult{}, &usecase.ShipmentRejectedError{Upstream: upErr}
		default:
			return usecase.ShipmentResult{}, err
		}
	}
	return usecase.ShipmentResult{}, lastErr
}

// verify перечитывает черновик из 1С и сверяет с отправленным: код 201
// говорит лишь, что 1С что-то записала.
func (s *Service) verify(ctx context.Context, draft shipment.Draft, ref string) (shipment.Recorded, error) {
	recorded, err := s.d.Shipments.Get(ctx, ref)
	if err != nil {
		return shipment.Recorded{}, err
	}
	if mm := shipment.Verify(draft, recorded); len(mm) > 0 {
		return recorded, &usecase.ShipmentNotConfirmedError{Ref: recorded.Ref, Number: recorded.Number, Mismatches: mm}
	}
	return recorded, nil
}

// FindShipmentDraft — наш черновик по заказу, если он уже есть.
func (s *Service) FindShipmentDraft(ctx context.Context, ref order.Ref) (shipment.Recorded, error) {
	ctx, cancel := s.readContext(ctx)
	defer cancel()

	candidates, err := s.d.Shipments.FindByOrder(ctx, ref)
	if err != nil {
		return shipment.Recorded{}, err
	}
	ours, ok := shipment.FindOurs(ref, candidates)
	if !ok {
		return shipment.Recorded{}, usecase.ErrShipmentDraftNotFound
	}
	return ours, nil
}

// ServiceStatus — состояние связи с 1С без запроса в неё.
func (s *Service) ServiceStatus() usecase.IntegrationHealth {
	return s.d.Health.Health()
}
