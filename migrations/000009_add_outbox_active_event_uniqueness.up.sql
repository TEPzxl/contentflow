BEGIN;

WITH ranked_active_outbox_events AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY topic, event_key
            ORDER BY created_at ASC, id ASC
        ) AS row_number
    FROM outbox_events
    WHERE status IN ('pending', 'failed', 'processing')
)
UPDATE outbox_events
SET
    status = 'deduplicated',
    last_error = 'deduplicated by migration 000009; an older active outbox event with the same topic and key was kept',
    claim_id = '',
    locked_until = NULL,
    updated_at = NOW()
WHERE id IN (
    SELECT id
    FROM ranked_active_outbox_events
    WHERE row_number > 1
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_outbox_events_active_topic_key_unique
    ON outbox_events(topic, event_key)
    WHERE status IN ('pending', 'failed', 'processing');

COMMIT;
