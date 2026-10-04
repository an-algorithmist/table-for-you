# AGENTS.md

> **Note:** `AGENTS.md` is a convention, not a formal standard. Treat everything here as
> guidance. **MUST** means "do this unless there is a strong reason". **SHOULD** means
> "default, deviate if it helps". If you deviate, say why in the commit message or a short
> code comment. The goal is a codebase a senior engineer can read quickly, not rule-following.

## 1. Project context

A chat agent that recommends restaurants to a traveller in any city (Europe first, Barcelona
as the demo). The user gives city, meal (breakfast/lunch/dinner) and dietary needs (veg,
vegan, gluten-free, no onion/garlic, free text). The agent finds restaurants, shortlists
them, reads and translates menus, analyses positive and negative reviews, runs
context-specific review searches, and recommends dishes with prices and links. Every step
is shown to the user as a live trace.

This is an **assignment-grade working system, not production**. Prefer clarity and a
working end-to-end flow over completeness. Do not over-engineer.

Language: user input and output are English only. Menus and reviews may be in any language.

## 2. Stack (defaults)

- Backend: **Go** (1.22 or newer), single module, `cmd/` + `internal/` layout
- HTTP: standard library `net/http` (1.22 routing) or `chi`. Live trace via **Server-Sent Events (SSE)**
- LLM: Gemini via `google.golang.org/genai` (Google Search grounding; URL context if the SDK supports it, see section 5)
- Persistence: **PostgreSQL via an ORM** (GORM by default, see section 6)
- Frontend: minimal chat page (HTML + vanilla JS, embedded with `go:embed`), consuming SSE
- Logging: `log/slog`. Lint/format: `gofmt`, `go vet`, `golangci-lint`
- Tests: standard `testing` package, table-driven
- Local services: Docker Compose for Postgres

## 3. Architecture rules

1. **Layering, one direction only:** `httpapi` -> `pipeline` -> `stages` -> (`llm`, `tools`, `store`).
   Lower packages MUST NOT import higher ones. `stages` know nothing about HTTP.
2. **The pipeline is a fixed sequence of stages**, not a free-roaming agent loop. Each stage
   may call an LLM internally. Per-restaurant work runs as parallel sub-agents (goroutines).
3. **The frontend is a thin renderer.** The orchestrator emits typed trace events on a
   channel; the HTTP layer streams them as SSE. No business logic in handlers or JS.
4. **One stage, one package or file, one job.** A stage has typed input and typed output.
   No package-level mutable state, no `init()` side effects.
5. **Typed contracts.** Data between stages is plain Go structs defined in `internal/domain`.
   LLM output that must be structured is parsed into these structs and validated.
6. **Nothing city- or country-specific in code.** City is an input. Country differences
   (currency, local words for "vegetarian", etc.) live in `config/countries.yaml`.
   Adding a country should be a config change.
7. **Deterministic code for deterministic work** (URL cleanup, price parsing, dedupe,
   sorting). Use the LLM only for judgment, extraction and translation.
8. **Follow-up turns:** the first-prompt flow comes first. Keep a `SessionState` type in the
   design, but do not build follow-up handling until the first flow works end to end.

## 4. Go style and readability

- Follow standard Go conventions (Effective Go, Go Code Review Comments). Keep it idiomatic
  and boring.
- `context.Context` is the first parameter of any function that does I/O or can be cancelled.
  Propagate it everywhere; honour cancellation and timeouts.
- **Accept interfaces, return structs.** Define small interfaces in the package that
  *consumes* them (for example `type MenuFetcher interface { ... }` in `stages`). Do not
  create interfaces with a single implementation "just in case". The exception is the LLM
  client and repositories, which are interfaces so tests can fake them.
- Wire dependencies explicitly in `cmd/server/main.go` through constructors
  (`NewX(deps)`). No globals, no service locators.
- Errors: wrap with `fmt.Errorf("doing x: %w", err)`. Use typed or sentinel errors for
  failures the pipeline must react to (`errors.Is` / `errors.As`). Never ignore an error.
  No `panic` in application code.
- Concurrency: use `errgroup` with a bounded limit for sub-agents. No goroutine without a
  clear owner and a way to stop. No data races; run tests with `-race`.
- Exported identifiers have doc comments. Comments explain **why**, not what. Remove dead
  code and commented-out code.
- Keep packages small and cohesive. If a file passes ~300 lines or mixes two jobs, split it.
- Names describe intent (`ExtractMenuItems`, not `Process`). Avoid stutter
  (`menu.MenuItem` -> `menu.Item`).
- Write code a senior engineer can skim: clear structure, consistent patterns, no cleverness.

## 5. LLM and tool rules

- All model calls go through `internal/llm`. No other package imports
  `google.golang.org/genai`. That package owns retries, timeouts, logging and the budget cap.
- **Budget guard (MUST):** a per-run call counter with a hard cap, configurable via
  environment. A bug must not be able to drain the credit.
