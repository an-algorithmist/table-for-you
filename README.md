# Table for You

Table for You is an agentic chat application that helps travellers find restaurants for breakfast, lunch or dinner. It researches public menus and customer reviews, translates menu information into English, and recommends dishes with prices, dietary evidence and links to the supporting sources.

The application accepts cities and countries worldwide, with European cities as the initial focus. It combines conversational follow-ups, targeted web research and source validation to build a restaurant shortlist tailored to the traveller.

Example request:

> Find vegetarian lunch in Barcelona, Spain, under EUR 25 per dish. Show recommended dishes, brief descriptions, prices, restaurant links and menu links.

## Notable features

- **Conversational requirements:** accepts a city, meal, dietary preference, ingredient exclusions, food preferences such as chicken, cuisine, area and optional budget. Asks for missing or consequentially ambiguous information before restaurant research.
- **Dietary preferences:** supports vegetarian, vegan, non-vegetarian, gluten-free and ingredient exclusions such as onion or garlic. Distinguishes vegetarian dishes from an exclusively vegetarian venue.
- **Menu research and translation:** retrieves menu pages, follows relevant menu links and extracts original dish names, English descriptions, listed amounts and currency. A bounded Gemini transcription fallback can read supported public menu PDFs or images.
- **Source-backed recommendations:** links restaurant/menu pages and lets users inspect the passages supporting menu, price and dietary claims. Venue-wide evidence can produce a **Pure veg** or **Vegan venue** badge.
- **Review context:** searches positive, negative and preference-specific review passages, then paraphrases supported observations. Missing negative evidence is reported rather than invented.
- **Visible research trail:** Server-Sent Events (SSE) expose research stages, source retrieval, cache reuse and failures. These are execution events, not private model reasoning.
- **Price transparency:** shows source-listed prices with ISO currency where established. If an exact price cannot be extracted, the UI shows a supported per-person planning estimate when available, or **Price unavailable**. Estimates do not become exact prices for unpriced dishes.
- **Persistent chat and caching:** PostgreSQL stores browser-scoped conversations, research results, source snapshots and caches. Previous research stays accessible in collapsible sections, with owner-scoped pagination and separate source/trail maps for each run. Follow-up explanations can reuse saved evidence without new web searches.
- **Free-tier-compatible standard mode:** the default Gemini Flash-Lite + Tavily route can operate within their available free allowances, without Google Search grounding. Free usage depends on provider eligibility, account billing configuration, rate limits and remaining credits; the application does not enforce free-tier billing. Google lists free-tier input/output for Gemini 3.1 Flash-Lite, and Tavily provides a monthly free-credit allowance. See [Gemini pricing](https://ai.google.dev/gemini-api/docs/pricing) and [Tavily credits](https://docs.tavily.com/documentation/api-credits).
- **Optional Google-grounded mode:** a composer toggle enables Gemini with Google Search grounding and URL context after an explicit cost warning. This provides an alternative retrieval path that may improve source coverage and answer completeness. Results depend on the available sources. Grounded requests can incur model/search charges; no automatic paid retry or fallback is performed.
- **Bounded research:** request deadlines, call caps, daily admission quotas and cancellation constrain work. Provider credentials stay on the backend.

## Directory structure

```text
table-for-you/
├── README.md
├── AGENTS.md
├── .env.example                   # configuration template; no credentials
├── .gitignore
├── .golangci.yml
├── Makefile
├── go.work                        # backend/frontend Go workspace
├── go.work.sum
├── docker-compose.yml             # local PostgreSQL
├── render.yaml                    # hosting blueprint
├── backend/
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/
│   │   ├── server/main.go          # configuration, wiring, startup and shutdown
│   │   └── providercheck/main.go   # explicit provider diagnostic; may consume quota
│   ├── config/
│   │   ├── countries.yaml         # selected currency/search/branch defaults
│   │   └── embed.go
│   ├── migrations/                # embedded, versioned PostgreSQL SQL migrations
│   └── internal/
│       ├── config/                # environment loading and country settings
│       ├── domain/                # chat, research, evidence and grounding contracts
│       ├── httpapi/               # sessions, handlers, request admission and SSE
│       ├── pipeline/              # intent, discovery, retrieval and extraction stages
│       ├── llm/                   # Gemini adapter, retry policy and call budget
│       │   └── prompts/           # embedded Markdown instructions
│       ├── tools/                 # Tavily search/extraction and menu-file fetching
│       ├── providers/             # shared HTTP errors and URL safety checks
│       ├── evidence/              # deterministic citation, dietary and price checks
│       ├── storage/postgres/      # repositories, quotas, history and caches
│       └── testutil/              # isolated PostgreSQL test schemas
└── frontend/
    ├── go.mod
    ├── embed.go                   # assets embedded into the backend binary
    └── assets/
        ├── index.html
        ├── markdown.js               # shared safe Markdown rendering
        ├── app.js
        └── styles.css
```

The backend and frontend are separate Go modules connected by `go.work` and a local module replacement. The small frontend module packages static assets for the single backend executable. No Node.js build step is required to run the application.

## High-level design

The standard route is a **retrieval-augmented generation (RAG) pipeline over live web evidence**. Retrieved menu/review text is supplied to Gemini for structured extraction and translation, then checked deterministically before display. It uses PostgreSQL caches and source snapshots, not embeddings or a vector database.

```mermaid
flowchart TD
    UI[Agentic chat interface] --> API[Go HTTP API: session, ownership and admission]
    API --> INT[Interpret request and resolve follow-up context]
    INT --> ACTION{Required next action}
    ACTION -->|Missing details| ASK[Ask a clarification question]
    ASK --> UI
    ACTION -->|Explain existing result| SAVED[Read matching stored evidence]
    SAVED --> ANSWER[Generate an evidence-based explanation]
    ACTION -->|Research| MODE{Research mode}

    subgraph STANDARD[Standard web RAG pipeline]
        DISC[Tavily discovery search] --> SHORT[Shortlist up to 3 restaurant branches]
        SHORT --> FETCH[Retrieve menus, prices and contextual reviews]
        FETCH --> IR[URL reranking and BM25 menu passages]
        IR --> EXTRACT[Gemini: structured extraction and English translation]
        EXTRACT --> CHECK[Validate citations, branch, diet, meal and price]
        CHECK --> GAPS{Consequential evidence gaps and budget remaining?}
        GAPS -->|Yes: at most 2 rounds| TARGET[Targeted search and extraction]
        TARGET --> CHECK
        GAPS -->|No| CARDS[Partition supported choices, possible options and exclusions]
    end

    MODE -->|Default| DISC
    MODE -->|Explicit cost acknowledgement| GROUND[Gemini with Google Search and URL context]
    GROUND --> ATTR[Grounded prose, provider citations and search metadata]
    CARDS --> STORE[Persist result, usage and source evidence]
    ATTR --> FORMAT[One plain formatting attempt: attributed shortlist]
    FORMAT --> STORE
    ANSWER --> STORE
    STORE --> UI

    DB[(PostgreSQL: history, caches, snapshots, quotas and trace events)] -.-> API
    DB -.-> SAVED
    DB -.-> FETCH
    DB -.-> EXTRACT
    STORE --> DB
    API -.-> SSE[SSE: persisted research progress and completion]
    CHECK -.-> SSE
    GROUND -.-> SSE
    SSE --> UI
```

### Standard pipeline steps

1. **Interpret:** combine the new message with bounded conversation context, preserve explicit restrictions and identify whether research, clarification or an existing-result explanation is needed.
2. **Discover and shortlist:** search for relevant restaurants, retain names supported by discovery passages, deduplicate candidates and check available links.
3. **Retrieve and rank:** research each branch sequentially, with explicit discovery, menu, price and review purposes. Rank URLs by restaurant identity, location, official domain and menu/price signals; use BM25-ranked literal menu passages with adjacent rows and descriptions. Prefer official menus, follow relevant linked menus, search specifically for missing prices, and collect positive/negative/context-specific review evidence. Third-party menu text or snippets can provide partial evidence when full extraction is unavailable.
4. **Extract and translate:** pass restaurant-scoped source text to Gemini using a structured response schema. Extract dishes, descriptions, amounts, currency, venue labels and short review summaries with exact supporting citations.
5. **Validate:** check that cited passages exist, evidence belongs to the restaurant branch, dietary/meal requirements are supported, and amounts/currency have an acceptable source basis. Compare supported dish prices against a same-currency per-dish budget.
6. **Repair gaps:** target missing prices on selected dishes, ingredients or review criticism, then extract and validate again while call/time budgets allow.
7. **Recommend and persist:** return source-supported choices and possible options with consequential gaps. Strictly unsuitable or unverified restricted dishes are kept out of the shortlist. Save results, usage and trace events for history and follow-ups.

### Google-grounded route

Interpretation, clarification, ownership and admission are shared with standard mode. When enabled for research, the backend makes one grounded provider request with Google Search and URL context; Google controls search fan-out inside that request. One plain, non-grounded model attempt formats the answer into up to four restaurant cards with three dishes each. Literal answer passages and provider source references are checked for consistency; formatting never repeats the paid search. The UI retains the original answer, citations and Google Search Suggestions if formatting fails.

**Grounded prose is provider-attributed output, not a result independently validated by the standard menu-extraction pipeline.** Structured cards enforce four restaurants and three dishes per restaurant; the query target in the research prompt is an instruction, not an enforced provider search limit. Google documents that a single grounded request can produce multiple search queries. Consult [Google Search grounding documentation](https://ai.google.dev/gemini-api/docs/google-search/) and [Gemini pricing](https://ai.google.dev/gemini-api/docs/pricing) for availability and billing.

## Tools and technologies

| Component | Technology | Role |
|---|---|---|
| Backend | Go 1.26+, `net/http`, `log/slog` | HTTP service, orchestration and logging |
| Frontend | HTML, CSS, vanilla JavaScript | Chat, recommendation cards, citations and trace rendering |
| Model | Gemini through `google.golang.org/genai` | Intent, translation, structured extraction and optional menu transcription |
| Web retrieval | Tavily Search and Extract APIs | Restaurant discovery, menu/review searches and text extraction |
| Optional grounded retrieval | Gemini Google Search grounding + URL context | Alternative search-grounded answer with provider attribution |
| Persistence | PostgreSQL 17, `pgx/v5` | Sessions, history, run state, quotas, evidence snapshots and caches |
| Live progress | SSE | Persisted event replay and completion updates |
| Packaging | `go:embed`, Go workspace | Frontend, prompts, country resource and migrations bundled with the binary |
| Local database | Docker Compose | PostgreSQL container and persistent volume |
| Validation | Go tests, `go vet`, `gofmt`, `golangci-lint` | Deterministic/fake-provider and PostgreSQL integration checks |

## Assumptions

- **Interaction:** queries and responses are English; source menus may be translated while retaining original dish names.
- **Requirements:** a city, country and meal are required; budget, cuisine and area are optional. Vegetarian does not imply no onion/garlic or a specific egg policy.
- **Prices and currency:** official, third-party and older listed prices serve as planning guidance. Source currency is preferred; a verified location may provide a labelled local-currency assumption. No currency conversion is performed.
- **Budgets and estimates:** exact budget checks use same-currency per-dish prices. Supported per-person estimates assume one dish, with a 25% allowance for derived ranges; drinks, sides and fees are extra.
- **Dietary evidence:** strict restrictions need explicit source support; non-vegetarian matches require meat or fish. Venue badges require venue-wide evidence.
- **Sources and availability:** reviews are a selected sample, and menu listings describe published offerings. Standard extraction excludes delivery-platform menus.
- **Caching and history:** searches/reviews are cached for six hours and menus for 24 hours. Guest history is browser-scoped and expires after seven days of inactivity.

## Limitations

### Geographic coverage

[`backend/config/countries.yaml`](backend/config/countries.yaml) provides selected currency defaults, local menu-search terms and city/branch slugs; **it is not an allowlist**. Unlisted locations use generic searches and source/user-supplied currency. Local-language and branch validation rules currently cover selected destinations, so retrieval depth varies by location.

Coverage can be extended through country defaults and relevant Go language/identity rules. The configuration uses JSON-compatible YAML 1.2 and is embedded in the binary, so updates require a rebuild.

### Web retrieval and source coverage

- Tavily may return incomplete snippets or fail to extract restricted, bot-protected or timed-out pages. A successful HTTP response can contain both extracted and failed URLs. See the [Tavily Extract API](https://docs.tavily.com/documentation/api-reference/endpoint/extract).
- Interactive menus, embedded viewers, PDFs, images and multi-column layouts can lose dish/price associations during extraction. The application has bounded PDF/image transcription, but no interactive browser-scraping fallback.
- Source text is capped at 60,000 bytes per document. Filtering and extraction can omit prices visible in a browser. The UI displays a supported estimate when available, otherwise **Price unavailable**.
- Review coverage depends on accessible passages; positive, negative and preference-specific evidence may be partial.
- Multiple searches/extractions consume credits per request. Advanced menu search and extraction use more credits than basic search. See [Tavily credits](https://docs.tavily.com/documentation/api-credits).

### Recommendation and execution limits

- **Standard mode:** up to **3 restaurant branches**, **4 extracted dishes** and **8 review observations per branch**; the UI initially shows two dishes. Evidence filtering may reduce the final shortlist.
- **Research budget:** up to 14 uncached searches, 20 source fetch attempts, 2 visual transcriptions and 2 gap-repair rounds. Model attempts default to 9, configurable up to 20; runs have a 180-second deadline.
- **Grounded mode:** the prompt targets up to 4 restaurants, 3 dishes each and 6 search queries. Google controls search fan-out; these are prompt targets rather than enforced caps.
- **Admission:** two active runs globally, one per conversation. Daily defaults are 20 turns per browser owner/100 globally, with separate grounded limits of 3/10. Accepted clarification and explanation turns count toward quotas.

### Model, dietary and pricing validation

- Model translation, extraction and visual transcription can misinterpret source material. Standard mode checks supporting passages; grounded prose uses provider citations without the same independent dish-level validation.
- Ingredient omissions do not establish absence, and cross-contamination is not assessed. Published menus do not confirm stock or availability for a future visit.
- Listed prices are not checked for freshness. Estimates provide planning ranges rather than exact dish prices or guaranteed per-person budget matches.

### Access, providers and scope

- History uses browser ownership rather than account authentication; losing the session cookie can prevent recovery.
- Free usage depends on provider quotas and account billing. Application call limits do not enforce a monetary ceiling or provider-wide RPM/TPM throttle. Grounded work already started may be billed even after cancellation.
- Provider interfaces are separable; Gemini and Tavily are the implemented adapters. An OpenAI adapter is not included.
- The prototype supports restaurant research and recommendations; bookings, payments and exhaustive worldwide indexing are outside its scope.

## Local setup

### Prerequisites

- Go 1.26 or a compatible newer version.
- Docker Desktop or Docker Engine with Compose and Linux container support.
- A Gemini API key with access to the configured models.
- A Tavily API key and sufficient available credits for standard mode.

A ChatGPT/Gemini chat subscription is not a substitute for these API credentials. Provider accounts determine free-tier eligibility and billing. Run the following commands from the `table-for-you` repository root.

### 1. Start PostgreSQL

```powershell
docker compose up -d postgres
docker compose ps
```

The local database listens on `127.0.0.1:5432`. Compose creates a persistent volume and the development database/user `nebula`. The supplied password is for this local development setup only.

### 2. Configure the backend

Create the private environment file only if it does not already exist:

```powershell
if (!(Test-Path backend/.env)) {
    Copy-Item .env.example backend/.env
}
```

For Linux/macOS:

```bash
[ -f backend/.env ] || cp .env.example backend/.env
```

Edit `backend/.env` and set `GEMINI_API_KEY` and `TAVILY_API_KEY`. The template includes the local `DATABASE_URL`. Keep real credentials out of `.env.example` and Git. Process environment variables take precedence over the file. `NEBULA_ENV_FILE` can select a different private env file.

### 3. Build, migrate and run

Windows PowerShell:

```powershell
Set-Location backend
go build -o bin/nebula.exe ./cmd/server
./bin/nebula.exe migrate
./bin/nebula.exe serve
```

Linux/macOS:

```bash
cd backend
go build -o bin/nebula ./cmd/server
./bin/nebula migrate
./bin/nebula serve
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080). Local mode uses a browser guest session. Leave the Google-grounded toggle off for standard research. To try optional grounding, enable it and acknowledge the cost warning; model/tool access must be available for the API project.

Frontend assets and SQL migrations are bundled with the executable. The binary still needs runtime configuration and a running PostgreSQL database. Check `/healthz` for service health and `/readyz` for database readiness; neither endpoint performs model inference.

### Configuration reference

| Variable | Default / purpose |
|---|---|
| `DATABASE_URL` | Required PostgreSQL connection; local value provided in the template |
| `GEMINI_API_KEY`, `TAVILY_API_KEY` | Server-only API credentials |
| `MODEL_PROVIDER` | `gemini`; unsupported values fail explicitly |
| `MODEL_NAME` | `gemini-3.1-flash-lite` |
| `APP_ENV`, `PORT` | `local`, `8080`; local mode binds to loopback |
| `REVIEWER_ACCESS_CODE` | Required outside local mode; shared reviewer gate, not account authentication |
| `NEBULA_ENV_FILE` | Optional private env-file path; default `.env` in the backend working directory |
| `MAX_MODEL_CALLS_PER_RUN` | 9; maximum configurable value 20 |
| `MAX_DAILY_RUNS_PER_OWNER`, `MAX_DAILY_RUNS_GLOBAL` | 20, 100 |
| `ENABLE_GOOGLE_GROUNDED` | `true` exposes the optional route; the UI toggle is off by default |
| `GROUNDED_MODEL_NAME` | `gemini-3.8-flash` |
| `MAX_DAILY_GROUNDED_PER_OWNER`, `MAX_DAILY_GROUNDED_GLOBAL` | 3, 10 |
| `GROUNDED_MAX_OUTPUT_TOKENS` | 8192 maximum |

The grounding switch resets on reload and new conversation. Grounded admission counters survive history deletion; global limits reduce the effect of creating fresh browser sessions. Cancellation cannot guarantee that provider work already started will not be billed.

## Tests and development commands

From the repository root:

```powershell
go test ./backend/... ./frontend/...
go vet ./backend/... ./frontend/...
golangci-lint run ./backend/... ./frontend/...
```

The tests use fake models/search and local HTTP fixtures. They do not call real Gemini or Tavily. PostgreSQL integration tests require a separate test database; create it once with the local container:

```powershell
docker exec nebulaiq-postgres-1 psql -U nebula -d nebula -c "CREATE DATABASE nebula_test"
$env:TEST_DATABASE_URL = "postgres://nebula:nebula_local_only@127.0.0.1:5432/nebula_test?sslmode=disable"
go test -count=1 ./backend/... ./frontend/...
go test -race -count=1 ./backend/... ./frontend/...
```

Each integration test creates and drops an isolated schema. Without `TEST_DATABASE_URL`, database tests are skipped. Windows race tests require a compatible C compiler; Linux Go containers provide an alternative. The lint configuration enables `govet`, `staticcheck` and `unused`.

Make targets are available for `build`, `run`, `migrate`, `db-up`, `test`, `vet`, `race`, `lint` and `fmt`. The `providercheck` command makes an explicit real provider diagnostic request and can consume quota; it is not part of the offline test suite.
