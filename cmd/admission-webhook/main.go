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

	"github.com/mmmihaeel/go-authorization-platform/internal/admission"
)

var version = "dev"
var commit = "unknown"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	address := env("WEBHOOK_ADDRESS", ":8443")
	certFile := env("WEBHOOK_TLS_CERT_FILE", "/tls/tls.crt")
	keyFile := env("WEBHOOK_TLS_KEY_FILE", "/tls/tls.key")

	mux := http.NewServeMux()
	mux.Handle("POST /validate", admission.Handler{})
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	logger.Info("admission webhook listening", "address", address, "version", version, "commit", commit)
	go func() {
		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve admission webhook", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("shutdown admission webhook", "error", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
