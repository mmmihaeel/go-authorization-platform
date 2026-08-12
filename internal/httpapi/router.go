package httpapi

import (
	"context"
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
	Verifier            authn.Verifier
	Resources           ResourceResolver
	Readiness           func(context.Context) error
}

type ResourceResolver interface {
	Resolve(context.Context, string, string) (authorization.Resource, error)
}

type StaticResourceResolver map[string]authorization.Resource

func (s StaticResourceResolver) Resolve(_ context.Context, resourceType, resourceID string) (authorization.Resource, error) {
	resource, ok := s[resourceType+":"+resourceID]
	if !ok {
		return authorization.Resource{}, fmt.Errorf("resource %s:%s was not found", resourceType, resourceID)
	}
	resource.Attributes = cloneMap(resource.Attributes)
	return resource, nil
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
	check := http.Handler(http.HandlerFunc(r.check))
	extAuthz := http.Handler(http.HandlerFunc(r.extAuthz))
	if !r.config.TrustRequestSubject {
		check = authn.RequireBearer(r.config.Verifier, check)
		extAuthz = authn.RequireBearer(r.config.Verifier, extAuthz)
	}
	mux.Handle("POST /v1/decisions/check", check)
	mux.Handle("/v1/ext-authz", extAuthz)
	mux.Handle("/v1/ext-authz/", extAuthz)
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

func (r *Router) ready(w http.ResponseWriter, request *http.Request) {
	if r.config.Readiness != nil {
		ctx, cancel := context.WithTimeout(request.Context(), time.Second)
		defer cancel()
		if err := r.config.Readiness(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "not_ready", "required persistence dependency is unavailable")
			return
		}
	}
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
		input.Subject.Type = claims.SubjectType
		input.Subject.TenantID = claims.TenantID
		input.Subject.Roles = append([]string(nil), claims.Roles...)
		input.Subject.Claims = map[string]any{"authorized_party": claims.AuthorizedBy, "actor": claims.Actor}
	} else if !r.config.TrustRequestSubject {
		r.metrics.RecordDecision(false, true)
		writeError(w, http.StatusUnauthorized, "authentication_required", "no authenticated subject is present")
		return
	}
	if r.config.Resources != nil {
		resource, err := r.config.Resources.Resolve(request.Context(), input.Resource.Type, input.Resource.ID)
		if err != nil {
			r.metrics.RecordDecision(false, true)
			writeError(w, http.StatusNotFound, "resource_not_found", err.Error())
			return
		}
		input.Resource = resource
	}

	if input.CorrelationID == "" {
		input.CorrelationID = request.Header.Get("X-Request-ID")
	}
	decision, err := r.service.Check(request.Context(), input)
	r.metrics.RecordDecision(decision.Allowed, err != nil)
	r.metrics.RecordLatency(decision.LatencyMS)
	r.metrics.RecordAudit(decision.AuditRecorded)
	if err != nil {
		status := http.StatusInternalServerError
		code := "authorization_failed"
		message := "authorization request failed"
		if errors.Is(err, authorization.ErrInvalidRequest) {
			status, code, message = http.StatusBadRequest, "invalid_request", err.Error()
		}
		var dependencyErr *authorization.DependencyError
		if errors.As(err, &dependencyErr) {
			status, code, message = http.StatusServiceUnavailable, "dependency_unavailable", "an authorization dependency is unavailable"
		}
		var auditErr *authorization.AuditError
		if errors.As(err, &auditErr) {
			status, code, message = http.StatusServiceUnavailable, "audit_unavailable", "authorization audit persistence is unavailable"
		}
		writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message}, "decision": decision})
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

// extAuthz exposes the HTTP contract expected by Envoy's ext_authz filter.
// Identity and MFA are derived from the verified bearer token. Device, risk,
// and purpose headers are edge-provided context and require a trusted hop.
func (r *Router) extAuthz(w http.ResponseWriter, request *http.Request) {
	claims, ok := authn.FromContext(request.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "no authenticated subject is present")
		return
	}
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
	if resourceID == "" {
		writeError(w, http.StatusBadRequest, "invalid_resource", "resource ID is required")
		return
	}

	riskScore, err := parseIntHeader(request, "X-Risk-Score", 0, 100)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_context", err.Error())
		return
	}
	deviceTrusted, err := parseBoolHeader(request, "X-Device-Trusted")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_context", err.Error())
		return
	}
	if r.config.Resources == nil {
		writeError(w, http.StatusServiceUnavailable, "resource_resolver_unavailable", "resource resolver is not configured")
		return
	}
	resource, err := r.config.Resources.Resolve(request.Context(), resourceType, resourceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "resource_not_found", err.Error())
		return
	}

	input := authorization.CheckRequest{
		Subject: authorization.Subject{
			Type: claims.SubjectType, ID: claims.Subject, TenantID: claims.TenantID,
			Roles:  append([]string(nil), claims.Roles...),
			Claims: map[string]any{"authorized_party": claims.AuthorizedBy, "actor": claims.Actor},
		},
		Resource:      resource,
		Action:        action,
		Relation:      strings.TrimSpace(request.Header.Get("X-Relation")),
		CorrelationID: request.Header.Get("X-Request-ID"),
		Context: authorization.RequestContext{
			IPAddress:     request.RemoteAddr,
			DeviceTrusted: deviceTrusted,
			MFA:           claims.MFA,
			RiskScore:     riskScore,
			Purpose:       request.Header.Get("X-Access-Purpose"),
		},
	}

	decision, err := r.service.Check(request.Context(), input)
	r.metrics.RecordDecision(decision.Allowed, err != nil)
	r.metrics.RecordLatency(decision.LatencyMS)
	r.metrics.RecordAudit(decision.AuditRecorded)
	w.Header().Set("X-Authorization-Decision-ID", decision.ID)
	if err != nil {
		var dependencyErr *authorization.DependencyError
		if errors.As(err, &dependencyErr) {
			writeJSON(w, http.StatusServiceUnavailable, decision)
			return
		}
		var auditErr *authorization.AuditError
		if errors.As(err, &auditErr) {
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

func parseIntHeader(request *http.Request, name string, minimum, maximum int) (int, error) {
	value := strings.TrimSpace(request.Header.Get(name))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", name, minimum, maximum)
	}
	return parsed, nil
}

func parseBoolHeader(request *http.Request, name string) (bool, error) {
	value := strings.TrimSpace(request.Header.Get(name))
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return parsed, nil
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
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
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
