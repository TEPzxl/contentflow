BEGIN;

DROP TABLE IF EXISTS collection_dlq_items;
DROP TABLE IF EXISTS outbox_events;

COMMIT;
