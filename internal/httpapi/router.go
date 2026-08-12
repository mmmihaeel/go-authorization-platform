package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mmmihaeel/go-authorization-platform/internal/authn"
	"github.com/mmmihaeel/go-authorization-platform/internal/authorization"
	"github.com/mmmihaeel/go-authorization-platform/internal/observability"
)

type RouterConfig struct {
	ServiceName         string
	TrustRequestSubject bool
	MaxRequestBodyBytes int64
}

type Router struct {
	service *authorization.Service
	metrics *observability.Metrics
	logger  *slog.Logger
	config  RouterConfig
}

func NewRouter(service *authorization.Service, metrics *observability.Metrics, logger *slog.Logger, config RouterConfig) (*Router, error) {
	if service == nil {
		return nil, errors.New("authorization service is required")
	}
	if metrics == nil {
		metrics = observability.NewMetrics()
	}
	if logger == nil {
		logger = slog.Default()
	}
	if config.ServiceName == "" {
		config.ServiceName = "authz-api"
	}
	if config.MaxRequestBodyBytes <= 0 {
		config.MaxRequestBodyBytes = 1 << 20
	}
	return &Router{service: service, metrics: metrics, logger: logger, config: config}, nil
}

func (r *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", r.index)
	mux.HandleFunc("GET /health/live", r.live)
	mux.HandleFunc("GET /health/ready", r.ready)
	mux.HandleFunc("GET /metrics", r.prometheus)
	mux.HandleFunc("POST /v1/decisions/check", r.check)
	mux.HandleFunc("/v1/ext-authz", r.extAuthz)
	mux.HandleFunc("/v1/ext-authz/", r.extAuthz)
	return r.requestLog(r.requestID(mux))
}

func (r *Router) index(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service":   r.config.ServiceName,
		"status":    "ok",
		"endpoints": []string{"POST /v1/decisions/check", "ANY /v1/ext-authz/*", "GET /health/live", "GET /metrics"},
	})
}

func (r *Router) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "live"})
}
func (r *Router) ready(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (r *Router) prometheus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	if err := r.metrics.WritePrometheus(w); err != nil {
		r.logger.Error("write metrics", "error", err)
	}
}

