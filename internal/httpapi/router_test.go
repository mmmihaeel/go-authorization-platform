package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmmihaeel/go-authorization-platform/internal/authn"
	"github.com/mmmihaeel/go-authorization-platform/internal/authorization"
	"github.com/mmmihaeel/go-authorization-platform/internal/observability"
)

type checkerStub struct{}

func (checkerStub) Check(_ context.Context, query authorization.RelationshipQuery) (bool, error) {
	return query.Subject.ID == "alice", nil
}

type checkerErrorStub struct{}

func (checkerErrorStub) Check(context.Context, authorization.RelationshipQuery) (bool, error) {
	return false, errors.New("upstream response contained private detail")
}

type evaluatorStub struct{}

func (evaluatorStub) Evaluate(_ context.Context, input authorization.PolicyInput) (authorization.PolicyResult, error) {
	return authorization.PolicyResult{Allow: input.RelationshipAllowed}, nil
}

type verifierStub struct{}

type auditStub struct{}

func (auditStub) RecordDecision(context.Context, authorization.AuditRecord) error { return nil }

func (verifierStub) Verify(context.Context, string) (authn.Claims, error) {
	return authn.Claims{TokenSubject: "keycloak-id", Subject: "alice", SubjectType: "user", TenantID: "acme", Roles: []string{"tenant_admin"}}, nil
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	return testHandlerWithChecker(t, checkerStub{})
}

func testHandlerWithChecker(t *testing.T, checker authorization.RelationshipChecker) http.Handler {
	t.Helper()
	service, err := authorization.NewService(checker, evaluatorStub{}, auditStub{}, authorization.ServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouter(service, observability.NewMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil)), RouterConfig{
		Verifier:  verifierStub{},
		Resources: StaticResourceResolver{"document:roadmap": {Type: "document", ID: "roadmap", TenantID: "acme"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return router.Handler()
}

func authorize(request *http.Request) { request.Header.Set("Authorization", "Bearer test-token") }

func TestDecisionEndpoint(t *testing.T) {
	payload := map[string]any{
		"subject":  map[string]any{"type": "user", "id": "alice", "tenant_id": "acme", "roles": []string{"tenant_admin"}},
		"resource": map[string]any{"type": "document", "id": "roadmap", "tenant_id": "acme", "attributes": map[string]any{"classification": "internal"}},
		"action":   "view",
		"context":  map[string]any{"device_trusted": true, "mfa": true, "risk_score": 5},
	}
	body, _ := json.Marshal(payload)
	request := httptest.NewRequest(http.MethodPost, "/v1/decisions/check", bytes.NewReader(body))
	authorize(request)
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

func TestDecisionEndpointDoesNotExposeDependencyErrors(t *testing.T) {
	payload := `{"subject":{"id":"alice","tenant_id":"acme"},"resource":{"type":"document","id":"roadmap","tenant_id":"acme"},"action":"view"}`
	request := httptest.NewRequest(http.MethodPost, "/v1/decisions/check", bytes.NewBufferString(payload))
	authorize(request)
	response := httptest.NewRecorder()
	testHandlerWithChecker(t, checkerErrorStub{}).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if bytes.Contains(response.Body.Bytes(), []byte("private detail")) {
		t.Fatalf("dependency detail leaked in response: %s", response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"code":"dependency_unavailable"`)) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}

func TestDecisionEndpointRejectsUnknownFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/decisions/check", bytes.NewBufferString(`{"unknown":true}`))
	authorize(request)
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDecisionEndpointRejectsSecondJSONValue(t *testing.T) {
	payload := `{"subject":{"id":"alice","tenant_id":"acme"},"resource":{"type":"document","id":"roadmap","tenant_id":"acme"},"action":"view"} {"extra":true}`
	request := httptest.NewRequest(http.MethodPost, "/v1/decisions/check", bytes.NewBufferString(payload))
	authorize(request)
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDecisionEndpointRejectsMalformedTrailingJSON(t *testing.T) {
	payload := `{"subject":{"id":"alice","tenant_id":"acme"},"resource":{"type":"document","id":"roadmap","tenant_id":"acme"},"action":"view"} {`
	request := httptest.NewRequest(http.MethodPost, "/v1/decisions/check", bytes.NewBufferString(payload))
	authorize(request)
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
	authorize(request)
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestExtAuthzContract(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/ext-authz/documents/roadmap", nil)
	authorize(request)
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Authorization-Decision-ID") == "" {
		t.Fatal("expected decision ID response header")
	}
}

func TestExtAuthzIgnoresSpoofedIdentityHeaders(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/ext-authz/documents/roadmap", nil)
	authorize(request)
	request.Header.Set("X-Subject-ID", "mallory")
	request.Header.Set("X-Tenant-ID", "globex")
	request.Header.Set("X-MFA", "true")
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
