BEGIN;

CREATE TABLE authorization_decisions (
    id                  uuid PRIMARY KEY,
    correlation_id      text,
    tenant_id           text NOT NULL,
    subject_type        text NOT NULL,
    subject_id          text NOT NULL,
    resource_type       text NOT NULL,
    resource_id         text NOT NULL,
    action              text NOT NULL,
    relation            text NOT NULL,
    allowed             boolean NOT NULL,
    reason_codes        jsonb NOT NULL DEFAULT '[]'::jsonb,
    request_payload     jsonb NOT NULL,
    decision_payload    jsonb NOT NULL,
    evaluated_at        timestamptz NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX authorization_decisions_tenant_time_idx
    ON authorization_decisions (tenant_id, evaluated_at DESC);
CREATE INDEX authorization_decisions_subject_idx
    ON authorization_decisions (tenant_id, subject_type, subject_id, evaluated_at DESC);
CREATE INDEX authorization_decisions_resource_idx
    ON authorization_decisions (tenant_id, resource_type, resource_id, evaluated_at DESC);
CREATE INDEX authorization_decisions_denied_idx
    ON authorization_decisions (tenant_id, evaluated_at DESC)
    WHERE allowed = false;

CREATE TABLE outbox_events (
    id              uuid PRIMARY KEY,
    aggregate_type  text NOT NULL,
    aggregate_id    text NOT NULL,
    event_type      text NOT NULL,
    routing_key     text NOT NULL,
    payload         jsonb NOT NULL,
    status          text NOT NULL CHECK (status IN ('pending','publishing','published','failed')),
    attempts        integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at    timestamptz NOT NULL DEFAULT now(),
    locked_at       timestamptz,
    published_at    timestamptz,
    last_error      text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX outbox_events_dispatch_idx
    ON outbox_events (available_at, created_at)
    WHERE status IN ('pending','failed');

CREATE INDEX outbox_events_stale_lock_idx
    ON outbox_events (locked_at, created_at)
    WHERE status = 'publishing';

CREATE TABLE inbox_events (
    consumer_name   text NOT NULL,
    event_id        text NOT NULL,
    status          text NOT NULL CHECK (status IN ('processing','completed','failed')),
    payload_hash    text,
    attempts        integer NOT NULL DEFAULT 1,
    last_error      text,
    started_at      timestamptz NOT NULL DEFAULT now(),
    completed_at    timestamptz,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer_name, event_id)
);

CREATE TABLE consumed_audit_events (
    event_id        uuid PRIMARY KEY,
    event_type      text NOT NULL,
    payload         jsonb NOT NULL,
    consumed_at     timestamptz NOT NULL DEFAULT now()
);

COMMIT;