func (r *Router) check(w http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(w, request.Body, r.config.MaxRequestBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input authorization.CheckRequest
	if err := decoder.Decode(&input); err != nil {
		r.metrics.RecordDecision(false, true)
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		r.metrics.RecordDecision(false, true)
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if claims, ok := authn.FromContext(request.Context()); ok {
		if input.Subject.ID != "" && input.Subject.ID != claims.Subject {
			r.metrics.RecordDecision(false, true)
			writeError(w, http.StatusForbidden, "subject_mismatch", "request subject must match the authenticated token")
			return
		}
		if input.Subject.TenantID != "" && input.Subject.TenantID != claims.TenantID {
			r.metrics.RecordDecision(false, true)
			writeError(w, http.StatusForbidden, "tenant_mismatch", "request tenant must match the authenticated token")
			return
		}
		input.Subject.ID = claims.Subject
		input.Subject.TenantID = claims.TenantID
		input.Subject.Roles = append([]string(nil), claims.Roles...)
	} else if !r.config.TrustRequestSubject {
		r.metrics.RecordDecision(false, true)
		writeError(w, http.StatusUnauthorized, "authentication_required", "no authenticated subject is present")
		return
	}

	if input.CorrelationID == "" {
		input.CorrelationID = request.Header.Get("X-Request-ID")
	}
	decision, err := r.service.Check(request.Context(), input)
	r.metrics.RecordDecision(decision.Allowed, err != nil)
	if err != nil {
		status := http.StatusInternalServerError
		code := "authorization_failed"
		if errors.Is(err, authorization.ErrInvalidRequest) {
			status, code = http.StatusBadRequest, "invalid_request"
		}
		var dependencyErr *authorization.DependencyError
		if errors.As(err, &dependencyErr) {
			status, code = http.StatusServiceUnavailable, "dependency_unavailable"
		}
		writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": err.Error()}, "decision": decision})
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

// extAuthz exposes the HTTP contract expected by Envoy's ext_authz filter.
// Identity headers must be injected by a trusted JWT validation layer; they
// must never be accepted directly from untrusted internet clients.
func (r *Router) extAuthz(w http.ResponseWriter, request *http.Request) {
	action := strings.ToLower(strings.TrimSpace(request.Header.Get("X-Action")))
	if action == "" {
		switch request.Method {
		case http.MethodGet, http.MethodHead:
			action = "view"
		case http.MethodPost:
			action = "create"
		case http.MethodPut, http.MethodPatch:
			action = "edit"
		case http.MethodDelete:
			action = "delete"
		default:
			action = "view"
		}
	}

	resourceID := strings.TrimSpace(request.Header.Get("X-Resource-ID"))
	const extAuthzPrefix = "/v1/ext-authz/"
	if resourceID == "" && strings.HasPrefix(request.URL.Path, extAuthzPrefix) {
		trimmed := strings.TrimPrefix(request.URL.Path, extAuthzPrefix)
		parts := strings.Split(strings.Trim(trimmed, "/"), "/")
		if len(parts) > 0 {
			resourceID = parts[len(parts)-1]
		}
	}
	resourceType := strings.TrimSpace(request.Header.Get("X-Resource-Type"))
	if resourceType == "" {
		resourceType = "document"
	}

	riskScore, _ := strconv.Atoi(request.Header.Get("X-Risk-Score"))
	mfa, _ := strconv.ParseBool(request.Header.Get("X-MFA"))
	deviceTrusted, _ := strconv.ParseBool(request.Header.Get("X-Device-Trusted"))
	roles := splitCSV(request.Header.Get("X-Subject-Roles"))
	attributes := map[string]any{}
	if ownerID := strings.TrimSpace(request.Header.Get("X-Resource-Owner-ID")); ownerID != "" {
		attributes["owner_id"] = ownerID
	}
	if classification := strings.TrimSpace(request.Header.Get("X-Resource-Classification")); classification != "" {
		attributes["classification"] = classification
	}

	input := authorization.CheckRequest{
		Subject: authorization.Subject{
			Type:     "user",
			ID:       strings.TrimSpace(request.Header.Get("X-Subject-ID")),
			TenantID: strings.TrimSpace(request.Header.Get("X-Tenant-ID")),
			Roles:    roles,
		},
		Resource: authorization.Resource{
			Type:       resourceType,
			ID:         resourceID,
			TenantID:   strings.TrimSpace(request.Header.Get("X-Tenant-ID")),
			Attributes: attributes,
		},
		Action:        action,
		Relation:      strings.TrimSpace(request.Header.Get("X-Relation")),
		CorrelationID: request.Header.Get("X-Request-ID"),
		Context: authorization.RequestContext{
			IPAddress:     request.Header.Get("X-Forwarded-For"),
			DeviceTrusted: deviceTrusted,
			MFA:           mfa,
			RiskScore:     riskScore,
			Purpose:       request.Header.Get("X-Access-Purpose"),
		},
	}

	decision, err := r.service.Check(request.Context(), input)
	r.metrics.RecordDecision(decision.Allowed, err != nil)
	w.Header().Set("X-Authorization-Decision-ID", decision.ID)
	if err != nil {
		var dependencyErr *authorization.DependencyError
		if errors.As(err, &dependencyErr) {
			writeJSON(w, http.StatusServiceUnavailable, decision)
			return
		}
		writeJSON(w, http.StatusBadRequest, decision)
		return
	}
	if !decision.Allowed {
		writeJSON(w, http.StatusForbidden, decision)
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (r *Router) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		id := strings.TrimSpace(request.Header.Get("X-Request-ID"))
		if id == "" {
			id = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		request.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, request)
	})
}

func (r *Router) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, request)
		r.logger.Info("http request", "method", request.Method, "path", request.URL.Path, "request_id", request.Header.Get("X-Request-ID"), "duration_ms", float64(time.Since(started).Microseconds())/1000)
	})
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	switch {
	case errors.Is(err, io.EOF):
		return nil
	case err == nil:
		return errors.New("request body must contain exactly one JSON value")
	default:
		return fmt.Errorf("invalid trailing JSON: %w", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
