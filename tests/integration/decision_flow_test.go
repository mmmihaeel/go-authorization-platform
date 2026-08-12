package integration

import (
	"context"
	"sync"
	"testing"

	"github.com/mmmihaeel/go-authorization-platform/internal/authorization"
)

type recordingAudit struct {
	mu      sync.Mutex
	records []authorization.AuditRecord
}

func (r *recordingAudit) RecordDecision(_ context.Context, record authorization.AuditRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)
	return nil
}

func (r *recordingAudit) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.records)
}

type denyCrossTenant struct{}

func (denyCrossTenant) Check(_ context.Context, query authorization.RelationshipQuery) (bool, error) {
	return query.Subject.TenantID == query.Resource.TenantID, nil
}

type policy struct{}

func (policy) Evaluate(_ context.Context, input authorization.PolicyInput) (authorization.PolicyResult, error) {
	allow := input.RelationshipAllowed && input.Subject.TenantID == input.Resource.TenantID
	return authorization.PolicyResult{Allow: allow}, nil
}

func TestMultiTenantDecisionFlow(t *testing.T) {
	recorder := &recordingAudit{}
	service, err := authorization.NewService(denyCrossTenant{}, policy{}, recorder, authorization.ServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}

	request := authorization.CheckRequest{
		Subject:  authorization.Subject{ID: "alice", TenantID: "tenant-a", Roles: []string{"tenant_admin"}},
		Resource: authorization.Resource{Type: "document", ID: "strategy", TenantID: "tenant-b"},
		Action:   "view",
		Context:  authorization.RequestContext{DeviceTrusted: true, MFA: true, RiskScore: 5},
	}
	decision, err := service.Check(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed {
		t.Fatal("cross-tenant authorization must be denied")
	}
	if recorder.count() != 1 {
		t.Fatal("denied decisions must still be audited")
	}
}
