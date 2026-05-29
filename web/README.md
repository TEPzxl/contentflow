# contentflow web

Next.js + React + TypeScript + Tailwind CSS frontend for contentflow.

## Scripts

```fish
npm install
npm run dev
npm run typecheck
npm run lint
npm run build
```

Set `NEXT_PUBLIC_CONTENTFLOW_API_BASE_URL` when the backend is not running at `http://localhost:8080/api/v1`.

## E2E tests

Playwright is configured to use one worker by default to keep local CPU and memory usage low. Local runs keep tracing off; CI records traces only on the first retry. The Playwright-managed dev server uses Next.js webpack dev mode with source maps and server Fast Refresh disabled to avoid Turbopack CPU spikes during E2E startup.

Use `--list` for a safe discovery check without running the full browser test suite:

```fish
npm run test -- --list
```

The real backend E2E test is opt-in. It only runs when `CONTENTFLOW_E2E_REAL_BACKEND=1` and both API base URL variables match:

```fish
env CONTENTFLOW_E2E_REAL_BACKEND=1 \
  CONTENTFLOW_E2E_API_BASE_URL=http://127.0.0.1:8080/api/v1 \
  NEXT_PUBLIC_CONTENTFLOW_API_BASE_URL=http://127.0.0.1:8080/api/v1 \
  npm run test -- tests/e2e/real-backend-auth.spec.ts
```
