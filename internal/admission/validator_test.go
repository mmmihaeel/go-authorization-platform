package admission

import (
	"encoding/json"
	"testing"
)

func requestFor(t *testing.T, kind string, object any) *Request {
	t.Helper()
	raw, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	return &Request{UID: "uid-1", Kind: GroupVersionKind{Kind: kind}, Object: raw}
}

func TestAllowsUnprotectedWorkload(t *testing.T) {
	request := requestFor(t, "Pod", map[string]any{
		"metadata": map[string]any{"annotations": map[string]string{}},
		"spec":     map[string]any{"containers": []map[string]string{{"name": "app", "image": "demo"}}},
	})
	if result := Validate(request); !result.Allowed {
		t.Fatalf("unexpected denial: %s", result.Message)
	}
}

func TestDeniesProtectedWorkloadWithoutPolicyReference(t *testing.T) {
	request := requestFor(t, "Deployment", map[string]any{
		"spec": map[string]any{"template": map[string]any{
			"metadata": map[string]any{"annotations": map[string]string{ProtectedAnnotation: "true"}},
			"spec":     map[string]any{"containers": []map[string]string{{"name": SidecarName, "image": "authz"}}},
		}},
	})
	result := Validate(request)
	if result.Allowed || result.Message == "" {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestAllowsProtectedWorkloadWithBindingAndSidecar(t *testing.T) {
	request := requestFor(t, "Deployment", map[string]any{
		"spec": map[string]any{"template": map[string]any{
			"metadata": map[string]any{"annotations": map[string]string{ProtectedAnnotation: "true", PolicyRefAnnotation: "document-api"}},
			"spec":     map[string]any{"containers": []map[string]string{{"name": "app", "image": "demo"}, {"name": SidecarName, "image": "authz"}}},
		}},
	})
	if result := Validate(request); !result.Allowed {
		t.Fatalf("unexpected denial: %s", result.Message)
	}
}
