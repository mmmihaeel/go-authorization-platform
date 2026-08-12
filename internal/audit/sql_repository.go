package audit

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mmmihaeel/go-authorization-platform/internal/authorization"
)

const stalePublishingLockTimeout = 2 * time.Minute

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) (*SQLRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	return &SQLRepository{db: db}, nil
}

func (r *SQLRepository) RecordDecision(ctx context.Context, record authorization.AuditRecord) error {
	record.Decision.AuditRecorded = true
	requestJSON, err := json.Marshal(record.Request)
	if err != nil {
		return fmt.Errorf("marshal authorization request: %w", err)
	}
	decisionJSON, err := json.Marshal(record.Decision)
	if err != nil {
		return fmt.Errorf("marshal authorization decision: %w", err)
	}
	reasonCodes, err := json.Marshal(record.Decision.ReasonCodes)
	if err != nil {
		return fmt.Errorf("marshal reason codes: %w", err)
	}

	eventID, err := newID()
	if err != nil {
		return fmt.Errorf("generate outbox event ID: %w", err)
	}
	eventPayload, err := json.Marshal(map[string]any{
		"event_id":    eventID,
		"event_type":  "authorization.decision.v1",
		"occurred_at": record.Decision.EvaluatedAt,
		"data":        record,
	})
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin audit transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
        INSERT INTO authorization_decisions (
            id, correlation_id, tenant_id, subject_type, subject_id,
            resource_type, resource_id, action, relation, allowed,
            reason_codes, request_payload, decision_payload, evaluated_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
    `,
		record.Decision.ID,
		nullable(record.Decision.CorrelationID),
		record.Request.Subject.TenantID,
		record.Request.Subject.Type,
		record.Request.Subject.ID,
		record.Request.Resource.Type,
		record.Request.Resource.ID,
		record.Request.Action,
		record.Request.Relation,
		record.Decision.Allowed,
		reasonCodes,
		requestJSON,
		decisionJSON,
		record.Decision.EvaluatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert authorization decision: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
        INSERT INTO outbox_events (
            id, aggregate_type, aggregate_id, event_type, routing_key,
            payload, status, attempts, available_at, created_at
        ) VALUES ($1,'authorization_decision',$2,'authorization.decision.v1',
                  'authorization.decision',$3,'pending',0,now(),now())
    `, eventID, record.Decision.ID, eventPayload)
	if err != nil {
		return fmt.Errorf("insert audit outbox event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit audit transaction: %w", err)
	}
	return nil
}

func (r *SQLRepository) ClaimPending(ctx context.Context, limit int) ([]OutboxMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin outbox claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
        SELECT id, event_type, routing_key, payload, attempts, created_at
        FROM outbox_events
        WHERE (
            status IN ('pending','failed') AND available_at <= now()
        ) OR (
            status = 'publishing'
            AND locked_at <= now() - ($2::bigint * interval '1 millisecond')
        )
        ORDER BY created_at
        FOR UPDATE SKIP LOCKED
        LIMIT $1
    `, limit, stalePublishingLockTimeout.Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("select outbox events: %w", err)
	}
	defer rows.Close()

	messages := make([]OutboxMessage, 0, limit)
	for rows.Next() {
		var message OutboxMessage
		if err := rows.Scan(&message.ID, &message.EventType, &message.RoutingKey, &message.Payload, &message.Attempts, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}

	for _, message := range messages {
		if _, err := tx.ExecContext(ctx, `
            UPDATE outbox_events
            SET status='publishing', locked_at=now(), attempts=attempts+1, updated_at=now()
            WHERE id=$1
        `, message.ID); err != nil {
			return nil, fmt.Errorf("lock outbox event %s: %w", message.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit outbox claim: %w", err)
	}
	return messages, nil
}

func (r *SQLRepository) MarkPublished(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
        UPDATE outbox_events
        SET status='published', published_at=now(), locked_at=NULL, last_error=NULL, updated_at=now()
        WHERE id=$1
    `, id)
	if err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	return nil
}

func (r *SQLRepository) MarkFailed(ctx context.Context, id string, cause error, retryAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
        UPDATE outbox_events
        SET status='failed', available_at=$2, locked_at=NULL, last_error=$3, updated_at=now()
        WHERE id=$1
    `, id, retryAt, cause.Error())
	if err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}
	return nil
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func newID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw)
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}
