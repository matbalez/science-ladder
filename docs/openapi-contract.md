# REST integration contract

All routes use `/v1`. JSON camelCase, UUID resource IDs, RFC3339 timestamps, exact integer ticks encoded as strings. Errors: `{error:{code,message,details?}}`. Every authenticated mutation requires an `Idempotency-Key` (8–128 characters); retrying the same request returns the same resource, changed content with the same key is rejected. Same-origin browser cookies; scoped bearer tokens for CLI. The application never runs scientific validators.

## Discovery and account

- `GET /healthz`, `GET /readyz` (outside /v1).
- `GET /v1/me` → `{user:null|{id,githubId,login,avatarUrl,role,invited},quotas:{remaining,activeLimit},capabilities:{creation,submission,review},configuration:{githubAuth,scientificReview,officialRunner}}`.
- `GET /v1/auth/github` starts GitHub OAuth; callback `/v1/auth/github/callback`.
- `POST /v1/auth/logout` clears session.
- `GET /v1/challenges?search=&limit=24&cursor=` → `{challenges:Challenge[],nextCursor?}`.
- `GET /v1/challenges/{slug}` → Challenge.
- Challenge: `{id,slug,title,summary,domain,status,reviewStatus,intakeStatus,economicMode:"none",versionId,repository,sourceCommit,createdAt,deadline,metric:{name,direction,units,quantum,baselineTicks},milestones:[{id,label,thresholdTicks,claimedBy?,claimedAt?}],verifiedBest?:{submissionId,scoreTicks},publicFrontier?:{submissionId,scoreTicks},badges:string[],manifest?:object,reviews:object[],submissions:Submission[]}`.
- `GET /v1/dashboard` → `{challenges:Challenge[],candidates:Candidate[],submissions:Submission[],intents:Intent[]}` (owner only).

## Creator

- `GET /v1/prompts/challenge-scout/{version}` → `{version,prompt}` (`v1` alias).
- `POST /v1/prompts/challenge-scout/{version}/prefill` with `{topic}` → `{version,prompt}`.
- `POST /v1/candidates/validate` with `{document:string}` → `{valid:boolean,findings:Finding[],candidate?:object}`.
- `POST /v1/candidates/import` with `{document:string}` → `{id,status:"resolving_sources",candidate:object,findings:[]}`. Resolution is asynchronous.
- `GET /v1/candidates/{id}` → `{id,status,candidate,findings,createdAt}`.
- `POST /v1/challenges` with `{candidateId,repository:"owner/repo",ref:"full 40-character SHA",adoptionStatement}` → `{id,slug,versionId,status:"draft"}`. Server fetches the exact remote commit, manifest, and full source tree. Public repository required.
- `POST /v1/challenge-versions/{id}/preflights` with `{}` → `{id,status:"queued",versionId}`.
- `GET /v1/preflights/{id}` → `{id,versionId,status,findings,reports,createdAt}`.
- `POST /v1/challenge-versions/{id}/lock` with `{}` → `{versionId,status,lockDigest}`. Requires passing executable receipts and science review.
- `POST /v1/challenge-versions/{id}/publish` with `{}` → `{versionId,status:"published"}`.

## Solver

- `POST /v1/submission-intents` with `{versionId,repository,ref,previewDigest?,parentFrontierDigest?,license,attribution:{model?,harness?,disclosure?,platformSeeded?:boolean},publish:boolean}` → Intent.
- Intent: `{id,versionId,status,repository,sourceCommit?,artifactDigest?,findings,submissionId?,createdAt}`. Status `github_fetch`, `quarantine_pending`, `ready`, `failed`, `accepted`.
- `GET /v1/submission-intents/{id}` → Intent.
- `POST /v1/submission-intents/{id}/accept` with `{}` → `{submissionId,sequence,receiptDigest,status:"accepted"}`. Only ready intents; no place in line until capacity and grants are atomically reserved.
- `GET /v1/submissions/{id}` → Submission (owner/editor or public only).
- Submission: `{id,versionId,sequence,status,outcome,scoreTicks?,artifactDigest?,repository?,sourceCommit?,public,attribution,createdAt,receiptDigest?,adjudicationDigest?,claims:[],runs:[]}`.
- `POST /v1/submissions/{id}/publish` with `{}` → `{id,public:true}`.
- `GET /v1/challenge-versions/{id}/events` → SSE `event: update`, public audit projection.
- `GET /v1/receipts/{digest}` → signed protocol envelope (private receipt authorization retained).
- `GET /v1/exports/challenge-versions/{id}` → JSON full public challenge, receipts, audit, artifacts descriptors.
- `GET /v1/artifacts/{digest}` → streamed bytes when public or owned.

## Editorial and access

- `POST /v1/flags` with `{versionId,category,message,evidenceUrl?}` → `{id,status:"open"}`.
- `GET /v1/editor/queue` → `{flags:[],reviews:[],candidates:[]}` (editor/operator).
- `POST /v1/editor/decisions` with `{versionId,action:"approve_review"|"changes_required"|"reject"|"feature"|"unfeature"|"human_reviewed"|"pause"|"resume"|"compromise",reason}` → `{id,action}`.
- `POST /v1/invites` with `{githubId,role:"member"|"editor",validationQuota:20}` → `{githubId,role,validationQuota}` (operator).
- `POST /v1/auth/cli-sessions` with `{}` → `{id,userCode,verificationUrl,expiresAt}` (unauthenticated).
- `POST /v1/auth/cli-sessions/{id}/approve` with `{userCode}` → `{approved:true}` (logged in invited user).
- `POST /v1/auth/cli-sessions/{id}/token` with `{deviceSecret}` → `{token,expiresAt}` only once after approval.

## Trusted runner boundary

Separate listener `RUNNER_LISTEN_ADDR`, client TLS certificates required. `/internal/v1/runner/jobs/claim` and `/internal/v1/runner/jobs/{id}/result`. Host identities are pinned to public signing keys and independent administrative host groups. Signed jobs contain explicit digest-pinned inputs and one-use results capability. Queue claims are fenced and leased; stale or duplicate conflicting results fail. No browser session works on these endpoints. Missing runner trust prevents preflight/acceptance; it never produces a simulated official result.
