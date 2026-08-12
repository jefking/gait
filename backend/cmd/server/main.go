package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jefking/gait/backend/internal/api"
	"github.com/jefking/gait/backend/internal/dashboard"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	port := envOrDefault("PORT", "8080")
	staticDir := envOrDefault("STATIC_DIR", "../frontend/dist")
	dataDir := envOrDefault("DATA_DIR", "./data")
	concurrency := envPositiveInteger("SYNC_CONCURRENCY", 4)
	manager, err := dashboard.NewManager(dashboard.ManagerConfig{
		DataDir:     dataDir,
		Concurrency: concurrency,
	})
	if err != nil {
		return err
	}
	defer manager.Close()

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           api.NewRouter(staticDir, manager),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("server listening", "address", server.Addr, "static_dir", staticDir)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		slog.Info("shutting down server")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	err = <-serverErrors
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}

func envPositiveInteger(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
