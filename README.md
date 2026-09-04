# Science Ladder

Open computational challenges for human–agent teams. Publish a scientific question with an immutable checker, submit reproducible artifacts, and advance a shared frontier through independently checked results.

**Invitation preview.** The public application is designed for browsing; creation and submissions require a GitHub invitation and validation capacity. A controlled demonstration is labelled separately from an independently reviewed competition. The application refuses to fabricate scores when verification infrastructure is unavailable.

- Website: [science-ladder.fly.dev](https://science-ladder.fly.dev)
- Product and architecture: [v0.2 product specification](docs/specs/product-v0.2.md), [technical architecture](docs/specs/architecture-v0.2.md)
- Current decisions: [MIT licensing, Fly.io hosting, and deployment modes](docs/decisions.md)
- API: [OpenAPI document](docs/openapi.json), [frontend contract](docs/openapi-contract.md)
- Persistence: [database, immutable objects, and recovery](docs/persistence.md)

## How a challenge works

1. A creator uses the versioned Challenge Scout prompt to identify a well-supported, useful computational question, then adopts and reviews the candidate.
2. The platform archives an exact GitHub commit. Separate quarantine workers build the checker twice offline and run fixtures, adversarial probes, and baseline checks. Scientific review is recorded separately from executable conformance.
3. An approved version locks its rules, metric, resources, deadline, and milestone thresholds. Changing those terms requires a new version.
4. A solver submits a GitHub artifact at an exact commit. The server constructs canonical input and a read-only disk before assigning acceptance order and reserving capacity.
5. Separate verification hosts run the locked checker. Potential frontier or milestone advances require confirmation on a different physical host group. Exact integer ticks determine outcomes; receipt order determines the first qualifying claim for every crossed milestone.
6. Public advances publish their reproducible artifacts. Losing submissions remain private unless their owner chooses publication. Signed receipts and witnessed audit checkpoints preserve the history.

There are no payments, billing records, or monetary rewards in this implementation.

## Components

| Component | Location | Purpose |
|---|---|---|
| Next.js application | `web/` | Explore, create, review, submit, and inspect results |
| Go API and worker | `cmd/api`, `cmd/worker`, `internal/platform` | Identity, transactional state, queue, intake, and adjudication |
| Shared protocol | `pkg/protocol`, `protocol/` | Canonical formats, signatures, exact scores, artifact safety |
| Solver CLI | `cmd/sl` | Scout, scaffold, check, submit, and verify |
| Verifier and quarantine | `cmd/runnerd`, `internal/runner` | Isolated preparation, fixture testing, and Firecracker execution |
| Independent witness | `cmd/witness`, `internal/audit` | Verify and retain an append-only checkpoint chain |
| Signing | `internal/signing` | P-256 KMS adapter and explicit local demonstration keys |
| Deployment | `deploy/` | Pinned container builds and Fly.io application definitions |

## Local development

Install Go 1.27.1, Node.js 24, Python 3, and Docker with Compose. Local services bind only to loopback and retain data in named Docker volumes.

```sh
python3 scripts/dev-services.py up
python3 scripts/dev-services.py exec go run ./cmd/api migrate
python3 scripts/dev-services.py exec go run ./cmd/api
```

In another terminal:

```sh
python3 scripts/dev-services.py exec go run ./cmd/worker
```

Then start the web application:

```sh
cd web
npm ci
npm run dev
```

Open [localhost:3000](http://localhost:3000). Local service credentials are generated in ignored `work/local-services.env`; they are not production credentials. Configure GitHub and OpenAI credentials separately using the names in `.env.example`. The application exposes missing capabilities rather than offering fake authentication or fake scientific review.

```sh
python3 scripts/dev-services.py exec go test -race ./...
go vet ./...
go build -o bin/sl ./cmd/sl
cd web
npm test
npm run build
```

Database tests use `TEST_DATABASE_URL` and create isolated test schemas. The local script supplies that variable without displaying its value. Stop services with `python3 scripts/dev-services.py stop`; persistent volumes are retained.

## Deployment and trust

See [deployment](docs/deployment.md), [witness operation](docs/witnesses.md), and [release evidence](docs/release-gates.md). Public web and API containers run on Fly.io, backed by PostgreSQL and private S3-compatible storage. Verification runs belong on separately enrolled Linux amd64 hosts with the required isolation profile.

`controlled-demo` labels a demonstration using real configured runs. `production` also requires KMS custody, externally pinned root-signed key history, independent witness quorum, and a root-signed release attestation. Deployment alone does not supply independent security review or an external pilot.

## License

All first-party code, protocol definitions, prompts, and documentation are [MIT licensed](LICENSE). Dependencies and submitted datasets or artifacts retain their own licenses. Challenge creators must declare redistribution rights for every bundled input.
