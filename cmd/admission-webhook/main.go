package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/mmmihaeel/go-authorization-platform/internal/admission"
)

var version = "dev"
var commit = "unknown"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	address := env("WEBHOOK_ADDRESS", ":8443")
	certFile := env("WEBHOOK_TLS_CERT_FILE", "/tls/tls.crt")
	keyFile := env("WEBHOOK_TLS_KEY_FILE", "/tls/tls.key")

	mux := http.NewServeMux()
	mux.Handle("POST /validate", admission.Handler{})
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	logger.Info("admission webhook listening", "address", address, "version", version, "commit", commit)
	if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
		logger.Error("serve admission webhook", "error", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
