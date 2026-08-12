package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServiceName       string
	HTTPAddress       string
	AuthorizationMode string
	OpenFGAURL        string
	OpenFGAStoreID    string
	OpenFGAModelID    string
	OpenFGAAPIToken   string
	OPAURL            string
	OPADecisionPath   string
	DependencyTimeout time.Duration
	AuditRequired     bool
	OTLPEndpoint      string
}

func Load() (Config, error) {
	timeout, err := time.ParseDuration(getEnv("AUTHZ_DEPENDENCY_TIMEOUT", "750ms"))
	if err != nil {
		return Config{}, fmt.Errorf("AUTHZ_DEPENDENCY_TIMEOUT: %w", err)
	}
	auditRequired, err := strconv.ParseBool(getEnv("AUTHZ_AUDIT_REQUIRED", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("AUTHZ_AUDIT_REQUIRED: %w", err)
	}

	config := Config{
		ServiceName:       getEnv("SERVICE_NAME", "authz-api"),
		HTTPAddress:       getEnv("HTTP_ADDRESS", ":8080"),
		AuthorizationMode: strings.ToLower(getEnv("AUTHZ_MODE", "demo")),
		OpenFGAURL:        getEnv("OPENFGA_URL", "http://localhost:8081"),
		OpenFGAStoreID:    os.Getenv("OPENFGA_STORE_ID"),
		OpenFGAModelID:    os.Getenv("OPENFGA_MODEL_ID"),
		OpenFGAAPIToken:   os.Getenv("OPENFGA_API_TOKEN"),
		OPAURL:            getEnv("OPA_URL", "http://localhost:8181"),
		OPADecisionPath:   getEnv("OPA_DECISION_PATH", "authz/decision"),
		DependencyTimeout: timeout,
		AuditRequired:     auditRequired,
		OTLPEndpoint:      getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
	}
	if config.AuthorizationMode != "demo" && config.AuthorizationMode != "remote" {
		return Config{}, fmt.Errorf("AUTHZ_MODE must be demo or remote")
	}
	if config.AuthorizationMode == "remote" && config.OpenFGAStoreID == "" {
		return Config{}, fmt.Errorf("OPENFGA_STORE_ID is required when AUTHZ_MODE=remote")
	}
	return config, nil
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
