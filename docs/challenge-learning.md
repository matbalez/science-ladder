# Challenge learning guide

Each public challenge page includes an inline, anonymous conversation below its visualization. Suggested questions cover an introduction, the current view, and the significance of improvement. Follow-ups retain recent context. Participate remains the solving and submission entry point.

## Grounding

The API loads an allowlist from the exact current public challenge version: identity, frozen source, metric, milestones, public frontier, education, scientific rationale, evidence and evaluation rules. Drafts and withdrawn versions cannot be used. Private reviews, provider payloads, hidden suites and private artifacts are excluded.

The browser also reports explicit visualization fields: the selected matrix product and its arithmetic, edited triangle coordinates and selected bottleneck, visible pulse correlations, and selected public frontier results. This is reported display state, not a verification certificate. It is captured when a question is sent; changes during an answer apply to the next question. There is no screenshot or arbitrary DOM collection. Challenges without a custom interactive visualization still have the version-grounded guide.

The guide distinguishes proven optima from dated references, exact verification from visual approximations, and demonstrated mathematical progress from speculative applications. It can cite supplied source links but does not browse their contents. It has no execution, submission, mutation or browsing tools. Answers are AI explanations and can be wrong; they never create receipts or leaderboard entries. Record text, browser state and browser-supplied conversation history are treated as untrusted evidence, not instructions.

## API and storage

`POST /v1/challenges/{slug}/ask` takes `versionId`, alternating `messages` ending in a question, and optional `view`. See [OpenAPI](openapi.json). It streams answer text via SSE (`start`, `delta`, `done`, `error`). Incomplete streams remain labeled as partial answers and are excluded from subsequent model history. Stop or navigation aborts the upstream request.

The server reuses its existing `OPENAI_API_KEY`; the browser never receives it. `OPENAI_TUTOR_MODEL` defaults to `gpt-6-astra`, with low reasoning effort and a 3,000 output-token ceiling. The Responses API uses `store: false`. Questions and context are still processed by OpenAI under the API account's data controls; this setting is not a claim of zero provider retention.

The platform does not store or log questions or answers. Up to 20 completed exchanges are kept in browser session storage, scoped to the immutable challenge version; Clear conversation removes them. At most seven earlier exchanges are sent with a question. The page discloses the tab storage and OpenAI processing.

Migration 015 stores admission metadata only: a purpose-separated HMAC visitor identity, request time, lease expiration, and finish time. Anonymous identity is an opaque, signed, HTTP-only, same-site cookie; signed-in accounts use a hashed user ID. Raw IPs are not stored. Metadata older than two days is removed on the next request.

## Public capacity

Database admission is serialized across all API replicas: 30 requests per visitor per rolling hour, 1,000 globally per UTC day, and eight active requests. Anonymous cookies are a convenience limit rather than strong identity; the global caps also apply when cookies are reset. Attempts count toward limits even when the provider fails. A 110-second upstream timeout and two-minute lease recover capacity after failures. The web proxy timeout is 120 seconds. These limits are independent of scientific verification capacity and never pause verification jobs.

Tests cover input boundaries, exclusion of private fields, answer-only streaming, interruption handling, signed cookies, cross-request admission, public-version visibility, mobile layout, selected-view context, follow-up history, tab persistence and retry behavior. Paid live checks are performed explicitly at release, not automatically in CI.
