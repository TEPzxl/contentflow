BEGIN;

CREATE TABLE IF NOT EXISTS outbox_events (
    id BIGSERIAL PRIMARY KEY,
    topic VARCHAR(200) NOT NULL,
    event_key VARCHAR(300) NOT NULL,
    payload_json JSONB NOT NULL,
    status VARCHAR(50) NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_status_next_attempt
    ON outbox_events(status, next_attempt_at);

CREATE INDEX IF NOT EXISTS idx_outbox_events_topic
    ON outbox_events(topic);

CREATE TABLE IF NOT EXISTS collection_dlq_items (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(100) NOT NULL,
    user_id BIGINT NOT NULL,
    source_id BIGINT NOT NULL,
    idempotency_key VARCHAR(200) NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    payload_json JSONB NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    replayed_at TIMESTAMPTZ,
    handled_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_collection_dlq_items_status_created
    ON collection_dlq_items(status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_collection_dlq_items_source_id
    ON collection_dlq_items(source_id);

CREATE INDEX IF NOT EXISTS idx_collection_dlq_items_task_id
    ON collection_dlq_items(task_id);

COMMIT;
