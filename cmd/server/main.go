// Command server runs the service: settings from the environment, the
// handler from internal/server, and a stop that lets requests in flight
// finish (SIGINT, SIGTERM).
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

	"github.com/tinhthanh/go-service-template/internal/platform/config"
	"github.com/tinhthanh/go-service-template/internal/platform/db"
	"github.com/tinhthanh/go-service-template/internal/platform/logging"
	"github.com/tinhthanh/go-service-template/internal/server"
)

const _readHeaderTimeout = 5 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logging.New(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.MigrateOnStart {
		if err := db.Migrate(cfg.DatabaseURL); err != nil {
			return err
		}
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	features, err := server.Wire(&cfg, pool)
	if err != nil {
		return err
	}

	handler, err := server.New(&cfg, log, features)
	if err != nil {
		return err
	}

	srv := &http.Server{Addr: cfg.Addr, Handler: handler, ReadHeaderTimeout: _readHeaderTimeout}

	failed := make(chan error, 1)

	go func() {
		log.Info("listening", "addr", cfg.Addr, "api", cfg.APIPrefix)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			failed <- err
		}
	}()

	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}

	log.Info("stopping")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server: stop: %w", err)
	}

	return nil
}
