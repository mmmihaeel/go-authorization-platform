package authorization

import (
	"context"
	"fmt"
	"strings"
)

// DemoRelationshipChecker keeps the starter executable without external
// infrastructure. It is deliberately small and must not be used as a
// production authorization source of truth.
type DemoRelationshipChecker struct{}

func (DemoRelationshipChecker) Check(_ context.Context, query RelationshipQuery) (bool, error) {
	if query.Subject.TenantID != query.Resource.TenantID {
		return false, nil
	}
	if hasRole(query.Subject.Roles, "tenant_admin") {
		return true, nil
	}

	ownerID, _ := query.Resource.Attributes["owner_id"].(string)
	isOwner := ownerID != "" && ownerID == query.Subject.ID

	switch query.Relation {
	case "can_view":
		return isOwner || hasRole(query.Subject.Roles, "tenant_auditor") || containsSubject(query.Resource.Attributes["viewers"], query.Subject.ID), nil
	case "can_edit":
		return isOwner || containsSubject(query.Resource.Attributes["editors"], query.Subject.ID), nil
	case "can_delete":
		return isOwner, nil
	case "can_manage":
		return false, nil
	default:
		return false, fmt.Errorf("unsupported demo relation %q", query.Relation)
	}
}

type DemoPolicyEvaluator struct{}

func (DemoPolicyEvaluator) Evaluate(_ context.Context, input PolicyInput) (PolicyResult, error) {
	reasons := make([]string, 0, 4)

	if !input.RelationshipAllowed {
		reasons = append(reasons, "relationship_denied")
	}
	if input.Subject.TenantID != input.Resource.TenantID {
		reasons = append(reasons, "tenant_boundary_violation")
	}
	if boolClaim(input.Subject.Claims, "account_suspended") {
		reasons = append(reasons, "subject_suspended")
	}
	if input.Context.RiskScore > 70 {
		reasons = append(reasons, "risk_score_too_high")
	}
	if (input.Action == "delete" || input.Action == "manage") && !input.Context.MFA {
		reasons = append(reasons, "mfa_required")
	}
	classification, _ := input.Resource.Attributes["classification"].(string)
	if classification == "restricted" && !input.Context.DeviceTrusted {
		reasons = append(reasons, "trusted_device_required")
	}
	if classification == "restricted" && strings.TrimSpace(input.Context.Purpose) == "" {
		reasons = append(reasons, "access_purpose_required")
	}

	return PolicyResult{Allow: len(reasons) == 0, Reasons: reasons}, nil
}

func hasRole(roles []string, expected string) bool {
	for _, role := range roles {
		if role == expected {
			return true
		}
	}
	return false
}

func containsSubject(value any, subjectID string) bool {
	switch subjects := value.(type) {
	case []string:
		for _, candidate := range subjects {
			if candidate == subjectID {
				return true
			}
		}
	case []any:
		for _, candidate := range subjects {
			if text, ok := candidate.(string); ok && text == subjectID {
				return true
			}
		}
	}
	return false
}

func boolClaim(claims map[string]any, key string) bool {
	value, _ := claims[key].(bool)
	return value
}
