# DocMind

DocMind is a production-minded RAG assistant demo for company knowledge bases. The included scenario is a Berlin legal firm, but ingestion, retrieval and channels are domain-neutral. It answers only from indexed material, exposes the source chunks used for a response, and is designed to escalate uncertain conversations to a human.

> **Current implementation:** Phase 3 — customer channels. A one-script vanilla widget and optional Telegram bot reuse the same RAG, source and audit pipeline. The React admin console, dashboard and CI workflow remain scheduled in [`ROADMAP.md`](ROADMAP.md).

## Architecture

```mermaid
flowchart LR
  Client["Customer website / Telegram"] --> Nginx["nginx : public edge"]
  Admin["React + Vite admin"] --> Nginx
  Nginx --> API["Go API"]
  API --> RAG["Ingestion, hybrid retrieval, chat"]
  RAG --> DB[("PostgreSQL + pgvector")]
  RAG --> AI["Fake AI / OpenRouter"]
  TG["Telegram long-polling"] --> RAG
```

## Quick start

```bash
docker compose up --build
open http://localhost:18080
curl http://localhost:18080/readyz
```

The public endpoint is nginx on port `18080`. It serves the React admin interface and proxies `/api/*` plus the embeddable widget to the Go service. Log in with `demo-admin-token` in demo mode.

The `seed` service indexes three fictional firm documents automatically. Ask a grounded question:

```bash
curl -N http://localhost:18080/v1/chat/messages \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"demo-browser","question":"How much does an initial consultation cost?"}'
```

The response is Server-Sent Events: `sources`, `token`, and `complete`. The `sources` payload contains the document name, section, excerpt, RRF score and chunk ID for a future widget to display as a disclosure.

## Embed the chat widget

Add exactly one script tag to any page. It injects its own scoped UI, streams responses using `fetch` + SSE, exposes source excerpts, collects 👍/👎 feedback, and offers a human-handoff button.

```html
<script
  src="https://api.example.com/docmind-widget.js"
  data-docmind-api="https://api.example.com"
  data-docmind-title="Ask our team"
></script>
```

When `data-docmind-api` is omitted, the widget uses the origin serving the script. Configure `DOCMIND_CORS_ORIGINS` to include the host page’s origin before embedding on a different site.

## Telegram

The optional long-polling adapter uses the same saved conversations and sources as the widget. Set `TELEGRAM_BOT_TOKEN` and start it with:

```bash
docker compose --profile telegram up --build
```

Use `/start` to introduce the bot and `/human` to escalate the active Telegram conversation.

## Upload a document

```bash
curl -X POST http://localhost:18080/v1/admin/documents \
  -H 'Authorization: Bearer demo-admin-token' \
  -F 'file=@./example.md' \
  -F 'source_type=markdown'
```

The API returns `202 Accepted` and an ingestion job ID. Poll it until its status is `completed`:

```bash
curl -H 'Authorization: Bearer demo-admin-token' \
  http://localhost:18080/v1/admin/jobs/<job-id>
```

Supported now: TXT, Markdown, HTML and text-based PDF. URL ingestion accepts `text/html`, `text/plain`, Markdown and PDF response types; it rejects private, loopback and link-local destinations.

## API contracts

| Endpoint | Description |
|---|---|
| `POST /v1/admin/documents` | authenticated multipart upload; returns a job or a deduplication result |
| `GET /v1/admin/jobs/{id}` | ingestion job state |
| `GET /v1/admin/documents` | active documents |
| `POST /v1/admin/documents/url` | authenticated JSON URL ingestion with SSRF protection |
| `POST /v1/admin/documents/{id}/reindex` | creates a fresh version from the last successful source |
| `DELETE /v1/admin/documents/{id}` | soft-deletes a document from retrieval |
| `POST /v1/chat/messages` | creates/reuses a session, saves a question, then emits answer SSE |
| `POST /v1/chat/messages/{id}/feedback` | saves a 👍 (`1`) or 👎 (`-1`) evaluation |
| `POST /v1/chat/conversations/{id}/escalate` | creates an operator-handoff event |
| `GET /docmind-widget.js` | single-file embeddable widget |
| `GET /metrics` | Prometheus-compatible operational metrics |
| `GET /v1/admin/dashboard` | dashboard aggregates and top questions |
| `GET /v1/admin/conversations` | filterable conversation list |
| `GET /v1/admin/conversations/{id}` | full transcript with retrieval context |
| `GET /healthz` | process liveness |
| `GET /readyz` | database-aware readiness |

## Design decisions

- **Hybrid retrieval with RRF:** pgvector catches semantic variants and PostgreSQL full-text search catches exact legal/product language. Reciprocal Rank Fusion avoids assuming their raw scores are comparable.
- **Evidence gate:** after fusion, a relevance threshold is applied before generation. Missing evidence returns a clear not-found response instead of a fabricated answer.
- **Save-first:** the user message is saved before query rewriting, retrieval, or generation. Provider failure can therefore be audited and later delivered through a dead-letter path (Phase 2).
- **Atomic version publishing:** new chunks are staged, then the active document version is flipped in one transaction. Retrieval reads only active versions.
- **Offline demo mode:** `FakeProvider` generates stable 64-dimensional embeddings and grounded local answers, so no API key or external AI call is necessary.
- **Production AI:** `DOCMIND_AI_MODE=openrouter` activates an HTTP adapter with 25-second request timeouts and exponential retry for temporary provider failures. It uses OpenRouter's documented `/api/v1/embeddings` endpoint and requests a 64-dimensional output to match the pgvector schema. [OpenRouter embeddings reference](https://openrouter.ai/docs/api/reference/embeddings)

## Development checks

```bash
go test ./...
go vet ./...
GOCACHE=/tmp/docmind-go-build go test -race ./...
npm --prefix web/admin run build
```

## CI and operations

GitHub Actions in [`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs Go tests, `go vet`, the race detector, the Vite build, Docker image builds, and an nginx/Compose smoke test. Runtime instrumentation is exposed on `/api/metrics`; readiness remains `/api/readyz` through nginx.

The React admin console is intentionally a thin client: it uses an admin Bearer token, lists active documents, queues uploads/reindexing, and shows the exact sources provided to each model answer. This makes retrieval failures debuggable rather than opaque.

With a dedicated PostgreSQL test database, the optional end-to-end and reindex tests run with:

```bash
DOCMIND_TEST_DATABASE_URL='postgres://docmind:docmind@localhost:5432/docmind?sslmode=disable' \
  go test -tags=integration ./tests/integration
```

## Configuration

All runtime configuration comes from environment variables; see [`.env.example`](.env.example). Compose supplies safe demo defaults, so copying this file is optional; create `.env` from it before changing configuration. Never use the demo admin token outside local development. Set `DOCMIND_AI_MODE=openrouter` only with an API key and explicit model configuration.

## Limits in this increment

There is intentionally no React admin UI, dashboard, token-by-token upstream model streaming, Slack/Telegram operator-notification connector, or CI workflow yet. Those features are tracked with acceptance gates in [`ROADMAP.md`](ROADMAP.md), rather than being represented by misleading stubs.
