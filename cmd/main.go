// Command external-dns-technitium-dns is an external-dns webhook provider for Technitium DNS Server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xruins/external-dns-technitium-dns/internal/config"
	"github.com/xruins/external-dns-technitium-dns/internal/technitium"
	"github.com/xruins/external-dns-technitium-dns/internal/webhook"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Configure structured JSON logging.
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "WARN":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	// Authenticate with Technitium DNS.
	client, err := technitium.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("creating Technitium client: %w", err)
	}
	ctx := context.Background()
	if err := client.Login(ctx); err != nil {
		return fmt.Errorf("authenticating with Technitium: %w", err)
	}

	webhookServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		Handler:      webhook.NewServer(cfg, client),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Trap SIGINT and SIGTERM for graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("Webhook server listening", "addr", webhookServer.Addr)
		if err := webhookServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Webhook server error", "error", err)
		}
	}()

	<-quit
	slog.Info("Shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := webhookServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("Webhook server shutdown error", "error", err)
	}

	return nil
}
