package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type SQLInbox struct {
	db           *sql.DB
	consumerName string
	lease        time.Duration
}

func NewSQLInbox(db *sql.DB, consumerName string, lease time.Duration) (*SQLInbox, error) {
	if db == nil || consumerName == "" {
		return nil, fmt.Errorf("database and consumer name are required")
	}
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	return &SQLInbox{db: db, consumerName: consumerName, lease: lease}, nil
}

func (i *SQLInbox) TryBegin(ctx context.Context, id string) (InboxClaim, error) {
	var acquired bool
	err := i.db.QueryRowContext(ctx, `
        INSERT INTO inbox_events (consumer_name, event_id, status)
        VALUES ($1, $2, 'processing')
        ON CONFLICT (consumer_name, event_id) DO UPDATE
        SET status='processing', attempts=inbox_events.attempts+1,
            started_at=now(), updated_at=now(), last_error=NULL
        WHERE inbox_events.status <> 'completed'
          AND inbox_events.updated_at <= now() - ($3::bigint * interval '1 millisecond')
        RETURNING true
	`, i.consumerName, id, i.lease.Milliseconds()).Scan(&acquired)
	if err != nil {
		if err != sql.ErrNoRows {
			return 0, fmt.Errorf("acquire inbox event: %w", err)
		}
		var status string
		if err := i.db.QueryRowContext(ctx, `
            SELECT status FROM inbox_events WHERE consumer_name=$1 AND event_id=$2
        `, i.consumerName, id).Scan(&status); err != nil {
			return 0, fmt.Errorf("inspect inbox event: %w", err)
		}
		if status == "completed" {
			return ClaimCompleted, nil
		}
		return ClaimBusy, nil
	}
	if !acquired {
		return 0, fmt.Errorf("acquire inbox event: invalid result")
	}
	return ClaimAcquired, nil
}

func (i *SQLInbox) Complete(ctx context.Context, id string) error {
	result, err := i.db.ExecContext(ctx, `
        UPDATE inbox_events SET status='completed', completed_at=now(), updated_at=now()
        WHERE consumer_name=$1 AND event_id=$2 AND status='processing'
    `, i.consumerName, id)
	if err != nil {
		return fmt.Errorf("complete inbox event: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return fmt.Errorf("complete inbox event: processing lease was lost")
	}
	return nil
}

func (i *SQLInbox) Release(ctx context.Context, id string) error {
	_, err := i.db.ExecContext(ctx, `DELETE FROM inbox_events WHERE consumer_name=$1 AND event_id=$2 AND status='processing'`, i.consumerName, id)
	if err != nil {
		return fmt.Errorf("release inbox event: %w", err)
	}
	return nil
}

type SQLAuditHandler struct{ db *sql.DB }

func NewSQLAuditHandler(db *sql.DB) (*SQLAuditHandler, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	return &SQLAuditHandler{db: db}, nil
}

func (h *SQLAuditHandler) Handle(ctx context.Context, payload []byte) error {
	var event struct {
		EventID   string          `json:"event_id"`
		EventType string          `json:"event_type"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("decode audit event: %w", err)
	}
	if event.EventID == "" || event.EventType != "authorization.decision.v1" || len(event.Data) == 0 {
		return fmt.Errorf("audit event is missing required fields")
	}
	_, err := h.db.ExecContext(ctx, `
        INSERT INTO consumed_audit_events (event_id, event_type, payload)
        VALUES ($1, $2, $3)
        ON CONFLICT (event_id) DO NOTHING
    `, event.EventID, event.EventType, payload)
	if err != nil {
		return fmt.Errorf("persist consumed audit event: %w", err)
	}
	return nil
}
