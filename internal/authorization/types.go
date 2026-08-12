package authorization

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidRequest = errors.New("invalid authorization request")

type Subject struct {
	Type     string         `json:"type"`
	ID       string         `json:"id"`
	TenantID string         `json:"tenant_id"`
	Roles    []string       `json:"roles,omitempty"`
	Claims   map[string]any `json:"claims,omitempty"`
}

type Resource struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	TenantID   string         `json:"tenant_id"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

type RequestContext struct {
	IPAddress     string    `json:"ip_address,omitempty"`
	DeviceTrusted bool      `json:"device_trusted"`
	MFA           bool      `json:"mfa"`
	RiskScore     int       `json:"risk_score,omitempty"`
	Purpose       string    `json:"purpose,omitempty"`
	RequestedAt   time.Time `json:"requested_at,omitempty"`
}

type CheckRequest struct {
	Subject       Subject        `json:"subject"`
	Resource      Resource       `json:"resource"`
	Action        string         `json:"action"`
	Relation      string         `json:"relation,omitempty"`
	Context       RequestContext `json:"context"`
	CorrelationID string         `json:"correlation_id,omitempty"`
}

func (r *CheckRequest) Normalize(now time.Time) {
	r.Subject.Type = strings.TrimSpace(r.Subject.Type)
	r.Subject.ID = strings.TrimSpace(r.Subject.ID)
	r.Subject.TenantID = strings.TrimSpace(r.Subject.TenantID)
	r.Resource.Type = strings.TrimSpace(r.Resource.Type)
	r.Resource.ID = strings.TrimSpace(r.Resource.ID)
	r.Resource.TenantID = strings.TrimSpace(r.Resource.TenantID)
	r.Action = strings.ToLower(strings.TrimSpace(r.Action))
	r.Relation = strings.ToLower(strings.TrimSpace(r.Relation))
	r.CorrelationID = strings.TrimSpace(r.CorrelationID)

	if r.Subject.Type == "" {
		r.Subject.Type = "user"
	}
	if r.Relation == "" && r.Action != "" {
		r.Relation = "can_" + r.Action
	}
	if r.Context.RequestedAt.IsZero() {
		r.Context.RequestedAt = now.UTC()
	}
}

func (r CheckRequest) Validate() error {
	var missing []string
	if r.Subject.ID == "" {
		missing = append(missing, "subject.id")
	}
	if r.Subject.TenantID == "" {
		missing = append(missing, "subject.tenant_id")
	}
	if r.Resource.Type == "" {
		missing = append(missing, "resource.type")
	}
	if r.Resource.ID == "" {
		missing = append(missing, "resource.id")
	}
	if r.Resource.TenantID == "" {
		missing = append(missing, "resource.tenant_id")
	}
	if r.Action == "" {
		missing = append(missing, "action")
	}
	if r.Relation == "" {
		missing = append(missing, "relation")
	}
	if r.Context.RiskScore < 0 || r.Context.RiskScore > 100 {
		return fmt.Errorf("%w: context.risk_score must be between 0 and 100", ErrInvalidRequest)
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing %s", ErrInvalidRequest, strings.Join(missing, ", "))
	}
	return nil
}

type RelationshipQuery struct {
	Subject  Subject
	Resource Resource
	Relation string
	Context  RequestContext
}

type PolicyInput struct {
	Subject             Subject        `json:"subject"`
	Resource            Resource       `json:"resource"`
	Action              string         `json:"action"`
	Relation            string         `json:"relation"`
	Context             RequestContext `json:"context"`
	RelationshipAllowed bool           `json:"relationship_allowed"`
}

type PolicyResult struct {
	Allow   bool     `json:"allow"`
	Reasons []string `json:"reasons,omitempty"`
}

type ComponentLatency struct {
	OpenFGA time.Duration `json:"-"`
	OPA     time.Duration `json:"-"`
	Total   time.Duration `json:"-"`
}

type LatencyMillis struct {
	OpenFGA float64 `json:"openfga"`
	OPA     float64 `json:"opa"`
	Total   float64 `json:"total"`
}

type Decision struct {
	ID            string        `json:"decision_id"`
	CorrelationID string        `json:"correlation_id,omitempty"`
	Allowed       bool          `json:"allowed"`
	ReasonCodes   []string      `json:"reason_codes"`
	EvaluatedAt   time.Time     `json:"evaluated_at"`
	LatencyMS     LatencyMillis `json:"latency_ms"`
	AuditRecorded bool          `json:"audit_recorded"`
}

type AuditRecord struct {
	Request  CheckRequest `json:"request"`
	Decision Decision     `json:"decision"`
}

type RelationshipChecker interface {
	Check(context.Context, RelationshipQuery) (bool, error)
}

type ContextPolicyEvaluator interface {
	Evaluate(context.Context, PolicyInput) (PolicyResult, error)
}

type AuditRecorder interface {
	RecordDecision(context.Context, AuditRecord) error
}

type DependencyError struct {
	Component string
	Err       error
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("authorization dependency %s failed: %v", e.Component, e.Err)
}

func (e *DependencyError) Unwrap() error { return e.Err }
