BEGIN;

CREATE INDEX IF NOT EXISTS idx_articles_search_tsv
    ON articles USING GIN (
        to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(summary, '') || ' ' || coalesce(content, ''))
    );

COMMIT;
