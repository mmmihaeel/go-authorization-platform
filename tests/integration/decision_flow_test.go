package integration

import (
	"context"
	"testing"

	"github.com/mmmihaeel/go-authorization-platform/internal/audit"
	"github.com/mmmihaeel/go-authorization-platform/internal/authorization"
)

func TestMultiTenantDecisionFlow(t *testing.T) {
	recorder := audit.NewMemoryRecorder()
	service, err := authorization.NewService(authorization.DemoRelationshipChecker{}, authorization.DemoPolicyEvaluator{}, recorder, authorization.ServiceConfig{})
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
	if len(recorder.Records()) != 1 {
		t.Fatal("denied decisions must still be audited")
	}
}
