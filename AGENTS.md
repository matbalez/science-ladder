# Science Ladder

Implement the payment-free v0.2 product and architecture in docs/specs. Current user decisions override those documents: all first-party code is MIT; public web/API run on Fly.io; separately isolated verification infrastructure is allowed; public browsing and invited, capped creation/submission; no payments or billing in the MVP. Never represent seeded submissions or internal tests as independent security review or pilot completion.

Go module: github.com/matbalez/science-ladder. Next.js app: web/. Public REST API: /v1. State is PostgreSQL; large immutable artifacts live in S3-compatible private storage. Idempotent state changes; exact GitHub commits; signed typed receipts; integer ticks; fail closed. No production validator execution in the application process. No fake results, fallback scientific scores, fabricated sources, or hidden failure states.

Keep private keys, API credentials, and local configuration out of source and logs. The task-level .env.local is outside this repository and must never be copied into it. Ship .env.example with names and safe empty values only.

Coordinate file ownership between agents. Protocol agent owns pkg/protocol, protocol/, prompts/, cmd/sl, cmd/runnerd, internal/runner. Backend agent owns internal/platform, internal/storage, cmd/api, cmd/worker, migrations, docs/openapi.*. Frontend agent owns web/. Root owns deployment, docs, integration, release, and final verification. Do not modify other agents' files without coordinating. Root owns go.mod/go.sum; communicate dependency requirements.

Use relevant tests for protocol parsing, malicious artifacts, ordering, concurrency, authorization, persistence, and runner fail-closed behavior. Preserve real data through migrations. Document remaining deployment and external launch gates candidly.
