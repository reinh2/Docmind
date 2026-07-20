# DocMind

[![CI](https://github.com/reinh2/docmind/actions/workflows/ci.yml/badge.svg)](https://github.com/reinh2/docmind/actions/workflows/ci.yml)
[![Go 1.22](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**DocMind** is a portfolio-grade, source-grounded RAG assistant for a company knowledge base. It accepts internal documents, indexes them in PostgreSQL with pgvector, answers customer questions through an embeddable web widget or Telegram, and retains the evidence behind every response for review in an admin console.

The demo is preloaded with fictional documents from a Berlin law firm, but the ingestion, retrieval, and chat layers are domain-neutral.

> **Why this exists:** most RAG demos stop at "embed some text and call an LLM." DocMind is built to show the parts that actually matter in production — an evidence gate that refuses to guess, a full audit trail behind every answer, SSRF-hardened ingestion, prompt-injection defenses, token streaming, and operational plumbing (health checks, metrics, rate limiting, migrations, CI).

## What it demonstrates

- A Go backend structured around domain services and adapter interfaces.
- Asynchronous document ingestion for TXT, Markdown, HTML, text-based PDF, and vetted remote URLs.
- Hybrid retrieval: vector similarity plus PostgreSQL full-text search, combined with Reciprocal Rank Fusion (RRF).
- Evidence gating: when retrieval is insufficient, the assistant returns a clear not-found response instead of inventing an answer.
- An audit trail for conversations, source chunks, feedback, escalations, failures, and document versions.
- A dependency-free, single-script customer widget with SSE responses, source disclosure, feedback, and human handoff.
- A React/Vite admin console for document operations, conversation inspection, dashboard metrics, and source debugging.
- Optional Telegram long polling using the same conversation and retrieval pipeline.
- Docker Compose, PostgreSQL migrations, nginx edge routing, health checks, Prometheus-style metrics, and CI.

## Screenshots

| Overview dashboard | Knowledge management |
|---|---|
| [![Admin overview](docs/admin-overview.png)](docs/admin-overview.png) | [![Knowledge management](docs/admin-knowledge.png)](docs/admin-knowledge.png) |

| Conversation & evidence inspector | Embeddable customer widget |
|---|---|
| [![Conversations](docs/admin-conversations.png)](docs/admin-conversations.png) | [![Widget](docs/widget.png)](docs/widget.png) |

## Demo in three commands

Requirements: Docker Desktop with Docker Compose. Nothing else is needed for the standard demo.

```bash
git clone https://github.com/reinh2/docmind.git
cd docmind
make up
```

Then open [http://localhost:18080](http://localhost:18080). The demo admin token is `demo-admin-token`.

Compose starts PostgreSQL with pgvector, applies migrations, indexes three sample documents, builds the admin console, and exposes nginx on port `18080`.

Check readiness and ask the seeded knowledge base a question:

```bash
curl http://localhost:18080/api/readyz

curl -N http://localhost:18080/api/v1/chat/messages \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"demo-browser","question":"How much does an initial consultation cost?"}'
```

The chat endpoint streams Server-Sent Events: one `sources` event with the evidence, then a series of `token` events as the answer is generated, and a final `complete` event (a `token`-less `error` event is emitted instead if generation fails mid-stream). In demo mode, the response should state that an initial consultation costs **EUR 190 plus VAT** and provide the source chunks used.

Stop the demo while keeping the database volume:

```bash
docker compose down
```

To remove the demo data as well:

```bash
make clean
```

`make help` lists the available Docker shortcuts. The most useful ones are `make logs`, `make health`, `make seed`, `make telegram`, and `make down`.

## Architecture

```mermaid
flowchart LR
  Website[Customer website] --> Widget[Embeddable widget]
  Telegram[Telegram] --> Bot[Telegram adapter]
  Admin[React admin console] --> Edge[nginx :18080]
  Widget --> Edge
  Edge --> API[Go HTTP API]
  Bot --> Chat[Chat service]
  API --> Ingest[Ingestion service]
  API --> Chat
  Ingest --> AI[Fake provider or OpenRouter]
  Chat --> AI
  Ingest --> DB[(PostgreSQL + pgvector)]
  Chat --> DB
```

The public nginx gateway serves the admin app at `/`, the widget at `/docmind-widget.js`, and proxies API routes under `/api/*`. The Go application itself uses the routes without `/api`; this is useful when running the API directly during development.

## How a document becomes an answer

1. An administrator uploads a file or submits a URL. The API checks the request size and authentication; URL ingestion also rejects loopback, private, link-local, multicast, and unspecified network addresses.
2. The source is stored in an ingestion job and queued. A worker extracts text, removes HTML scripts/styles, splits the text into sentence-aware overlapping chunks, and creates an embedding for each chunk.
3. Chunks are published as a new immutable document version in one database transaction. Only active versions participate in retrieval, so a reindex does not expose a partially indexed document.
4. For a customer question, the service first persists the user message, then optionally rewrites the search query, runs vector and full-text searches, and fuses their ranked results with RRF.
5. The highest fused score must pass `DOCMIND_RETRIEVAL_THRESHOLD`. Otherwise the assistant receives no context and responds that the knowledge base does not contain the answer.
6. The provider produces an answer from the selected chunks and streams it token by token. The HTTP API forwards the evidence first, then relays each token as it arrives; the assistant message and exact source snapshot are persisted once generation finishes.

```mermaid
sequenceDiagram
  participant A as Admin
  participant I as Ingestion worker
  participant DB as PostgreSQL + pgvector
  participant U as Customer channel
  participant C as Chat service
  A->>I: Upload document / URL
  I->>I: Extract, chunk, embed
  I->>DB: Publish active version atomically
  U->>C: Ask question
  C->>DB: Save question, vector search, full-text search
  C->>C: Fuse ranks and apply evidence gate
  C-->>U: SSE: sources, then streamed tokens
  C->>DB: Save answer and source snapshot
  C-->>U: SSE: complete
```

## Features

### Knowledge ingestion

| Capability | Details |
|---|---|
| File formats | `.txt`, `.md`, `.markdown`, `.html`, `.htm`, and text-based `.pdf` |
| Size limits | 5 MiB for uploads and remote documents |
| HTML parsing | Strips `script` and `style` elements and reduces markup to text |
| PDF parsing | Uses Poppler `pdftotext`; scanned/image-only PDFs are intentionally rejected |
| URL ingestion | HTTP(S) only, 12-second client timeout, up to 3 redirects, Content-Type allow-list, DNS and dial-time SSRF checks |
| Deduplication | SHA-256 hash of extracted text prevents duplicate documents |
| Reindexing | Publishes a replacement version transactionally |
| Deletion | Soft-deletes a document so it disappears from retrieval |

### Retrieval and answer quality

- Vector search uses 64-dimensional `pgvector` cosine similarity.
- Full-text search uses PostgreSQL `websearch_to_tsquery` and a GIN index.
- RRF combines rank positions rather than incomparable raw search scores.
- Up to five evidence chunks are passed to the answer provider.
- The production prompt treats source blocks as untrusted reference data and instructs the model to ignore instructions inside them.
- Answers stream token by token: OpenRouter is consumed as an SSE stream and relayed to the client, and the offline fake provider emits word-sized deltas so the streaming contract is exercised without credentials.
- Every answer stores its source chunk IDs, ranks, scores, excerpts, and document metadata.

### Customer channels

- The standalone widget renders its own UI, streams the answer via `fetch` + SSE, displays source excerpts, accepts 👍/👎 feedback, and requests a human handoff.
- The Telegram bot supports `/start` and `/human` and delegates normal messages to the same chat service.
- Escalations are durable database records and are emitted to the current structured-log notifier.

### Operations and safety

- Admin routes use a Bearer token with constant-time comparison.
- The HTTP server has panic recovery, request logging, body limits, configurable CORS, and in-memory IP/session rate limiting.
- `/healthz` is a liveness probe; `/readyz` additionally verifies PostgreSQL connectivity.
- `/metrics` exposes a small Prometheus-compatible registry for queue depth, provider errors, not-found answers, escalation count, and retrieval/generation latency.
- The runtime container is non-root; migrations are protected with a PostgreSQL advisory lock.

## Design decisions

A few choices are deliberate and worth calling out, because they are where a real system diverges from a tutorial.

- **An evidence gate, not a confidence score.** Retrieval must clear `DOCMIND_RETRIEVAL_THRESHOLD` on the fused RRF score or the model is handed *no* context and told to say the answer is not in the knowledge base. Refusing to answer is treated as a feature, not a failure mode — it is what keeps a support bot from inventing policy.
- **Rank fusion over raw scores.** Vector cosine distances and `ts_rank` values are not on comparable scales, so blending them numerically is meaningless. RRF fuses *rank positions*, which is scale-free and robust when one retriever is confident and the other is noisy.
- **Save-first conversation flow.** `Ask` persists the user's question before any rewrite, retrieval, or generation, and routes provider failures to a dead-letter record. A question is never lost because a downstream model call failed.
- **Immutable document versions.** A reindex publishes a brand-new active version in a single transaction; only active versions are retrievable. A half-indexed document can never leak into an answer, and rollback is just flipping which version is active.
- **Sources as untrusted data.** Retrieved chunks are wrapped in explicit `SOURCE` delimiters and the system prompt instructs the model to treat everything inside as reference data, never instructions — a concrete defense against prompt injection smuggled into an ingested document. The same posture drives the SSRF checks on URL ingestion.
- **A provider port with a deterministic fake.** All model behavior sits behind one `ai.Provider` interface. The offline `FakeProvider` makes the entire system — ingestion, retrieval, streaming, audit — runnable and testable with zero credentials and zero network, which is what makes the three-command demo possible.
- **Stream from the edge.** The answer is relayed to the browser token by token over SSE, with the evidence sent up front so the UI can disclose *what* it is reading before it says anything.

## Admin console

Go to [http://localhost:18080](http://localhost:18080) and enter `demo-admin-token` when running Compose with defaults.

The console provides:

- **Overview:** questions today, not-found ratio, escalation ratio, average latency, feedback counts, and top questions.
- **Knowledge:** document upload, list active documents, reindex, and remove from retrieval.
- **Conversations:** filter by widget or Telegram, inspect the complete transcript, and expand the chunks supplied to the model.

## Embed the widget

Add one script tag to a customer-facing page:

```html
<script
  src="https://api.example.com/docmind-widget.js"
  data-docmind-api="https://api.example.com"
  data-docmind-title="Ask our team"
></script>
```

`data-docmind-api` may be omitted when the widget and API share an origin. The widget posts to `/v1/chat/messages` relative to that value, so configure the value without the nginx `/api` prefix when it points to a direct Go API; with the supplied nginx deployment, set it to `https://api.example.com/api`.

For a cross-origin host page, allow its origin explicitly:

```env
DOCMIND_CORS_ORIGINS=https://www.example.com
```

The widget also exposes `window.DocMindWidget.init(options)` for manual initialization. Set `data-docmind-autostart="false"` to disable automatic initialization.

## API reference

All paths below are Go API paths. Through the included nginx gateway, add `/api` before every API path. `GET /docmind-widget.js` is the exception and is served at the gateway root.

| Method and path | Auth | Purpose |
|---|---:|---|
| `POST /v1/admin/documents` | Admin | Multipart upload; returns a job or deduplication result |
| `POST /v1/admin/documents/url` | Admin | Fetch and queue an HTTP(S) document |
| `GET /v1/admin/documents` | Admin | List active documents |
| `POST /v1/admin/documents/{id}/reindex` | Admin | Queue a new version from the last successful source |
| `DELETE /v1/admin/documents/{id}` | Admin | Soft-delete a document |
| `GET /v1/admin/jobs/{id}` | Admin | Get ingestion state |
| `GET /v1/admin/dashboard` | Admin | Dashboard aggregates and top questions |
| `GET /v1/admin/conversations` | Admin | List conversations; accepts `channel` and `limit` |
| `GET /v1/admin/conversations/{id}` | Admin | Transcript and persisted retrieval sources |
| `POST /v1/chat/messages` | Public | Ask a question; returns SSE |
| `POST /v1/chat/messages/{id}/feedback` | Public | Save a rating of `1` or `-1` and an optional comment |
| `POST /v1/chat/conversations/{id}/escalate` | Public | Create a human-handoff request |
| `GET /healthz` | Public | Process liveness |
| `GET /readyz` | Public | Database-aware readiness |
| `GET /metrics` | Public | Prometheus text metrics |
| `GET /docmind-widget.js` | Public | Single-file embeddable widget |

### Upload and poll an ingestion job

```bash
curl -X POST http://localhost:18080/api/v1/admin/documents \
  -H 'Authorization: Bearer demo-admin-token' \
  -F 'file=@./handbook.md' \
  -F 'source_type=markdown'

curl http://localhost:18080/api/v1/admin/jobs/<job-id> \
  -H 'Authorization: Bearer demo-admin-token'
```

The first request returns `202 Accepted` for a new document. Poll until `status` is `completed` or `failed`. A duplicate is returned as `200 OK` with `duplicate: true`.

### Ask a question

```bash
curl -N http://localhost:18080/api/v1/chat/messages \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id": "website-visitor-42",
    "channel": "widget",
    "question": "How do I change an appointment?"
  }'
```

`session_id` is optional; the server generates one when absent. `channel` defaults to `widget`. A question may be 1–4,000 characters long.

## Configuration

Copy the sample file before changing Compose defaults:

```bash
cp .env.example .env
```

| Variable | Default | Meaning |
|---|---|---|
| `DOCMIND_HTTP_ADDR` | `:8080` | Go API listen address |
| `DOCMIND_DATABASE_URL` | local PostgreSQL URL | PostgreSQL connection string |
| `DOCMIND_ADMIN_TOKEN` | `change-me-before-production` | Bearer token for admin routes |
| `DOCMIND_AI_MODE` | `fake` | `fake` for an offline demo, `openrouter` for production provider calls |
| `OPENROUTER_API_KEY` | empty | Required in `openrouter` mode |
| `OPENROUTER_CHAT_MODEL` | empty in code / example supplied | Chat-completions model |
| `OPENROUTER_EMBEDDING_MODEL` | empty in code / example supplied | Embeddings model; it must support 64 dimensions |
| `DOCMIND_QUEUE_CAPACITY` | `32` | Buffered in-process ingestion queue capacity |
| `DOCMIND_CHUNK_SIZE` | `900` | Target chunk size in characters; minimum 100 |
| `DOCMIND_CHUNK_OVERLAP` | `160` | Sentence-level overlap; must be smaller than chunk size |
| `DOCMIND_RETRIEVAL_THRESHOLD` | `0.01` | Minimum fused RRF score to answer from evidence |
| `DOCMIND_CORS_ORIGINS` | `*` | Comma-separated origins or `*` |
| `DOCMIND_RATE_LIMIT_PER_MINUTE` | `60` | Per-IP and per-session fixed-window limit |
| `DOCMIND_HOST_PORT` | `18080` | nginx host port in Compose |
| `TELEGRAM_BOT_TOKEN` | empty | Enables the Telegram adapter |

### AI modes

`fake` is the default and is deliberately deterministic: it hashes tokens into 64-dimensional embeddings and returns an extractive answer from the best chunk. It lets the complete system run locally without credentials, but it is not a substitute for a language model.

With `DOCMIND_AI_MODE=openrouter`, DocMind uses OpenRouter’s embeddings and chat-completions APIs. It applies a 25-second HTTP timeout and retries temporary failures (408, 429, and 5xx) with exponential backoff. The provider requires all three OpenRouter variables.

## Telegram

Set `TELEGRAM_BOT_TOKEN` in `.env`, then run:

```bash
docker compose --profile telegram up --build
```

Use `/start` to introduce the bot and `/human` after asking at least one question to request a handoff.

## Local development and verification

Requirements for non-Docker development: Go 1.22+, Node.js 22+, npm, PostgreSQL 15+ with pgvector, and Poppler (`pdftotext`) for PDF ingestion.

```bash
GOCACHE=/tmp/docmind-go-build go test ./...
GOCACHE=/tmp/docmind-go-build go vet ./...
GOCACHE=/tmp/docmind-go-build go test -race ./...

npm --prefix web/admin ci
npm --prefix web/admin run build
```

Optional PostgreSQL integration tests require a dedicated reachable database:

```bash
DOCMIND_TEST_DATABASE_URL='postgres://docmind:docmind@localhost:5432/docmind?sslmode=disable' \
  GOCACHE=/tmp/docmind-go-build go test -tags=integration ./tests/integration
```

The GitHub Actions workflow in [`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs unit tests, `go vet`, race tests, the Vite build, Docker builds, and a Compose smoke test.

## Repository layout

```text
cmd/                 Application entry points: API, seed data, Telegram bot
internal/ai/         Provider interface, deterministic demo provider, OpenRouter adapter
internal/chat/       Conversation lifecycle, retrieval orchestration, feedback, escalation
internal/ingestion/  Parsing, PDF and URL fetching, chunking, background indexing
internal/retrieval/  Reciprocal Rank Fusion
internal/store/      PostgreSQL and pgvector persistence
internal/httpapi/    Routes, middleware, SSE, embedded widget asset
web/admin/           React/Vite administrative interface
deploy/nginx/        Static admin hosting and API reverse proxy
migrations/          PostgreSQL schema migrations (mirrored for embedding)
tests/integration/   Opt-in end-to-end PostgreSQL tests
```

## Known limitations and production next steps

This is a strong portfolio demonstrator, not an unattended production SaaS. The most important next steps are:

- Replace the single in-process ingestion worker with a durable worker system; jobs interrupted while marked `processing` are not automatically reclaimed yet.
- Add tenant isolation, user authentication/authorization, secret management, HTTPS, a restrictive production CORS policy, and a non-demo admin token.
- Move rate limiting and session coordination to shared infrastructure for multiple API instances.
- Add a real escalation destination (Slack, email, CRM, or ticketing); the included notifier logs events structurally.
- Persist Telegram update offsets and the handoff lookup so `/human` works seamlessly after a bot restart.
- Add broader database and load-test coverage; the current suite covers chunking, parsing, RRF, retry and streaming behavior, rate limiting, HTTP auth/validation/CORS, and the fake-provider flow end to end.
- Replace the sample organization and OpenRouter metadata before publishing your own portfolio version.

## License

Released under the [MIT License](LICENSE).
