# nebulaIQ — Table for you

An English chat agent that researches restaurants for a city and meal, retrieves menus and review passages, translates dishes, and separates source-supported choices from options that require confirmation.

**Current implementation: Go backend, PostgreSQL, Gemini and Tavily, with the connected frontend. Account signup/signin is deferred until this stage is reviewed.** Browser sessions isolate history in the meantime. The original fictional-data prototype remains under `outputs/frontend` as a design artifact; the runnable app is embedded from `web/assets`.

## Run locally

Requirements: Go 1.26 or compatible later version, Docker Desktop with Linux containers, Gemini API access and a Tavily API key. Model/search billing and quotas belong to your provider accounts; this app does not purchase credits or switch to a paid fallback.

1. Start PostgreSQL:

   ```powershell
   docker compose -p nebulaiq up -d postgres
   ```

2. Copy `.env.example` to `.env` if `.env` does not already exist. Put real keys **only in `.env` or environment variables**. Environment variables take precedence. `.env` is ignored by Git. The default database credentials are exclusively for the localhost Docker database.

3. Build, migrate and serve from the repository root:

   ```powershell
   go build -o bin/nebula.exe ./cmd/nebula
   ./bin/nebula.exe migrate
   ./bin/nebula.exe serve
   ```

   Linux/macOS: `go build -o bin/nebula ./cmd/nebula`, then `./bin/nebula migrate` and `./bin/nebula serve`.

