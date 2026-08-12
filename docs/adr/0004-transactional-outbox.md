# ADR 0004: PostgreSQL transactional outbox for audit events

- Status: Accepted
- Date: 2026-08-12

## Context

Writing an authorization decision to PostgreSQL and publishing an audit event to RabbitMQ cannot be made atomic with a normal database transaction. Publishing first can create events for rolled-back decisions; committing first can lose an event if the process crashes.

## Decision

Write the decision and an outbox row in the same PostgreSQL transaction. A separate worker claims rows with `FOR UPDATE SKIP LOCKED`, publishes at least once, and records completion/retry state. Consumers use an inbox/event-ID guard before business side effects.

## Consequences

- no distributed transaction coordinator is required;
- broker downtime produces backlog rather than data loss;
- duplicate delivery remains possible and is part of the contract;
- backlog monitoring, stale-lock recovery, dead-lettering, and replay tooling become operational responsibilities.
