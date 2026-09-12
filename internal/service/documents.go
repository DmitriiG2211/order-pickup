package service

import (
	"context"
	"fmt"

	"orderissue/internal/domain/pickup"
	"orderissue/internal/usecase"
)

// PreviewDocument собирает документ, даже когда заказ нельзя выдавать —
// причины лежат в документе, а запрет на печать проверяет PrintDocument.
func (s *Service) PreviewDocument(ctx context.Context, req usecase.DocumentRequest) (pickup.Document, error) {
	// Ввод проверяем до обращения к 1С: ошибка в телефоне не должна ждать
	// ответа 1С и видна, даже когда 1С лежит.
	receiver, err := pickup.NewReceiver(req.Receiver)
	if err != nil {
		return pickup.Document{}, err
	}

	ctx, cancel := s.readContext(ctx)
	defer cancel()

	o, refs, err := s.loadOrder(ctx, req.OrderRef)
	if err != nil {
		return pickup.Document{}, err
	}
	return pickup.ComposeDocument(o, refs, receiver, s.d.Inflector)
}

// PrintDocument — тот же документ в PDF; печать блокируют причины из BlockReasons.
func (s *Service) PrintDocument(ctx context.Context, req usecase.DocumentRequest) ([]byte, pickup.Document, error) {
	doc, err := s.PreviewDocument(ctx, req)
	if err != nil {
		return nil, pickup.Document{}, err
	}
	if len(doc.BlockReasons) > 0 {
		return nil, doc, &usecase.OrderNotIssuableError{Number: doc.OrderNumber, Reasons: doc.BlockReasons}
	}
	pdf, err := s.d.Renderer.Render(doc)
	if err != nil {
		return nil, doc, fmt.Errorf("формирование PDF: %w", err)
	}
	return pdf, doc, nil
}
