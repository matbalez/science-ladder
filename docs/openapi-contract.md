# REST integration contract

All routes use `/v1`. JSON camelCase, UUID resource IDs, RFC3339 timestamps, exact integer ticks encoded as strings. Errors: `{error:{code,message,details?}}`. Every authenticated mutation requires an `Idempotency-Key` (8–128 characters); retrying the same request returns the same resource, changed content with the same key is rejected. Same-origin browser cookies; scoped bearer tokens for CLI. The application never runs scientific validators.

## Discovery and account

- `GET /healthz`, `GET /readyz` (outside /v1).
- `GET /v1/me` → `{user:null|{id,githubId,login,avatarUrl,role,invited},quotas:{remaining,activeLimit},capabilities:{creation,submission,review},configuration:{githubAuth,scientificReview,platformRunner,independentRunner,officialRunner}}`.
- `GET /v1/auth/github` starts GitHub OAuth; callback `/v1/auth/github/callback`.
- `POST /v1/auth/logout` clears session.
- `GET /v1/challenges?search=&limit=24&cursor=` → `{challenges:Challenge[],nextCursor?}`.
- `GET /v1/challenges/{slug}` → Challenge.
- Challenge: `{id,slug,title,summary,domain,status,reviewStatus,intakeStatus,economicMode:"none",verificationPolicy:"platform"|"independent",versionId,repository,sourceCommit,createdAt,deadline,metric:{name,direction,units,quantum,baselineTicks},milestones:[{id,label,thresholdTicks,claimedBy?,claimedAt?}],verifiedBest?:{submissionId,scoreTicks},publicFrontier?:{submissionId,scoreTicks},badges:string[],manifest?:object,reviews:object[],submissions:Submission[]}`.
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
- Submission: `{id,versionId,sequence,status,outcome,verificationPolicy:"platform"|"independent",verificationStatus:""|"platform_verified"|"independently_replicated",independentReplication:boolean,scoreTicks?,artifactDigest?,repository?,sourceCommit?,public,attribution,createdAt,receiptDigest?,adjudicationDigest?,claims:[],runs:[]}`.
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

## Hidden suites and version transitions

`POST /v1/suites` accepts `{document,license,provenance}`. `document` is JSON `{files:{"relative/path.json":"base64..."}}`, bounded to 1 MiB of inert decoded data. The response contains a suite ID and salted commitment, never the encryption key, salt, or private content digest. The creator binds that commitment into the manifest. At-rest secret material uses `HIDDEN_SUITE_KMS_KEY_ID` and `HIDDEN_SUITE_KMS_REGION`; local AES master keys are permitted only in explicitly local or controlled-demo deployments. Separate host X25519 keys receive job-bound capsules. `POST /v1/suites/{id}/reveal` publishes signed reveal evidence only when all referencing seasons are closed and every immutable reveal time has passed. Reveal keeps source bytes encrypted in object storage: the public receipt binds `sourceFormat: encrypted-suite-object-v1`, ciphertext `sourceDigest`, `plaintextDigest`, salt and suite key. Consumers decrypt with the protocol suite-object decoder using role `source`; no plaintext source is uploaded before the atomic eligibility decision.

`POST /v1/challenges/{id}/versions` accepts `{repository,ref,adoptionStatement,transitionKind?}`. The default `season` closes a drained predecessor and requires fresh global milestone IDs. A `security_migration` requires paused or closed predecessor intake and a fully resolved receipt watermark. If a previous public frontier exists, the source must contain a `previous_frontier` fixture whose canonical artifact bytes exactly match that frontier, with a declared valid outcome and expected ticks. All fixtures must pass the new version’s declared verification policy before locking. Publishing a migration returns `202` and `migration_signing`; only after the immutable migration receipt is signed does a serializable transaction transfer still-unclaimed milestone mappings, close the predecessor, and open the successor. A pending security migration prevents predecessor resume.

Voluntarily publishing a finalized valid submission advances the public frontier if it satisfies the locked minimum improvement, preserves existing milestone claims, and appends a signed `SolutionPublicationReceipt`. Public submission exports disclose the salted acceptance commitment opening. Every run includes the original host envelope and a combined envelope retaining both the host signature and platform countersignature.

## Audit verification

- `GET /v1/audit/events?after=0&limit=1000` returns `{events}` from the contiguous global hash chain. Sequence values are decimal strings.
- `GET /v1/audit/checkpoints?afterDigest=<digest>&limit=1` returns `{checkpoints:[{id,digest,bundle:{checkpoint,witnesses,events},quorumVerified,issuedAt}],deploymentMode}`. An unknown committed digest returns 404. `after` is an alternative numeric cursor; supplying both is invalid. Maximum limit is 20.
- `POST /v1/audit/checkpoints/{digest}/witness` accepts `{envelope}` and authenticates by independently delegated witness signatures over exactly the known committed checkpoint. Browser identity and `Idempotency-Key` are unnecessary.

Challenge export audit entries are a version projection, not a contiguous chain. Exports point to the global audit endpoints and include authorized artifact descriptors, signed migration/publication/reveal evidence, and original host run signatures. Large artifact downloads redirect to a short-lived storage capability after owner/public authorization.

The separate runner inventory must include each host's immutable signing key, mTLS certificate fingerprint, independent physical administrative group, guest execution profile digest, allowed job purposes, and separate X25519 encryption key. Preflight hosts additionally need operator-approved `advisory_snapshot_digest` and `runtime_inventory_digest`; both are bound into every signed preflight job and must agree across independent hosts. Missing scan policy or unresolved high/critical vulnerabilities blocks conformance. Production hosts also need active root-delegated `validation-run` authority bound to that host ID.

Scientific review independently resolves the final locked candidate manifest sources, records its manifest digest and current source findings, and cannot inherit earlier candidate evidence after edits. HTML quotation matching excludes scripts, comments, metadata, templates and explicitly hidden markup. Unretrievable or unsafe final sources force changes required; uncertain exact evidence or recency requires a human decision.

## Immutable verification policy

New manifests default to `platform` and new locks explicitly freeze `verificationPolicy`. Platform preflight requires baseline and valid fixtures to run twice in fresh isolated VMs with signed child-run evidence; submission acceptance requires one enrolled host group and a separate primary and fresh confirmation job. Matching repeats produce `platform_verified`, never independent replication. `independent` additionally requires two distinct enrolled host groups for preflight and submission confirmation and produces `independently_replicated`. Existing locks and jobs without the field preserve their historical `independent` requirement. Changing the policy requires a new version and lock.

Assurance is separate from `deploymentMode` and `officialAcceptance`: controlled demos may show genuine platform verification while external release gates remain incomplete. `/me.configuration.officialRunner` is a compatibility alias for platform host availability; use `platformRunner` and `independentRunner` for explicit UI labels.
