BEGIN;
DROP TABLE IF EXISTS consumed_audit_events;
DROP TABLE IF EXISTS inbox_events;
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS authorization_decisions;
COMMIT;
