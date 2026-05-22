BEGIN;

CREATE INDEX IF NOT EXISTS idx_article_embeddings_user_model_version_updated
    ON article_embeddings(user_id, model, version, updated_at DESC);

COMMIT;
