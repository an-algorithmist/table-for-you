# Table for You

Assignment-grade English restaurant research chat: city, meal and dietary requirements go through interpretation, restaurant discovery, menu/review retrieval, translation and deterministic citation validation. Standard research uses Gemini plus Tavily. Optional Google-grounded research remains explicitly selected and cost-acknowledged.

## Layout

```text
table-for-you/
  backend/
    cmd/server/          # startup, dependency wiring, graceful shutdown
    cmd/providercheck/   # manually invoked, potentially billable diagnostic
    config/countries.yaml
    internal/
      config/            # environment and embedded country settings
      domain/            # JSON contracts and evidence snapshots
      httpapi/           # sessions, admission, handlers, persisted SSE replay
      pipeline/          # intent, discovery, retrieval, extraction, validation stages
      llm/               # Gemini SDK, plain retry policy, run budget, embedded prompts
      tools/             # Tavily search/extraction and safe menu-file retrieval
      providers/         # SDK-independent HTTP errors and URL boundaries
      evidence/          # deterministic dietary, price and citation rules
      storage/postgres/  # session, conversation, run, event and cache repositories
    migrations/
  frontend/              # HTML, vanilla JS, CSS and small Go embedding module
  go.work
  docker-compose.yml
```

Two Go modules permit the backend binary to embed sibling frontend assets without copying them or using invalid parent-directory embed patterns. Run the commands below from this repository root unless a command explicitly enters `backend`.

## Run locally

Requirements: Go 1.26 or newer, Docker Desktop with Linux containers, runtime Gemini and Tavily credentials.

```powershell
docker compose up -d postgres
Copy-Item .env.example backend/.env # only for a NEW setup; never overwrite existing keys
Set-Location backend
go build -o bin/nebula.exe ./cmd/server
./bin/nebula.exe migrate
./bin/nebula.exe serve
```

Linux/macOS: build `bin/nebula` and run `./bin/nebula migrate`, then `./bin/nebula serve`. Open http://127.0.0.1:8080. Frontend files and SQL migrations are embedded in the binary; provider keys and PostgreSQL remain runtime dependencies.

Put keys only in `backend/.env` or process environment. Existing environment variables take precedence. `NEBULA_ENV_FILE` can select another private env file. The preserved private env file and binaries are ignored by Git.

Example: `Find vegetarian lunch in Barcelona, Spain, under EUR 25 per dish. Show dishes, descriptions, prices and restaurant/menu links.`

## Configuration

| Variable | Default / purpose |
|---|---|
| DATABASE_URL | Required PostgreSQL connection |
| GEMINI_API_KEY / TAVILY_API_KEY | Server-only credentials |
| MODEL_PROVIDER | `gemini`; unsupported values fail explicitly |
| MODEL_NAME | `gemini-3.1-flash-lite`; account access must be checked manually |
| APP_ENV / PORT | `local` / `8080`; local binds to loopback |
| REVIEWER_ACCESS_CODE | Required outside local mode; separate from account authentication |
| MAX_DAILY_RUNS_PER_OWNER / MAX_DAILY_RUNS_GLOBAL | 20 / 100 accepted runs per UTC day |
| MAX_MODEL_CALLS_PER_RUN | 9; configured maximum 20, shared across plain/grounded/visual attempts |
| ENABLE_GOOGLE_GROUNDED / GROUNDED_MODEL_NAME | `true` / `gemini-3.8-flash` |
| MAX_DAILY_GROUNDED_PER_OWNER / MAX_DAILY_GROUNDED_GLOBAL | 3 / 10 accepted turns, including clarification |
| GROUNDED_MAX_OUTPUT_TOKENS | 8192 maximum |

Country currencies, menu-search terms and known branch URL slugs live in `backend/config/countries.yaml`. This file uses the JSON subset of YAML 1.2 so the standard JSON decoder can read it without a new runtime dependency. Keep that syntax when editing. Unknown countries retain generic searches and an unspecified currency. Local currency defaults require verified branch location; they do not invent a source-listed currency.

## Architecture and preserved behavior

