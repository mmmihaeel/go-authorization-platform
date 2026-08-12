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

	"github.com/mmmihaeel/go-authorization-platform/internal/audit"
	"github.com/mmmihaeel/go-authorization-platform/internal/authn"
	"github.com/mmmihaeel/go-authorization-platform/internal/authorization"
	"github.com/mmmihaeel/go-authorization-platform/internal/config"
	"github.com/mmmihaeel/go-authorization-platform/internal/httpapi"
	"github.com/mmmihaeel/go-authorization-platform/internal/observability"
	"github.com/mmmihaeel/go-authorization-platform/internal/platform"
)

var version = "dev"
var commit = "unknown"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	if len(os.Args) == 3 && os.Args[1] == "healthcheck" {
		if err := healthcheck(os.Args[2]); err != nil {
			os.Exit(1)
		}
		return
	}

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

	db, err := platform.OpenPostgres(cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect audit database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	auditRepository, err := audit.NewSQLRepository(db)
	if err != nil {
		logger.Error("configure audit repository", "error", err)
		os.Exit(1)
	}

	relationships, policies, err := authorizationBackends(rootContext, cfg)
	if err != nil {
		logger.Error("configure authorization backends", "error", err)
		os.Exit(1)
	}

	verifier, err := authn.NewOIDCVerifier(rootContext, cfg.OIDCIssuer, cfg.OIDCAudience, cfg.OIDCJWKSURL)
	if err != nil {
		logger.Error("configure OIDC verifier", "error", err)
		os.Exit(1)
	}
	service, err := authorization.NewService(relationships, policies, auditRepository, authorization.ServiceConfig{AuditRequired: cfg.AuditRequired})
	if err != nil {
		logger.Error("create authorization service", "error", err)
		os.Exit(1)
	}
	router, err := httpapi.NewRouter(service, observability.NewMetrics(), logger, httpapi.RouterConfig{
		ServiceName: cfg.ServiceName,
		Verifier:    verifier,
		Resources: httpapi.StaticResourceResolver{
			"document:roadmap":       {Type: "document", ID: "roadmap", TenantID: "acme", Attributes: map[string]any{"classification": "internal"}},
			"document:security-plan": {Type: "document", ID: "security-plan", TenantID: "acme", Attributes: map[string]any{"classification": "restricted"}},
			"document:globex-plan":   {Type: "document", ID: "globex-plan", TenantID: "globex", Attributes: map[string]any{"classification": "internal"}},
		},
		Readiness: db.PingContext,
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
		MaxHeaderBytes:    32 << 10,
	}

	go func() {
		logger.Info("authorization API listening", "address", cfg.HTTPAddress, "version", version, "commit", commit)
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

func healthcheck(endpoint string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %s", response.Status)
	}
	return nil
}

func authorizationBackends(ctx context.Context, cfg config.Config) (authorization.RelationshipChecker, authorization.ContextPolicyEvaluator, error) {
	openFGAConfig, err := authorization.DiscoverOpenFGAConfig(ctx, authorization.OpenFGAConfig{
		BaseURL: cfg.OpenFGAURL, StoreID: cfg.OpenFGAStoreID, StoreName: cfg.OpenFGAStoreName,
		AuthorizationModelID: cfg.OpenFGAModelID, APIToken: cfg.OpenFGAAPIToken, Timeout: cfg.DependencyTimeout,
	})
	if err != nil {
		return nil, nil, err
	}
	openFGA, err := authorization.NewOpenFGAClient(openFGAConfig)
	if err != nil {
		return nil, nil, err
	}
	opa, err := authorization.NewOPAClient(authorization.OPAConfig{BaseURL: cfg.OPAURL, DecisionPath: cfg.OPADecisionPath, Timeout: cfg.DependencyTimeout})
	if err != nil {
		return nil, nil, err
	}
	return openFGA, opa, nil
}