- Prompts live in `internal/llm/prompts/*.md` with named placeholders, embedded with
  `go:embed` and rendered via `text/template`. No long prompt strings inside Go files.
- Grounded search returns prose plus source URLs. When structured output is needed, use a
  second plain call with a JSON response schema to convert grounded text into a struct.
  **Verify against the current SDK** whether grounding and a response schema can be
  combined in one call before relying on either pattern.
- Check in the spike whether the Go SDK exposes URL context. If not, fetch pages with
  `net/http` in `internal/tools` and pass extracted text to the model.
- Grounding source URLs may be redirect links. Resolve them in `internal/tools/links.go`
  before showing any link to the user.
- Reviews: summarize and paraphrase themes with source links. Do not store or display long
  verbatim review text.
- **Never invent** dishes, prices, ingredients or URLs. If something cannot be found, say so
  and lower confidence.

## 6. Data and persistence (Postgres + ORM)

- Persistent data MUST live in PostgreSQL and be accessed through the ORM (GORM by default).
  No raw SQL string building in application code. Raw SQL is acceptable only in migrations.
- Connection comes from `DATABASE_URL`.
- Keep **ORM models** (`internal/store/models.go`, table structure, GORM tags) separate from
  **domain types** (`internal/domain`). Map explicitly at the repository boundary. Do not
  leak ORM structs into stages or HTTP handlers.
- Access data through **repositories** (`internal/store/*_repo.go`) behind small interfaces
  (for example `RunRepository`, `RestaurantRepository`). Stages never use `*gorm.DB`
  directly.
- Pass `context.Context` into every repository call. Use transactions for multi-write
  operations, scoped to one clear boundary.
- Migrations: `golang-migrate` or SQL files in `migrations/`. For the assignment,
  `AutoMigrate` at startup is acceptable, but note it in the README as a limitation.
- Suggested tables: `runs`, `trace_events`, `restaurants`, `menu_snapshots`,
  `review_snapshots`, `sessions`. Store fetched menus and reviews with a timestamp and
  source URL so results can be reused, audited and shown in the trace.
- Secrets only in environment variables, never in the repo. Provide `.env.example`.

## 7. Error handling and resilience

- Each stage returns a result or a typed failure. Failures are values the orchestrator
  handles, not surprises.
- One retry with a timeout on network and LLM calls, then degrade gracefully.
- A failed restaurant MUST NOT fail the whole run. Show partial results with an honest
  note (for example "menu could not be read, showing review-based suggestions").
- Menu retrieval fallback ladder: official page, then PDF, then third-party menu snippets,
  then review mentions, then link only.
- Every dish carries a confidence label: `confirmed` (menu or reviews say so), `likely`
  (inferred from dish or cuisine), `ask_staff` (unknown, include the local-language phrase).

## 8. Observability

- Every pipeline step emits a typed trace event (`step_started`, `step_done`, `step_failed`,
  with a readable message and optional details). Events are streamed over SSE and persisted
  to `trace_events`.
- Use `slog` with structured key-value fields. No `fmt.Println` in application code.

## 9. Configuration

- Settings are loaded once in `internal/config` from environment variables into a single
  struct, passed to constructors. No scattered `os.Getenv` calls.
- Country-specific values come from `config/countries.yaml`. Provide a sensible default for
  unknown countries so the system still works.

## 10. Testing

- Aim for few, meaningful tests, not coverage numbers:
  - table-driven unit tests for deterministic helpers (price parsing, link cleanup,
    confidence rules)
  - fake the LLM client and repositories in stage tests
  - one end-to-end smoke test for the Barcelona demo flow, behind a build tag or env flag
    so it does not burn credit on every run
- Never call the real Gemini API from unit tests.

## 11. Scope guardrails (do not build)

Reservations, booking, delivery, payments; maps or geolocation; user accounts and auth;
direct scraping of Google Maps, TripAdvisor or Yelp; OCR of photographed menus; non-English
UI; caching infrastructure beyond the Postgres tables above; production concerns such as
scaling and monitoring. If a task seems to need one of these, stop and ask.

## 12. Working agreements

- Make small, reviewable changes. One concern per commit, with a clear message.
- Before large changes, state the plan in two or three sentences.
- If requirements are ambiguous, ask one focused question rather than guessing.
- Keep `README.md` current. Maintain two sections as you go: **Limitations** and
  **Assumptions**. Add to them whenever a shortcut is taken.
- Do not add a dependency without a one-line justification. Prefer the standard library.
- Do not use an agent framework unless asked.

## 13. Definition of done (per feature)

- Works end to end for the Barcelona demo and one other city with no code change
- `gofmt`, `go vet` and `golangci-lint` clean; `go test -race ./...` passes
- Doc comments on exported identifiers
- Failure path tried at least once manually (for example a menu that fails to load)
- Trace events visible in the UI
- README limitations and assumptions updated