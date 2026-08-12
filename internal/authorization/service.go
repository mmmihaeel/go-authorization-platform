package authorization

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type ServiceConfig struct {
	AuditRequired bool
}

type Service struct {
	relationships RelationshipChecker
	policies      ContextPolicyEvaluator
	audit         AuditRecorder
	config        ServiceConfig
	now           func() time.Time
	newID         func() (string, error)
}

func NewService(
	relationships RelationshipChecker,
	policies ContextPolicyEvaluator,
	audit AuditRecorder,
	config ServiceConfig,
) (*Service, error) {
	if relationships == nil {
		return nil, errors.New("relationship checker is required")
	}
	if policies == nil {
		return nil, errors.New("policy evaluator is required")
	}
	if config.AuditRequired && audit == nil {
		return nil, errors.New("audit recorder is required when audit is mandatory")
	}
	return &Service{
		relationships: relationships,
		policies:      policies,
		audit:         audit,
		config:        config,
		now:           time.Now,
		newID:         randomID,
	}, nil
}

func (s *Service) Check(ctx context.Context, request CheckRequest) (Decision, error) {
	startedAt := s.now()
	request.Normalize(startedAt)
	decisionID, err := s.newID()
	if err != nil {
		return Decision{}, fmt.Errorf("generate decision ID: %w", err)
	}
	decision := Decision{
		ID:            decisionID,
		CorrelationID: request.CorrelationID,
		Allowed:       false,
		ReasonCodes:   []string{"deny_by_default"},
		EvaluatedAt:   startedAt.UTC(),
	}

	if err := request.Validate(); err != nil {
		decision.ReasonCodes = []string{"invalid_request"}
		decision.LatencyMS.Total = durationMS(s.now().Sub(startedAt))
		s.recordBestEffort(ctx, request, &decision)
		return decision, err
	}

	relationshipStarted := s.now()
	relationshipAllowed, err := s.relationships.Check(ctx, RelationshipQuery{
		Subject:  request.Subject,
		Resource: request.Resource,
		Relation: request.Relation,
		Context:  request.Context,
	})
	decision.LatencyMS.OpenFGA = durationMS(s.now().Sub(relationshipStarted))
	if err != nil {
		decision.ReasonCodes = []string{"relationship_dependency_unavailable"}
		decision.LatencyMS.Total = durationMS(s.now().Sub(startedAt))
		s.recordBestEffort(ctx, request, &decision)
		return decision, &DependencyError{Component: "openfga", Err: err}
	}

	policyStarted := s.now()
	policyResult, err := s.policies.Evaluate(ctx, PolicyInput{
		Subject:             request.Subject,
		Resource:            request.Resource,
		Action:              request.Action,
		Relation:            request.Relation,
		Context:             request.Context,
		RelationshipAllowed: relationshipAllowed,
	})
	decision.LatencyMS.OPA = durationMS(s.now().Sub(policyStarted))
	if err != nil {
		decision.ReasonCodes = []string{"policy_dependency_unavailable"}
		decision.LatencyMS.Total = durationMS(s.now().Sub(startedAt))
		s.recordBestEffort(ctx, request, &decision)
		return decision, &DependencyError{Component: "opa", Err: err}
	}

	decision.Allowed = relationshipAllowed && policyResult.Allow
	switch {
	case !relationshipAllowed:
		decision.ReasonCodes = []string{"relationship_denied"}
	case !policyResult.Allow:
		if len(policyResult.Reasons) > 0 {
			decision.ReasonCodes = append([]string(nil), policyResult.Reasons...)
		} else {
			decision.ReasonCodes = []string{"context_policy_denied"}
		}
	default:
		decision.ReasonCodes = []string{"allowed"}
	}

	decision.LatencyMS.Total = durationMS(s.now().Sub(startedAt))
	if err := s.record(ctx, request, &decision); err != nil {
		if s.config.AuditRequired {
			decision.Allowed = false
			decision.ReasonCodes = []string{"audit_persistence_failed"}
			return decision, &AuditError{Err: err}
		}
	}

	return decision, nil
}

func (s *Service) record(ctx context.Context, request CheckRequest, decision *Decision) error {
	if s.audit == nil {
		return nil
	}
	if err := s.audit.RecordDecision(ctx, AuditRecord{Request: request, Decision: *decision}); err != nil {
		return err
	}
	decision.AuditRecorded = true
	return nil
}

func (s *Service) recordBestEffort(ctx context.Context, request CheckRequest, decision *Decision) {
	_ = s.record(ctx, request, decision)
}

func durationMS(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func randomID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}
