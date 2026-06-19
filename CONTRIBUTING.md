# Contributing to Cloudrift

The full contribution guide — philosophy, architecture, where to add things, and conventions —
now lives in the consolidated docs:

- **Interactive:** open [docs.html](docs.html) and choose the **Developer** audience view.
- **Markdown:** [docs/cloudrift-docs.md → Contributing](docs/cloudrift-docs.md#contributing).

## Quick reference

- Keep AWS I/O in `internal/collectors/`, business logic in `internal/scorers/` and
  `internal/validators/`, scan orchestration in `internal/pipeline/`, HTTP handlers in
  `internal/api/`, and the LLM answer layer in `internal/synth/`.
- `internal/pipeline` is the single scan entry point used by both the `scan` CLI and the
  dashboard; AWS access sits behind the `Source` interface so the scoring/persistence stages
  are testable without AWS.
- Run `go test ./...` before opening a PR (`make test` runs everything). The pipeline
  integration test (`internal/pipeline/pipeline_test.go`) drives the full pipeline with a fake
  source — no AWS/network; the `internal/synth` tests are httptest-based and need no API key.
- Frontend: React 18 + Vite 5 + Tailwind + TanStack Query. `npm run dev` in `dashboard/`
  proxies `/api` to `http://127.0.0.1:8080`.

JSON on disk is the source of truth for findings; the API serves it; Neo4j is the optional
graph-tier projection. Read-oriented audit role — no admin-by-default. Prefer tests for
scorers and API response shapes (stable `[]`, not `null`, for lists).