- HTTP admits owned, idempotent requests; the pipeline owns execution and cancellation. Lower layers do not import HTTP or orchestration. Model/search contracts live in the consuming pipeline package.
- Pipeline stage files cover interpretation, discovery, restaurant research, document retrieval/attribution, extraction and validation. Restaurants remain sequential because their attribution and call-budget state is shared; parallelizing that state is a separate change.
- All Gemini SDK imports are confined to `internal/llm`. Long research instructions are embedded Markdown resources rendered by `text/template`; user/source content is separate untrusted input.
- Plain model calls can retry one eligible 5xx failure. Each attempted call consumes the run budget even if token usage is unavailable. Google grounding has no automatic retry or paid fallback. Its internal search count is controlled by Google, so a request cap is not a monetary ceiling.
- PostgreSQL persists browser owners, sessions, conversations, messages, runs, traces, quotas, cached documents/extractions and immutable citation snapshots. SSE polls persisted events and supports replay after reconnect; a process restart does not replay provider calls.
- Exact citation quotes must occur in retrieved text. Strict vegan/gluten-free/ingredient exclusions require explicit evidence. Source-listed older and third-party prices are accepted as planning guidance, with source links. Prices use decimal text; no automatic currency conversion.
- Follow-up explanation turns reuse matching stored research and keep its cards without fresh searches. Changed requirements and explicit refresh run research. Mandatory ingredient exclusions survive ordinary follow-ups.
- Default limits remain two global active runs, one per conversation, 14 searches, 20 fetch attempts, at most two visual menu transcriptions, two follow-up research rounds and a 180-second deadline. Search cache is six hours; menu cache 24 hours; review cache six hours. History expires after seven days of inactivity.

## Verify without paid API calls

From the root:

```powershell
go test ./backend/... ./frontend/...
go vet ./backend/... ./frontend/...
```

Persistence and combined HTTP/pipeline tests require a separate local test database. Create `nebula_test` once, then:

```powershell
$env:TEST_DATABASE_URL = "postgres://nebula:nebula_local_only@127.0.0.1:5432/nebula_test?sslmode=disable"
go test -count=1 ./backend/... ./frontend/...
go test -race -count=1 ./backend/... ./frontend/...
```

Each test creates/drops its own schema. Missing TEST_DATABASE_URL skips integration tests, so that alone does not verify persistence. Windows race tests require a C compiler; alternatively run them in the official Go Docker image on the `nebulaiq_default` network with `postgres` as the database host.

`make test`, `make vet`, `make race`, `make lint`, `make build`, `make run`, `make migrate`, `make db-up` are convenience targets for systems with Make. Lint configuration enables govet, staticcheck and unused; unchecked best-effort legacy telemetry is not comprehensively audited by this refactor. `golangci-lint run ./backend/... ./frontend/...` uses the root config.

The tests use fake models/search and local HTTP servers, including Google-grounding fixtures. They do not call real Gemini or Tavily. The two-city HTTP/SSE test covers Barcelona/EUR and Tokyo/JPY, descriptions, reviews, history and follow-up evidence reuse. Manually running `backend/cmd/providercheck` or submitting live chat consumes provider quota. No live grounding check is part of verification.

## Limitations

No signup/signin, OpenAI adapter, bookings, maps, live availability guarantee, continuous crawling, vector database or production monitoring. Browser cookie loss prevents guest-history recovery; account authentication remains deferred. Review/menu retrieval may be blocked or incomplete. Source age is not a price gate and source links do not guarantee accuracy. No reliable evidence means a dish can be excluded or shown as requiring confirmation. The configured language-marker vocabulary is limited; adding country search settings does not automatically certify all source-language dietary phrases. Dynamic browser scraping is not implemented. Visual menu transcription remains model output requiring source inspection. Real provider availability, retrieval quality and billing must be checked manually; offline tests cannot prove those.

## Assumptions and guidance departures

English input/output; local Docker credentials are for development only; the reviewer supplies configured runtime keys/database. This refactor preserves existing JSON routes, database schema, UI flow, ownership, caching and quota semantics.

Retained parameterized pgx repositories instead of introducing GORM: an ORM conversion would change transaction behavior and schema access beyond restructuring. Retained persisted SSE instead of in-memory event channels so reconnect/restart behavior remains durable. Used stage files within one cohesive pipeline package rather than adding package cycles or abstractions with no independent consumers. Kept sequential restaurant work pending a separate concurrency design. Retained the existing visual fallback and follow-ups even though the suggested first-version guidance would defer them. No additional runtime dependencies were added.

## Hosting and submission

`render.yaml` is a deployment blueprint, updated for `backend/cmd/server`. Deployment, provider eligibility, service/database plan availability, GitHub publication and Loom recording are separate manual submission tasks. Never commit keys or substitute an unverified hosting/GitHub URL for a completed deliverable.