4. Open [http://127.0.0.1:8080](http://127.0.0.1:8080). Local mode binds to loopback. If no reviewer code is configured, the frontend creates a private guest session automatically.

Example request: `Find vegetarian lunch in Barcelona, Spain, with regular menu dishes under EUR 25 per dish.` Follow up with a lower budget or mandatory exclusions. Reopening a conversation reads PostgreSQL without external provider calls. Refresh bypasses search/document caches for a new run.

## Configuration

| Variable | Purpose |
|---|---|
| DATABASE_URL | PostgreSQL URL; required |
| GEMINI_API_KEY / TAVILY_API_KEY | Server-only provider credentials |
| MODEL_PROVIDER | Currently `gemini`; unsupported values fail explicitly |
| MODEL_NAME | Default `gemini-3.1-flash-lite`; configure a model available to your project |
| APP_ENV | `local` for loopback; `production` for hosting |
| PORT | Default 8080; Render injects its port |
| REVIEWER_ACCESS_CODE | Required outside local mode; share separately from the repository |
| MAX_DAILY_RUNS_PER_OWNER / GLOBAL | Defaults 20 / 100 accepted runs per UTC day |
| NEBULA_ENV_FILE | Optional alternative private env-file path |

The Gemini account used for verification rejected `gemini-2.5-flash` as unavailable to new users and directed use of `gemini-3.8-flash`. Full requests to gemini-3.8-flash returned HTTP 503 high-demand errors. A structured interpretation check succeeded with gemini-3.1-flash-lite, which is explicitly configured as the default; there is no automatic model fallback. Model access remains account-specific. No OpenAI adapter is implemented yet; the model and search interfaces are replaceable independently of storage/workflow.

## Architecture and evidence rules

- One Go service embeds the frontend and SQL migrations. PostgreSQL holds owners, browser sessions, conversations, messages, runs, events, caches, independent daily quota counters and immutable historical citation snapshots.
- Stable owner IDs are separate from browser-session IDs. Later account migration can add users and authentication sessions without rewriting the agent tables.
- The bounded workflow interprets requirements, shortlists up to three branches, researches menus and positive/negative/context-specific reviews, processes each restaurant separately with literal passage retrieval, validates citations and ranks supported dishes. Progress reports actual stages, not model chain of thought.
- Evidence quotes must occur in retrieved text. Missing ingredients stay unknown. Search snippets are incomplete menu evidence. Meal availability needs a cited label. Prices use decimal strings/rational comparisons. Currency comes from the item or the same menu; a verified branch can supply a clearly labeled local-currency assumption. No currency conversion is performed.
- The language-marker validation currently recognizes selected English, Spanish, French, Italian and German dietary/meal phrases. Other source languages may remain unknown despite translation; coverage is expandable and is not a worldwide guarantee.
- Search cache: six hours, owner + exact query and parameters. Menu documents: 24 hours; review documents: six hours. Extraction cache includes source hashes and model/prompt/schema/requirements versions. Budget changes can reuse compatible extraction, while dietary changes require reassessment.
- History expires seven days after activity. Guest cookie loss/expiry prevents recovery. Tokens are hashed at rest; requests enforce ownership, same-origin headers and cookie boundaries. Basic reviewer access is **not** signup/signin.
- Maximum two concurrent runs globally and one per conversation, 14 searches, 20 source-fetch attempts (including visual files), nine model calls including bounded retries and transcription, and a 180-second run deadline. At most two PDF/image transcriptions and two targeted follow-up rounds are allowed. SDK automatic retries are disabled; one application-controlled retry of eligible transient model failures consumes the call budget.
- PostgreSQL stores completed results and progress. Expired run leases become interrupted and require an explicit retry. There is no automatic provider-call replay after a restart.

## Verify

Domain/provider tests run without external paid calls:

```powershell
go test ./...
go vet ./...
```

Database integration tests need an isolated database. With the local container:

```powershell
docker exec nebulaiq-postgres-1 psql -U nebula -d nebula -c "CREATE DATABASE nebula_test"
$env:TEST_DATABASE_URL = "postgres://nebula:nebula_local_only@127.0.0.1:5432/nebula_test?sslmode=disable"
go test ./...
```

Each integration test creates and removes a unique schema inside the test database. Do not set TEST_DATABASE_URL to a database containing important data. Tests skip database cases when it is absent; skipped tests are not evidence of verified persistence.

`go run ./cmd/providercheck` performs one tiny real Gemini access check using configured credentials. It does not print keys. Live restaurant research also uses the configured providers and consumes quota. Manual inspection of menu/review citations is required; unit tests do not prove a live restaurant's facts.

## Hosting

The included `render.yaml` describes one free Go service and one free PostgreSQL database. Deployment has not been performed. Configure provider keys and the reviewer code in Render, not the YAML. Frontend and API share one origin. The local binary does not contain keys or PostgreSQL and needs runtime configuration.

As documented in the HLD, Render Free can sleep after 15 minutes idle; waking can take approximately one minute. Free PostgreSQL expires after 30 days and has no managed backups. Record actual expiry when provisioning; accounts would not change this limit. No paid upgrade is authorized.

## Limitations and deferred work

No signup/signin, profile editor, email verification/reset, bookings, availability guarantee, medical dietary certification, continuous OCR processing, dynamic browser scraping, continuous crawling, vector database or OpenAI adapter. Review text may be blocked or available only as snippets; absence of criticism is shown explicitly, not replaced with invented negative reviews. Tavily provides advanced text extraction. Gemini can additionally transcribe public PDFs and menu images within a bounded budget; these sources are labeled model transcriptions and require checking against the original file. Source text is bounded, so long pages may be incomplete. Strict ingredient exclusions and gluten-free/vegan requests require explicit supported dietary evidence; unverified dishes are excluded from the shortlist. Confirmed matches require supported meal and price evidence, which many real menus omit; a run can complete with only options requiring confirmation. Menu symbol legends are applied only when defined in the same source. Linked official and third-party menu prices, including older menus, are accepted as planning guidance. Price freshness is not verified in this assignment; source links allow checking the amount, but do not guarantee its accuracy. Amounts are not presented as independently verified current prices. Delivery-platform menus are excluded from regular-menu extraction because platform prices, fees and display currencies can differ from the restaurant menu. Shortlist names must occur in search passages; branch location citations are bound to the restaurant name/address. Saved results from older validation versions display a refresh note. This assignment demo is not production infrastructure.

See [HLD](outputs/HLD.md), [implementation plan](outputs/IMPLEMENTATION_PLAN.md), and [implementation verification](outputs/IMPLEMENTATION_STATUS.md). GitHub publication, hosted URL and Loom recording remain submission tasks; no placeholder URL is represented as a completed deliverable.


## Evidence follow-ups

Interpretation receives a bounded structured summary of the latest matching research result, including menu quotes, portion variants, prices, source links, snippet coverage and remaining checks. The model selects research, answer_existing or clarify. Explanations reuse owned conversation evidence without search/fetch calls, preserve cards and original retrieval dates, and persist their actual answer. Explicit refresh and changed requirements still run research. Follow-up answers use source-listed amounts without age warnings and never invent missing amounts.


## Optional Google-grounded research

Standard Gemini + Tavily remains the default. The composer switch enables normal Gemini Google Search grounding and URL context after a cost warning. This is not Antigravity. Grounded prose, citations, Google Search suggestions and research metadata are stored separately from validated restaurant cards. The switch resets on page load/new conversation. Both routes share clarification, history and cancellation. Google may issue multiple queries; billing is variable. No automatic paid retry or fallback.

Run `nebula migrate` before `nebula serve` for migration 003. Defaults: `ENABLE_GOOGLE_GROUNDED=true`, `GROUNDED_MODEL_NAME=gemini-3.8-flash`, `MAX_DAILY_GROUNDED_PER_OWNER=3`, `MAX_DAILY_GROUNDED_GLOBAL=10`, `GROUNDED_MAX_OUTPUT_TOKENS=8192`. The existing Gemini key is used only on the server. Disable with `ENABLE_GOOGLE_GROUNDED=false`. Quotas count accepted grounded turns, including clarification/follow-up, and survive history deletion. Owner limits are browser-bound; global limits prevent cookie resets from bypassing the whole budget. Limits do not guarantee an exact monetary ceiling.


## Restructured workspace

Backend Go code is in backend/. Frontend sources and its minimal Go embedding adapter are in frontend/. From the project root, copy .env.example to backend/.env for a new setup; the existing private backend/.env was preserved and is ignored by Git. Run Go commands from backend/, for example: go run ./cmd/nebula serve. The Docker Compose definition is docker-compose.yml. The frontend module allows the single backend binary to embed assets without a duplicate frontend copy.
