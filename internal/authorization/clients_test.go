package authorization

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenFGAClientBuildsTenantScopedObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stores/store-1/check" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var request openFGACheckRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.TupleKey.Object != "document:acme/roadmap" {
			t.Fatalf("unexpected object %s", request.TupleKey.Object)
		}
		if request.TupleKey.Relation != "can_view" {
			t.Fatalf("unexpected relation %s", request.TupleKey.Relation)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"allowed":true}`))
	}))
	defer server.Close()

	client, err := NewOpenFGAClient(OpenFGAConfig{BaseURL: server.URL, StoreID: "store-1"})
	if err != nil {
		t.Fatal(err)
	}
	allowed, err := client.Check(context.Background(), RelationshipQuery{
		Subject:  Subject{Type: "user", ID: "alice", TenantID: "acme"},
		Resource: Resource{Type: "document", ID: "roadmap", TenantID: "acme"},
		Relation: "can_view",
	})
	if err != nil || !allowed {
		t.Fatalf("allowed=%v err=%v", allowed, err)
	}
}

func TestOPAClientDecodesDecision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/data/authz/decision" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"allow":false,"reasons":["mfa_required"]}}`))
	}))
	defer server.Close()

	client, err := NewOPAClient(OPAConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Evaluate(context.Background(), PolicyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Allow || len(result.Reasons) != 1 || result.Reasons[0] != "mfa_required" {
		t.Fatalf("unexpected result %#v", result)
	}
}
