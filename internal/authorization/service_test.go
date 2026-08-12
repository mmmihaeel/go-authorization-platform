package authorization

import (
	"context"
	"errors"
	"testing"
	"time"
)

type checkerFunc func(context.Context, RelationshipQuery) (bool, error)

func (f checkerFunc) Check(ctx context.Context, query RelationshipQuery) (bool, error) {
	return f(ctx, query)
}

type evaluatorFunc func(context.Context, PolicyInput) (PolicyResult, error)

func (f evaluatorFunc) Evaluate(ctx context.Context, input PolicyInput) (PolicyResult, error) {
	return f(ctx, input)
}

type recorderStub struct {
	records []AuditRecord
	err     error
}

func (r *recorderStub) RecordDecision(_ context.Context, record AuditRecord) error {
	r.records = append(r.records, record)
	return r.err
}

func validRequest() CheckRequest {
	return CheckRequest{
		Subject:  Subject{Type: "user", ID: "alice", TenantID: "acme"},
		Resource: Resource{Type: "document", ID: "roadmap", TenantID: "acme"},
		Action:   "view",
		Context:  RequestContext{DeviceTrusted: true, MFA: true, RiskScore: 10},
	}
}

func TestNewServiceRequiresMandatoryAuditRecorder(t *testing.T) {
	_, err := NewService(
		checkerFunc(func(context.Context, RelationshipQuery) (bool, error) { return true, nil }),
		evaluatorFunc(func(context.Context, PolicyInput) (PolicyResult, error) { return PolicyResult{Allow: true}, nil }),
		nil,
		ServiceConfig{AuditRequired: true},
	)
	if err == nil {
		t.Fatal("expected mandatory audit configuration to require a recorder")
	}
}

func TestServiceAllowsOnlyWhenRelationshipAndPolicyAllow(t *testing.T) {
	recorder := &recorderStub{}
	service, err := NewService(
		checkerFunc(func(context.Context, RelationshipQuery) (bool, error) { return true, nil }),
		evaluatorFunc(func(_ context.Context, input PolicyInput) (PolicyResult, error) {
			if !input.RelationshipAllowed {
				t.Fatal("relationship result was not forwarded to policy")
			}
			return PolicyResult{Allow: true}, nil
		}),
		recorder,
		ServiceConfig{},
	)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC) }
	service.newID = func() (string, error) { return "decision-1", nil }

	decision, err := service.Check(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected allow, got %#v", decision)
	}
	if decision.ID != "decision-1" {
		t.Fatalf("unexpected decision ID %q", decision.ID)
	}
	if !decision.AuditRecorded || len(recorder.records) != 1 {
		t.Fatalf("expected one audit record, got %#v", recorder)
	}
}

func TestServiceDeniesRelationshipFailure(t *testing.T) {
	service, _ := NewService(
		checkerFunc(func(context.Context, RelationshipQuery) (bool, error) { return false, nil }),
		evaluatorFunc(func(context.Context, PolicyInput) (PolicyResult, error) { return PolicyResult{Allow: false}, nil }),
		nil,
		ServiceConfig{},
	)
	decision, err := service.Check(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed {
		t.Fatal("relationship denial must fail closed")
	}
	if decision.ReasonCodes[0] != "relationship_denied" {
		t.Fatalf("unexpected reasons: %v", decision.ReasonCodes)
	}
}

func TestServiceDeniesContextPolicy(t *testing.T) {
	service, _ := NewService(
		checkerFunc(func(context.Context, RelationshipQuery) (bool, error) { return true, nil }),
		evaluatorFunc(func(context.Context, PolicyInput) (PolicyResult, error) {
			return PolicyResult{Allow: false, Reasons: []string{"mfa_required"}}, nil
		}),
		nil,
		ServiceConfig{},
	)
	decision, err := service.Check(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed || decision.ReasonCodes[0] != "mfa_required" {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestServiceFailsClosedWhenDependencyFails(t *testing.T) {
	service, _ := NewService(
		checkerFunc(func(context.Context, RelationshipQuery) (bool, error) { return false, errors.New("timeout") }),
		evaluatorFunc(func(context.Context, PolicyInput) (PolicyResult, error) { return PolicyResult{Allow: true}, nil }),
		nil,
		ServiceConfig{},
	)
	decision, err := service.Check(context.Background(), validRequest())
	var dependencyErr *DependencyError
	if !errors.As(err, &dependencyErr) {
		t.Fatalf("expected DependencyError, got %v", err)
	}
	if decision.Allowed {
		t.Fatal("dependency failure must deny")
	}
}

func TestServiceCanRequireAuditPersistence(t *testing.T) {
	recorder := &recorderStub{err: errors.New("database unavailable")}
	service, _ := NewService(
		checkerFunc(func(context.Context, RelationshipQuery) (bool, error) { return true, nil }),
		evaluatorFunc(func(context.Context, PolicyInput) (PolicyResult, error) { return PolicyResult{Allow: true}, nil }),
		recorder,
		ServiceConfig{AuditRequired: true},
	)
	decision, err := service.Check(context.Background(), validRequest())
	if err == nil {
		t.Fatal("expected audit error")
	}
	if decision.Allowed {
		t.Fatal("required audit persistence failure must deny")
	}
	if decision.ReasonCodes[0] != "audit_persistence_failed" {
		t.Fatalf("unexpected reasons %v", decision.ReasonCodes)
	}
}

func TestRequestValidation(t *testing.T) {
	service, _ := NewService(
		checkerFunc(func(context.Context, RelationshipQuery) (bool, error) { return false, nil }),
		evaluatorFunc(func(context.Context, PolicyInput) (PolicyResult, error) { return PolicyResult{}, nil }),
		nil, ServiceConfig{},
	)
	request := validRequest()
	request.Subject.ID = ""
	decision, err := service.Check(context.Background(), request)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
	if decision.ReasonCodes[0] != "invalid_request" {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}
