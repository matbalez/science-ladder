# Science Ladder web application

Next.js 16 / React 19 / TypeScript, with local fonts and no client-side credentials. All UI data comes from the Go API. No production demonstration fixtures or fallback scientific scores exist.

## Run locally

```sh
npm ci
npm run dev
```

Open `http://localhost:3000`. Run the Go API on `http://127.0.0.1:8080`, or set `API_URL` to its private upstream before starting Next.js. Use `localhost` for the development browser origin; current Next.js protects its development resources against other origins.

## Production

```sh
API_URL=http://science-ladder-api.internal:8080 npm run build
npm start
```

`API_URL` is resolved into the rewrite configuration **at build time**. Rebuild when it changes. Next.js produces a standalone deployment. `/v1/*` and `/.well-known/science-ladder-keys.json` proxy to the Go API with same-origin cookies. No `NEXT_PUBLIC_*` API credential is needed or supported. GitHub OAuth redirects through the public web origin. GitHub App installation is available from Account.

Root-level deployment configuration owns Fly.io, TLS, domain, API secrets, database, and object storage. The browser never handles AI provider keys or runner credentials.

## Routes

- `/`: current public challenge activity, search, domain filters, sort, and milestone counts.
- `/challenges/[slug]`: primary evidence, immutable evaluation, exact score display, frontier graph, construction preview, milestone ladder, submission intents, flags, public history, and exports.
- `/create`: canonical versioned Scout prompt prefill, YAML validation/import, evidence resolution, accountable adoption, exact repository commit, asynchronous preflight, contract lock, and publication. Session storage preserves the current local draft. Account candidate links resume from the authoritative API.
- `/submissions/[id]`: orthogonal processing/validation/publication state, attribution, runs, milestone claims, receipt envelopes, and optional artifact publication.
- `/account`: GitHub sign-in, invitations, account quota, repository access, CLI session approval, and private activity.
- `/authorize`: CLI verification alias used by the API.
- `/review`: role-gated flags/reviews queue and documented editorial decisions.
- `/docs`: usable local CLI workflow and live, paginated public checkpoint evidence. Host-reported quorum is explicitly distinguished from independent client verification.

API contracts are documented in `../docs/openapi-contract.md`. Missing credentials, runner capacity, quota, unavailable services, and unfinished work remain explicit; the client cannot bypass backend capability or conformance gates.

The artifact viewer reads only bounded JSON data. It accepts direct point/center/coordinate/circle/vertex arrays or a canonical bundle with base64 JSON files. It renders 2D points/circles and rotatable 3D coordinate projections, never source HTML/SVG or executable content. Other artifact kinds remain downloadable. The preview is not scientific verification.

## Verify

```sh
npm run typecheck
npm test
npm run build
npm run test:e2e
```

Browser tests require the web server at `http://localhost:3000`; `TEST_BASE_URL` overrides this. On this development host tests use a fresh headless Chrome context from `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`. Set `PLAYWRIGHT_CHROME_PATH` to the installed browser executable elsewhere. Test fixtures are explicitly labeled TEST ONLY and confined to `tests/`; tests intercept their own browser network requests and do not publish anything to the platform.

Tests cover exact decimal rendering beyond JavaScript’s safe-integer range, chart normalization, unsafe URL rejection, mobile overflow, canonical source evidence, malformed YAML, versioned Scout prefill, exact-commit inspection followed by acceptance, and visible service failures. Screenshots are written to ignored `test-results/`.

```sh
npm run format
```
