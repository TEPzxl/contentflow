BEGIN;

DROP INDEX IF EXISTS idx_outbox_events_processing_lease;

ALTER TABLE outbox_events
    DROP COLUMN IF EXISTS locked_until,
    DROP COLUMN IF EXISTS claim_id;

COMMIT;
