# Repository Guidelines

## Project Structure & Module Organization

Contentflow is a Go backend with a Next.js frontend. Backend entrypoint is `cmd/server/main.go`; dependency wiring and runtime modes live in `internal/app`. HTTP routing and middleware live in `internal/http`. Business modules are under `internal/module/*`: `auth`, `source`, `collector`, `article`, `collectionjob`, `scheduler`, and `ai`. Database migrations are in `migrations/`; OpenAPI docs are in `api/openapi.yaml`; deployment assets are in `deployments/`. Frontend code lives in `web/`, with app pages in `web/app`, feature panels in `web/features`, and API/session helpers in `web/lib`.

## Build, Test, and Development Commands

- `go run ./cmd/server`: run the backend using `CONTENTFLOW_CONFIG` or `configs/config.yaml`.
- `go test ./...`: run all Go tests.
- `make build`: build the backend binary as `contentflow`.
- `docker compose -f deployments/docker-compose.yaml up --build`: start the full local stack.
- `npm --prefix web run dev`: start the frontend dev server.
- `npm --prefix web run typecheck`, `npm --prefix web run lint`, `npm --prefix web run build`: validate frontend TypeScript, ESLint, and production build.
- `scripts/ci.sh all`: run the main local quality gate. Use `scripts/ci.sh smoke-api` only when a backend is already running.

## Coding Style & Naming Conventions

Format Go code with `gofmt`; prefer table-driven tests. Keep packages small and aligned with existing module boundaries. Use clear sentinel errors and wrap lower-level errors with context. Frontend code uses strict TypeScript, React components in PascalCase, and feature directories such as `web/features/articles`. Avoid adding production dependencies unless the reason is documented.

## Testing Guidelines

Go tests use `testing`, `gomock`, and testcontainers for integration paths. Test files follow `*_test.go`; integration-style tests are already colocated with their packages. Frontend E2E uses Playwright via `npm --prefix web run test`. Before submitting backend or frontend changes, run the narrow test first, then `go test ./...` and relevant frontend checks.

## Commit & Pull Request Guidelines

Recent history mostly uses concise prefixes such as `feat:`, `test:`, `docs:`, and `chore:`; keep commits focused and imperative. PRs should describe the behavior change, list verification commands, mention migrations or config changes, and include screenshots for visible frontend updates.

After adding content or fixing bugs, run the relevant verification commands and create a git commit for the completed work. Stage only files that belong to the change; leave unrelated user or workspace changes untouched unless explicitly instructed.

## Security & Configuration Tips

Do not commit real secrets. Development defaults live in `configs/config.yaml`; production must override weak JWT secrets. Source configs are sensitive, especially email credentials. Preserve user scoping for authenticated resources and avoid bypassing `netguard` on outbound fetches.
