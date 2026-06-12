BEGIN;

DROP INDEX IF EXISTS idx_outbox_events_active_topic_key_unique;

COMMIT;
