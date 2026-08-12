package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmmihaeel/go-authorization-platform/internal/audit"
	"github.com/mmmihaeel/go-authorization-platform/internal/authorization"
	"github.com/mmmihaeel/go-authorization-platform/internal/observability"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	service, err := authorization.NewService(authorization.DemoRelationshipChecker{}, authorization.DemoPolicyEvaluator{}, audit.NewMemoryRecorder(), authorization.ServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouter(service, observability.NewMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil)), RouterConfig{TrustRequestSubject: true})
	if err != nil {
		t.Fatal(err)
	}
	return router.Handler()
}

func TestDecisionEndpoint(t *testing.T) {
	payload := map[string]any{
		"subject":  map[string]any{"type": "user", "id": "alice", "tenant_id": "acme", "roles": []string{"tenant_admin"}},
		"resource": map[string]any{"type": "document", "id": "roadmap", "tenant_id": "acme", "attributes": map[string]any{"classification": "internal"}},
		"action":   "view",
		"context":  map[string]any{"device_trusted": true, "mfa": true, "risk_score": 5},
	}
	body, _ := json.Marshal(payload)
	request := httptest.NewRequest(http.MethodPost, "/v1/decisions/check", bytes.NewReader(body))
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var decision authorization.Decision
	if err := json.NewDecoder(response.Body).Decode(&decision); err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed {
		t.Fatalf("unexpected decision %#v", decision)
	}
}

func TestDecisionEndpointRejectsUnknownFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/decisions/check", bytes.NewBufferString(`{"unknown":true}`))
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDecisionEndpointRejectsSecondJSONValue(t *testing.T) {
	payload := `{"subject":{"id":"alice","tenant_id":"acme"},"resource":{"type":"document","id":"roadmap","tenant_id":"acme"},"action":"view"} {"extra":true}`
	request := httptest.NewRequest(http.MethodPost, "/v1/decisions/check", bytes.NewBufferString(payload))
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDecisionEndpointRejectsMalformedTrailingJSON(t *testing.T) {
	payload := `{"subject":{"id":"alice","tenant_id":"acme"},"resource":{"type":"document","id":"roadmap","tenant_id":"acme"},"action":"view"} {`
	request := httptest.NewRequest(http.MethodPost, "/v1/decisions/check", bytes.NewBufferString(payload))
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMetricsEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d", response.Code)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("authz_decisions_total")) {
		t.Fatalf("unexpected metrics: %s", response.Body.String())
	}
}

func TestExtAuthzExactPathRequiresResourceID(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/ext-authz", nil)
	request.Header.Set("X-Subject-ID", "alice")
	request.Header.Set("X-Tenant-ID", "acme")
	request.Header.Set("X-Subject-Roles", "tenant_admin")
	request.Header.Set("X-Device-Trusted", "true")
	request.Header.Set("X-MFA", "true")
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestExtAuthzContract(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/ext-authz/documents/roadmap", nil)
	request.Header.Set("X-Subject-ID", "alice")
	request.Header.Set("X-Tenant-ID", "acme")
	request.Header.Set("X-Subject-Roles", "tenant_admin")
	request.Header.Set("X-Resource-Type", "document")
	request.Header.Set("X-Device-Trusted", "true")
	request.Header.Set("X-MFA", "true")
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Authorization-Decision-ID") == "" {
		t.Fatal("expected decision ID response header")
	}
}

// Compile-time check that a context-aware checker remains easy to fake.
type contextChecker struct{}

func (contextChecker) Check(context.Context, authorization.RelationshipQuery) (bool, error) {
	return true, nil
}
