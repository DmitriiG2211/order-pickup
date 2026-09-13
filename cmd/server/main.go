// Command server starts the order-pickup service on port 3000.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orderissue/internal/infrastructure/onec"
	"orderissue/internal/infrastructure/pdf"
	"orderissue/internal/infrastructure/petrovich"
	"orderissue/internal/service"
	"orderissue/internal/transport/httpapi"
	"orderissue/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("сервис остановлен с ошибкой", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	clock := realClock{}
	cfg := onec.DefaultConfig()
	cfg.BaseURL = os.Getenv("ONEC_BASE_URL")
	cfg.User = os.Getenv("ONEC_USER")
	cfg.Password = os.Getenv("ONEC_PASSWORD")
	client, err := onec.NewClient(cfg, clock, logger)
	if err != nil {
		return err
	}
	inflector, err := petrovich.New()
	if err != nil {
		return err
	}
	gateway := onec.NewGateway(client)
	implementation := service.New(usecase.Deps{
		Orders: gateway, Catalog: gateway, Stock: gateway, Shipments: gateway,
		Renderer: pdf.New(), Inflector: inflector, Health: client, Clock: clock,
		ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second,
	})
	addr := envOr("ADDR", ":3000")
	srv := &http.Server{Addr: addr, Handler: httpapi.NewServer(implementation), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errs := make(chan error, 1)
	go func() {
		logger.Info("сервис выдачи заказов запущен", "addr", addr)
		errs <- srv.ListenAndServe()
	}()
	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("остановка HTTP-сервера: %w", err)
		}
		return nil
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
func (realClock) Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
