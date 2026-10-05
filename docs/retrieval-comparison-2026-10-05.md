# Retrieval and rendering comparison — 2026-10-05

## Scope

Four live research runs: Barcelona vegetarian lunch below EUR 25 per dish, and Tokyo meat/fish dinner below JPY 3,000 per dish, once in each mode. Each conversation then received an evidence-only follow-up. Exactly two paid grounding requests were made, with one plain formatting attempt each. No paid rerun was performed.

## Live results before the final replay repairs

These measurements describe the first live implementation, not a claim that the final changes achieved these same results. Duration includes client polling; token counts are provider-reported usage, not invoice amounts.

| Request | Seconds | Search queries | Extract requests | URLs attempted/context outcomes | Model attempts | Cache hits | Input/output tokens | Restaurants/dishes/priced dishes |
|---|---:|---:|---:|---:|---:|---:|---|---|
| Barcelona / standard | 100.5 | 9 | 9 | 20 | 8 | 1 | 108850 / 9990 | 3/6/0 |
| Tokyo / standard | 101.9 | 9 | 9 | 20 | 7 | 0 | 195805 / 7886 | 0/0/0 |
| Barcelona / google_grounded | 24.4 | 6 | 0 | 0 | 3 | 0 | 8154 / 4391 | 3/0/0 |
| Tokyo / google_grounded | 24.4 | 6 | 0 | 0 | 3 | 0 | 7610 / 4757 | 0/0/0 |

All four evidence-only follow-ups used one plain model attempt and zero searches or URL fetches. Grounded query counts come from provider metadata; they are not extract HTTP requests.

## Causes and changes

- **Grounded formatting:** the prose contained useful dishes and prices, but presentation differences in literal quotes and mismatched attribution rejected structured cards. Formatting now maps presentation-only differences back to original text and includes a deterministic parser for explicit numbered restaurant headings and priced menu bullets. Unknown layouts retain readable prose. Unknown sources, invented prices and unrelated numeric substrings remain rejected.
- **Standard price recovery:** Aguaribay’s priced row existed in retrieved text, but generated flattened citations were not literal passages. Exact, dish-named currency-bearing rows now recover supported prices, including portion text. Bare numeric rows require a supported currency in the same full menu.
- **Retrieval budget:** initial menu and review retrieval consumed the URL allowance before targeted repairs. Initial source selection now reserves capacity for selected-dish price recovery; limits and deadlines were not increased. Restaurants still execute sequentially. No latency improvement from this allocation change has been established by a new live run.
- **Snippet reuse:** a targeted price snippet from an already-seen URL could be discarded behind its earlier generic snippet. Deduplication now uses URL and content hash, preserving new price evidence while dropping exact repeats.
- **Passage selection:** menu passages now use BM25 with price-row signals and selected-dish queries rather than document-order keyword windows. Full source snapshots remain available for citation validation. URL ranking is heuristic, not a learned reranker.
- **Tokyo suitability:** some candidates were over budget; a mackerel description was not recognized by the meat/fish vocabulary. Fish coverage has been extended, while explicitly morning-only offers are rejected for dinner. This does not establish an acceptable live Tokyo standard shortlist.
- **Failure reporting:** extraction reasons are retained; trace counters separate search calls, extraction requests, URL attempts, model attempts, cache hits and stage durations. Missing selected-dish prices trigger bounded recovery instead of a document-wide currency-count threshold.

## Recorded-response replay after repairs

| Recorded grounded answer | Restaurants | Dishes | Displayable currency-labelled prices |
|---|---:|---:|---:|
| Barcelona | 3 | 9 | 4 |
| Tokyo | 3 | 8 | 8 |

Barcelona has three delivery-attributed amounts and two “included in set / under EUR 16” amounts excluded from exact dish-price cards. Upper budget bounds are not listed dish prices. The original grounded prose remains available for source inspection. Tokyo price ranges are consistent with the recorded answer, but were not independently validated against every original webpage. Replay establishes formatting and attribution consistency, not broad retrieval accuracy or final live performance. Existing saved grounded answers are not automatically converted with billable calls.

## Sampled source inspection

The [Aguaribay menu listing on Guidavera](https://guidavera.com/spain/barcelona/restaurants/aguaribay) contains Koftas at EUR 8 for three pieces and EUR 11.50 for five. The recorded price fixture now recovers EUR 8 with the portion passage intact. This is third-party listed guidance, not restaurant-confirmed current pricing.

The grounded Barcelona metadata attributes several Teresa Carles prices to Uber Eats, which is unsuitable as regular dine-in price evidence. The formatter suppresses those amounts in cards and the research instruction now excludes delivery prices. The sampled Torigin and TheFork pages were not extractable during inspection; their prices are not described as independently verified.

The previous claim of “49 tool calls in 132 seconds” could not be verified as API usage. Two stored runs with 49 progress events recorded nine searches each, 16/11 URL attempts and seven/eight model attempts, with trace durations about 108/96 seconds. A progress event is not an API request.

## Verification

- Backend/frontend Go test suites with isolated PostgreSQL integration schemas.
- Linux Go race suite in the existing Go 1.26.5 Docker image.
- Go vet and golangci-lint 2.14.0 (configured govet, staticcheck and unused): pass.
- Node Markdown fixtures: fenced Markdown, headings, lists, emphasis, tables, escaped HTML, unsafe links and unknown references.
- Grounded fixtures: unknown sources, invented prices, currency, preserved paid prose at exhausted budget, delivery amounts and recorded layout replay.
- Price/IR fixtures: late menu rows retained by BM25, adjacent descriptions, accent aliases, older official menu ranking, exact numeric rows and portion-price recovery.
- Owner-scoped pagination: 20/3 run pages, stable cursors, invalid cursor rejection, foreign-owner rejection and retention.
- Playwright: both live routes, evidence-only follow-ups, previous-run expansion and reload; final recorded replay checks both layouts, lazy historical events, Markdown rendering and historical source isolation.

## Remaining gaps

Standard retrieval did not produce adequate price coverage in this live comparison, and Tokyo returned no usable standard shortlist. The final retrieval allocation and matching repairs were tested with fixtures, not another full live comparison. JavaScript-heavy, blocked and poorly transcribed menu sources remain gaps; unsupported dish prices still show **Price unavailable**. Provider-attributed grounding can select an unsuitable price source and does not independently establish dietary safety or branch accuracy.

Rendering, history and attribution checks are verified. A production-quality recommendation claim, universal price completeness, and an improved live latency claim would be unsupported. Render deployment is outside this change.
