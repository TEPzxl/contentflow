# Release Checklist

## Before release

- Confirm `scripts/ci.sh all` passes.
- Confirm `scripts/ci.sh integration` passes when Docker is available.
- Review database migrations and verify every `.up.sql` has a matching `.down.sql`.
- Confirm backend and frontend images build locally with `scripts/ci.sh docker-build`.
- Confirm secrets exist in the target secret manager for `database-password` and `jwt-secret`.
- Confirm alert rules and dashboards render in the target environment.

## Release

- Tag the release as `vX.Y.Z`.
- Wait for the Release Images workflow to publish backend and frontend images.
- Update the target Kustomize overlay image tags.
- Apply migrations before rolling out application pods.
- Roll out backend, worker, scheduler, and frontend deployments.

## After release

- Check `/readyz`, `/metrics`, and frontend `/`.
- Watch `ContentflowHighHTTPErrorRate`, collection failures, DLQ writes, and DB latency for at least 30 minutes.
- Confirm outbox pending/failed volume drains after deployment.

## Rollback

- Roll back deployments to the previous known-good image tags.
- Do not run down migrations automatically unless the failed release requires it and data compatibility is confirmed.
- Keep workers paused if replaying jobs would amplify the incident.
