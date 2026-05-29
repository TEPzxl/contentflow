BEGIN;

ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS claim_id VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_outbox_events_processing_lease
    ON outbox_events(status, locked_until)
    WHERE status = 'processing';

COMMIT;
