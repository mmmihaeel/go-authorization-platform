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
	OpenFGAURL        string
	OpenFGAStoreID    string
	OpenFGAStoreName  string
	OpenFGAModelID    string
	OpenFGAAPIToken   string
	OPAURL            string
	OPADecisionPath   string
	DependencyTimeout time.Duration
	AuditRequired     bool
	OTLPEndpoint      string
	DatabaseURL       string
	RabbitMQURL       string
	RabbitMQExchange  string
	RabbitMQQueue     string
	OIDCIssuer        string
	OIDCJWKSURL       string
	OIDCAudience      string
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
		OpenFGAURL:        getEnv("OPENFGA_URL", "http://localhost:8081"),
		OpenFGAStoreID:    os.Getenv("OPENFGA_STORE_ID"),
		OpenFGAStoreName:  getEnv("OPENFGA_STORE_NAME", "Go Authorization Platform"),
		OpenFGAModelID:    os.Getenv("OPENFGA_MODEL_ID"),
		OpenFGAAPIToken:   os.Getenv("OPENFGA_API_TOKEN"),
		OPAURL:            getEnv("OPA_URL", "http://localhost:8181"),
		OPADecisionPath:   getEnv("OPA_DECISION_PATH", "authz/decision"),
		DependencyTimeout: timeout,
		AuditRequired:     auditRequired,
		OTLPEndpoint:      getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		RabbitMQURL:       os.Getenv("RABBITMQ_URL"),
		RabbitMQExchange:  getEnv("RABBITMQ_EXCHANGE", "authorization.audit"),
		RabbitMQQueue:     getEnv("RABBITMQ_QUEUE", "authorization.audit.log"),
		OIDCIssuer:        os.Getenv("KEYCLOAK_ISSUER"),
		OIDCJWKSURL:       os.Getenv("OIDC_JWKS_URL"),
		OIDCAudience:      getEnv("OIDC_AUDIENCE", "authz-api"),
	}
	if config.DependencyTimeout <= 0 {
		return Config{}, fmt.Errorf("AUTHZ_DEPENDENCY_TIMEOUT must be positive")
	}
	return config, nil
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
