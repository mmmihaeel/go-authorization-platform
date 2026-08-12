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

	"github.com/mmmihaeel/go-authorization-platform/internal/audit"
	"github.com/mmmihaeel/go-authorization-platform/internal/authorization"
	"github.com/mmmihaeel/go-authorization-platform/internal/config"
	"github.com/mmmihaeel/go-authorization-platform/internal/httpapi"
	"github.com/mmmihaeel/go-authorization-platform/internal/observability"
)

var version = "dev"
var commit = "unknown"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	rootContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.SetupTracing(rootContext, cfg.ServiceName, cfg.OTLPEndpoint)
	if err != nil {
		logger.Error("setup tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(ctx); err != nil {
			logger.Error("shutdown tracing", "error", err)
		}
	}()

	relationships, policies, err := authorizationBackends(cfg)
	if err != nil {
		logger.Error("configure authorization backends", "error", err)
		os.Exit(1)
	}

	service, err := authorization.NewService(relationships, policies, audit.NewMemoryRecorder(), authorization.ServiceConfig{AuditRequired: cfg.AuditRequired})
	if err != nil {
		logger.Error("create authorization service", "error", err)
		os.Exit(1)
	}
	router, err := httpapi.NewRouter(service, observability.NewMetrics(), logger, httpapi.RouterConfig{
		ServiceName:         cfg.ServiceName,
		TrustRequestSubject: true, // Demo baseline. Use authn.RequireBearer in production wiring.
	})
	if err != nil {
		logger.Error("create HTTP router", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           observability.HTTPMiddleware("authz-api", router.Handler()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("authorization API listening", "address", cfg.HTTPAddress, "mode", cfg.AuthorizationMode, "version", version, "commit", commit)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve HTTP", "error", err)
			stop()
		}
	}()

	<-rootContext.Done()
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("graceful shutdown", "error", err)
	}
}

func authorizationBackends(cfg config.Config) (authorization.RelationshipChecker, authorization.ContextPolicyEvaluator, error) {
	if cfg.AuthorizationMode == "demo" {
		return authorization.DemoRelationshipChecker{}, authorization.DemoPolicyEvaluator{}, nil
	}
	openFGA, err := authorization.NewOpenFGAClient(authorization.OpenFGAConfig{
		BaseURL: cfg.OpenFGAURL, StoreID: cfg.OpenFGAStoreID, AuthorizationModelID: cfg.OpenFGAModelID,
		APIToken: cfg.OpenFGAAPIToken, Timeout: cfg.DependencyTimeout,
	})
	if err != nil {
		return nil, nil, err
	}
	opa, err := authorization.NewOPAClient(authorization.OPAConfig{BaseURL: cfg.OPAURL, DecisionPath: cfg.OPADecisionPath, Timeout: cfg.DependencyTimeout})
	if err != nil {
		return nil, nil, err
	}
	return openFGA, opa, nil
}
