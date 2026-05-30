BEGIN;

CREATE TABLE IF NOT EXISTS collection_job_executions (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(100) NOT NULL UNIQUE,
    idempotency_key VARCHAR(200) NOT NULL,
    source_id BIGINT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 0,
    run_id BIGINT REFERENCES collection_runs(id) ON DELETE SET NULL,
    last_error TEXT NOT NULL DEFAULT '',
    claim_id VARCHAR(100) NOT NULL DEFAULT '',
    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_collection_job_executions_idempotency_key
    ON collection_job_executions(idempotency_key);

CREATE INDEX IF NOT EXISTS idx_collection_job_executions_source_id
    ON collection_job_executions(source_id);

CREATE INDEX IF NOT EXISTS idx_collection_job_executions_status
    ON collection_job_executions(status);

CREATE INDEX IF NOT EXISTS idx_collection_job_executions_processing_lease
    ON collection_job_executions(status, locked_until)
    WHERE status = 'processing';

COMMIT;
