# Contentflow Operations Runbook

## Database latency or outage

Symptoms:

- `/readyz` returns `503`.
- `ContentflowDatabaseLatencyHigh` fires.
- API latency and error rate rise together.

Checks:

- Confirm PostgreSQL pod or container health.
- Check active connections and slow queries.
- Confirm recent migrations completed successfully.

Actions:

- Scale API workers down only if the database is overloaded by connection pressure.
- Restore database service before replaying collection jobs.
- After recovery, inspect DLQ items and replay only failures caused by transient database errors.

## Redis unavailable

Symptoms:

- `/readyz` returns `503`.
- Login or collection rate limiting may fail closed.
- Source/article list cache misses increase.

Checks:

- Confirm Redis health and persistence volume status.
- Check network connectivity from backend pods.

Actions:

- Restart Redis only after checking persistent volume status.
- Keep API replicas running; Redis-backed cache is disposable, but locks/rate limits depend on Redis availability.

## Kafka backlog or worker lag

Symptoms:

- Collection requests return queued but collection runs do not complete.
- `contentflow_kafka_jobs_total` stops increasing for `collection.completed`.
- Consumer lag grows in Kafka tooling.

Checks:

- Confirm worker deployment is healthy.
- Confirm Kafka broker health and topic availability.
- Inspect worker logs for retryable errors.

Actions:

- Scale `contentflow-worker` replicas if Kafka and database are healthy.
- Keep `outbox_events` under observation; failed rows with future `next_attempt_at` are normal retry behavior.
- Replay DLQ only after the root cause is fixed.

## Kafka DLQ has new items

Symptoms:

- `ContentflowKafkaDLQWrites` fires.
- `/api/v1/collection-dlq?status=pending` returns items.

Checks:

- Group DLQ items by `error_message`, `source_id`, and `attempt`.
- Distinguish permanent errors from transient infrastructure errors.

Actions:

- For transient errors, fix infrastructure, then call `POST /api/v1/collection-dlq/{id}/replay`.
- For permanent source errors, update the source or call `POST /api/v1/collection-dlq/{id}/handled`.

## Collection failure rate is high

Symptoms:

- `ContentflowCollectorFailureRateHigh` fires.
- `contentflow_collection_runs_total{status="failed"}` increases quickly.

Checks:

- Compare failures by source type.
- Check RSS upstream status or IMAP credentials.
- Check recent deployment changes.

Actions:

- Roll back the latest deployment if failures correlate with release time.
- Disable noisy sources temporarily if they overload workers.
- Replay DLQ after the upstream or credential issue is resolved.

## HTTP 5xx rate is high

Symptoms:

- `ContentflowHighHTTPErrorRate` fires.
- Grafana SLO dashboard availability drops.

Checks:

- Inspect backend logs by `request_id`.
- Check database, Redis, Kafka, and recent deploy events.

Actions:

- Roll back if the spike starts immediately after a release.
- If caused by dependency outage, reduce traffic and keep readiness accurate.
