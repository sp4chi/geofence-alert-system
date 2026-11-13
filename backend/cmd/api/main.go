// Command api is the geofencing HTTP + WebSocket server.
// main only wires pieces together: config → db → migrations → router → server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"geofencing/internal/config"
	"geofencing/internal/httpapi"
	"geofencing/internal/store"
	"geofencing/migrations"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

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

	// Cancelled on Ctrl+C or SIGTERM (what Docker sends on stop) to trigger a graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := store.Migrate(ctx, pool, migrations.Files); err != nil {
		return err
	}
	db := store.New(pool)

	server := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: httpapi.NewRouter(httpapi.Options{
			Health:      db,
			Store:       db,
			CORSOrigins: cfg.CORSOrigins,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		// No WriteTimeout: it would kill long-lived WebSocket connections on /ws/alerts.
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", server.Addr)
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
