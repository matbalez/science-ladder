package platform

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/jackc/pgx/v5"
	logaudit "github.com/matbalez/science-ladder/internal/audit"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/http"
	"time"
)

func (s *Server) queueSubmission(ctx context.Context, id string) error {
	var lockBytes []byte
	var version, artifact, disk, acceptance, storedLockDigest, acceptancePolicy string
	var grant, acceptedMode string
	var acceptedOfficial bool
	err := s.DB.QueryRow(ctx, `SELECT l.document,l.digest,COALESCE(NULLIF(a.payload->>'verificationPolicy',''),'independent'),ss.version_id,ss.artifact_digest,ss.disk_digest,ss.grant_id,ss.receipt_digest,a.payload->>'deploymentMode',COALESCE((a.payload->>'officialAcceptance')::boolean,false) FROM submissions ss JOIN receipts a ON a.digest=ss.receipt_digest JOIN challenge_versions v ON v.id=ss.version_id JOIN locks l ON l.digest=v.lock_digest WHERE ss.id=$1`, id).Scan(&lockBytes, &storedLockDigest, &acceptancePolicy, &version, &artifact, &disk, &grant, &acceptance, &acceptedMode, &acceptedOfficial)
	if err != nil {
		return err
	}
	var lock protocol.Lock
	if err = json.Unmarshal(lockBytes, &lock); err != nil {
		return err
	}
	lockDigest, err := protocol.Digest(json.RawMessage(lockBytes))
	if err != nil {
		return err
	}
	if lockDigest != storedLockDigest || !protocol.ValidVerificationPolicy(acceptancePolicy) || acceptancePolicy != protocol.LockVerificationPolicy(lock) {
		return fail(422, "acceptance_lock_mismatch", "Immutable lock bytes and verification policy must match the accepted contract")
	}
	job := protocol.RunnerJob{VerificationPolicy: protocol.LockVerificationPolicy(lock), DeploymentMode: acceptedMode, OfficialAcceptance: acceptedOfficial, APIVersion: protocol.APIVersion, Kind: "ValidationJob", ID: ID(), CreatedAt: time.Now().UTC(), Producer: "science-ladder", Purpose: "submission", SubmissionID: id, AcceptanceReceiptDigest: acceptance, ChallengeLockDigest: lockDigest, ArtifactDigest: artifact, SuiteDigest: lock.SuiteDigest, ExecutionProfileDigest: lock.ExecutionProfileDigest, RunnerEpoch: "1", FencingToken: 1, Manifest: lock.Manifest, ValidatorDisk: protocol.ObjectRef{Digest: lock.ValidatorDiskDigest}, SubmissionDisk: protocol.ObjectRef{Digest: disk}, SuiteDisk: protocol.ObjectRef{Digest: lock.SuiteDiskDigest}, ChallengeDisk: protocol.ObjectRef{Digest: lock.ValidatorDiskDigest}}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM runner_jobs WHERE submission_id=$1 AND purpose='submission')`, id).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err = tx.Exec(ctx, `INSERT INTO runner_jobs(id,purpose,version_id,submission_id,payload) VALUES($1,'submission',$2,$3,$4)`, job.ID, version, id, raw(job)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE validation_grants SET status='consumed' WHERE id=$1 AND status='reserved'`, grant); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE submissions SET status='queued' WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Expired leases never produce a competitive loss. Superseding the fence makes
// every old result incapable of committing, while reservations remain durable.
// Explicit platform policy permits a fresh attempt on the same host. Existing
// exclusions remain intact; independent and legacy jobs still exclude that host.
func (s *Server) recoverRunnerLeases(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `UPDATE runner_jobs j SET
		status=CASE WHEN attempts>=8 THEN 'attention_required' ELSE 'queued' END,
		fence=fence+1,
		payload=CASE WHEN payload->>'verificationPolicy'='platform' THEN payload
			ELSE jsonb_set(payload,'{excludedHostIds}',COALESCE(payload->'excludedHostIds','[]'::jsonb)||to_jsonb(host_id)) END,
		host_id=NULL,lease_expires_at=NULL,result_token_hash=NULL
		WHERE status='running' AND lease_expires_at<now()
		AND NOT EXISTS(SELECT 1 FROM runner_results r WHERE r.job_id=j.id)`)
	return err
}

type runnerIdentity struct {
	ID, Group, PublicKey, ExecutionProfile, EncryptionKey, AdvisorySnapshotDigest, RuntimeInventoryDigest string
	Purposes                                                                                              []string
}

func (s *Server) runnerIdentity(r *http.Request) (runnerIdentity, error) {
	var host runnerIdentity
	if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
		return host, fail(401, "runner_mtls_required", "A trusted runner client certificate is required")
	}
	fp := sha256.Sum256(r.TLS.PeerCertificates[0].Raw)
	err := s.DB.QueryRow(r.Context(), `SELECT id,host_group,public_key,execution_profile_digest,purposes,encryption_public_key,advisory_snapshot_digest,runtime_inventory_digest FROM runner_hosts WHERE certificate_fingerprint=$1 AND enabled`, hex.EncodeToString(fp[:])).Scan(&host.ID, &host.Group, &host.PublicKey, &host.ExecutionProfile, &host.Purposes, &host.EncryptionKey, &host.AdvisorySnapshotDigest, &host.RuntimeInventoryDigest)
	if err != nil {
		return host, fail(403, "runner_untrusted", "Runner certificate is not in the active trusted inventory")
	}
	return host, nil
}
func (s *Server) RunnerHandler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("POST /internal/v1/runner/jobs/claim", func(w http.ResponseWriter, r *http.Request) {
		host, err := s.runnerIdentity(r)
		if err == nil {
			err = s.claimRunnerJob(w, r, host)
		}
		if err != nil {
			writeError(w, err)
		}
	})
	m.HandleFunc("POST /internal/v1/runner/jobs/{id}/result", func(w http.ResponseWriter, r *http.Request) {
		host, err := s.runnerIdentity(r)
		if err == nil {
			err = s.runnerResult(w, r, host)
		}
		if err != nil {
			writeError(w, err)
		}
	})
	m.HandleFunc("POST /internal/v1/runner/jobs/{id}/objects", func(w http.ResponseWriter, r *http.Request) {
		host, err := s.runnerIdentity(r)
		if err == nil {
			err = s.runnerObject(w, r, host)
		}
		if err != nil {
			writeError(w, err)
		}
	})
	return http.MaxBytesHandler(m, 1<<20)
}
func (s *Server) RunRunnerListener(ctx context.Context) error {
	if s.Config.RunnerListenAddr == "" {
		return nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(s.Config.RunnerClientCA)) {
		return errors.New("runner mTLS client CA is missing or invalid")
	}
	server := &http.Server{Addr: s.Config.RunnerListenAddr, Handler: s.RunnerHandler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	return server.ListenAndServeTLS(s.Config.RunnerTLSCert, s.Config.RunnerTLSKey)
}
func (s *Server) claimRunnerJob(w http.ResponseWriter, r *http.Request, host runnerIdentity) error {
	if s.Store == nil {
		return fail(503, "storage_unavailable", "Immutable storage unavailable")
	}
	if err := s.verifyHostDelegation(host, time.Now().UTC()); err != nil {
		return err
	}
	key, err := s.signer()
	if err != nil {
		return fail(503, "signing_unavailable", "Job signer unavailable")
	}
	ctx := r.Context()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id string
	var payload []byte
	var fence int64
	err = tx.QueryRow(ctx, `SELECT id,payload,fence FROM runner_jobs WHERE status='queued' AND purpose=ANY($2) AND (purpose IN ('preflight','artifact_prepare') OR payload->>'executionProfileDigest'=$4) AND NOT COALESCE(payload->'excludedHostIds','[]'::jsonb) ? $3 AND (excluded_group IS NULL OR excluded_group<>$1) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`, host.Group, host.Purposes, host.ID, host.ExecutionProfile).Scan(&id, &payload, &fence)
	if errors.Is(err, pgx.ErrNoRows) {
		respond(w, 200, map[string]any{"job": nil})
		return nil
	}
	if err != nil {
		return err
	}
	var job protocol.RunnerJob
	if err = json.Unmarshal(payload, &job); err != nil {
		return err
	}
	if job.Purpose == "preflight" {
		if !protocol.ValidDigest(host.AdvisorySnapshotDigest) || !protocol.ValidDigest(host.RuntimeInventoryDigest) {
			return fail(503, "scan_policy_unconfigured", "Quarantine host requires operator-approved vulnerability and runtime inventory digests")
		}
		if job.AdvisorySnapshotDigest != "" && (job.AdvisorySnapshotDigest != host.AdvisorySnapshotDigest || job.RuntimeInventoryDigest != host.RuntimeInventoryDigest) {
			return fail(503, "independent_scan_policy_mismatch", "Independent preflight hosts must use identical approved scan inputs")
		}
		job.AdvisorySnapshotDigest = host.AdvisorySnapshotDigest
		job.RuntimeInventoryDigest = host.RuntimeInventoryDigest
	}
	if !protocol.ValidVerificationPolicy(protocol.JobVerificationPolicy(job)) {
		return fail(422, "verification_policy_invalid", "Unknown job verification policy")
	}
	job.RequiredHostGroup = host.Group
	job.FencingToken = fence
	job.ExpiresAt = time.Now().UTC().Add(15 * time.Minute)
	refs := []*protocol.ObjectRef{&job.ValidatorDisk, &job.SubmissionDisk, &job.SuiteDisk, &job.ChallengeDisk}
	if job.Purpose == "preflight" || job.Purpose == "artifact_prepare" {
		if job.SourceSnapshot == nil {
			return fail(422, "source_missing", "Preparation source snapshot missing")
		}
		refs = []*protocol.ObjectRef{job.SourceSnapshot}
		if host.ExecutionProfile == "" {
			return fail(503, "profile_unconfigured", "Quarantine host execution profile is not registered")
		}
		job.ExecutionProfileDigest = host.ExecutionProfile
	}
	for _, ref := range refs {
		var blob string
		if err = tx.QueryRow(ctx, `SELECT size,blob_digest FROM artifacts WHERE digest=$1`, ref.Digest).Scan(&ref.Size, &blob); err != nil {
			return fail(503, "disk_missing", "An immutable prepared input disk is unavailable")
		}
		if blob != ref.Digest {
			return fail(503, "disk_not_canonical", "Official input disk digest must identify its actual immutable bytes")
		}
	}
	// Sign and presign outside the transaction; claim is completed with a fence CAS.
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	for _, ref := range refs {
		ref.URL, err = s.Store.SignedRead(ctx, ref.Digest, 15*time.Minute)
		if err != nil {
			return err
		}
	}
	if err = s.grantHiddenSuite(ctx, &job, host); err != nil {
		return err
	}
	envelope, err := protocol.Sign(s.Config.ReceiptKeyID, key, job)
	if err != nil {
		return err
	}
	token := secret()
	tag, err := s.DB.Exec(ctx, `UPDATE runner_jobs SET status='running',host_id=$2,payload=$3,envelope=$4,result_token_hash=$5,lease_expires_at=$6,attempts=attempts+1 WHERE id=$1 AND status='queued' AND fence=$7`, id, host.ID, raw(job), raw(envelope), hash(token), job.ExpiresAt, fence)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fail(409, "job_claimed", "Job was claimed concurrently; request another")
	}
	_, err = s.DB.Exec(ctx, `UPDATE runner_hosts SET last_seen_at=now() WHERE id=$1`, host.ID)
	if err != nil {
		return err
	}
	respond(w, 200, map[string]any{"job": envelope, "resultToken": token})
	return nil
}
func parseRunnerKey(text string) (crypto.PublicKey, error) {
	block, _ := pem.Decode([]byte(text))
	if block == nil {
		return nil, errors.New("runner public key invalid")
	}
	return x509.ParsePKIXPublicKey(block.Bytes)
}
func (s *Server) runnerResult(w http.ResponseWriter, r *http.Request, host runnerIdentity) error {
	var in struct {
		Envelope    protocol.Envelope `json:"envelope"`
		ResultToken string            `json:"resultToken"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	key, err := parseRunnerKey(host.PublicKey)
	if err != nil {
		return err
	}
	payload, err := protocol.Verify(in.Envelope, map[string]crypto.PublicKey{host.ID: key})
	if err != nil {
		return fail(401, "run_signature_invalid", "Runner result signature did not verify")
	}
	var run protocol.RunReceipt
	if err = protocol.DecodeStrict(payload, &run); err != nil {
		return fail(422, "run_invalid", "Runner receipt is invalid")
	}
	if run.APIVersion != protocol.APIVersion || run.Kind != "ValidationRunReceipt" || run.CreatedAt.After(time.Now().Add(time.Minute)) || run.HostID != host.ID || run.HostGroup != host.Group || !run.Official || !run.CleanupAttested || run.JobID != r.PathValue("id") {
		return fail(422, "run_identity_mismatch", "Run identity, official isolation, or cleanup attestations do not match")
	}
	if err := s.verifyHostDelegation(host, run.CreatedAt); err != nil {
		return err
	}
	digest, err := protocol.Digest(run)
	if err != nil {
		return err
	}
	if run.BuildReport != nil {
		if err = s.verifyBuildObjects(r.Context(), run.JobID, run.BuildReport); err != nil {
			return err
		}
	}
	ctx := r.Context()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var jobData []byte
	var expectedToken, status, version string
	var claimedHost *string
	var deadline *time.Time
	var fence int64
	err = tx.QueryRow(ctx, `SELECT payload,result_token_hash,status,host_id,lease_expires_at,fence,version_id FROM runner_jobs WHERE id=$1 FOR UPDATE`, run.JobID).Scan(&jobData, &expectedToken, &status, &claimedHost, &deadline, &fence, &version)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(hash(in.ResultToken)), []byte(expectedToken)) {
		return fail(403, "result_capability_invalid", "One-use result capability is invalid")
	}
	if status == "complete" {
		var stored string
		if err = tx.QueryRow(ctx, `SELECT digest FROM runner_results WHERE job_id=$1`, run.JobID).Scan(&stored); err != nil {
			return err
		}
		if stored != digest {
			return fail(409, "result_conflict", "A different result already consumed this attempt")
		}
		respond(w, 200, map[string]any{"accepted": true, "duplicate": true})
		return nil
	}
	if status != "running" || claimedHost == nil || *claimedHost != host.ID || deadline == nil || time.Now().After(*deadline) || fence != run.FencingToken {
		return fail(409, "stale_runner_lease", "Run result belongs to a stale, expired, or different host lease")
	}
	var job protocol.RunnerJob
	if err = json.Unmarshal(jobData, &job); err != nil {
		return err
	}
	jobDigest, err := protocol.Digest(job)
	if err != nil {
		return err
	}
	if !protocol.ValidVerificationPolicy(protocol.JobVerificationPolicy(job)) || run.ParentJobDigest != job.ParentJobDigest || run.VerificationPolicy != job.VerificationPolicy || run.JobDigest != jobDigest || run.AcceptanceReceiptDigest != job.AcceptanceReceiptDigest || run.ChallengeLockDigest != job.ChallengeLockDigest || run.ArtifactDigest != job.ArtifactDigest || run.SuiteDigest != job.SuiteDigest || run.ExecutionProfileDigest != job.ExecutionProfileDigest || run.RunnerEpoch != job.RunnerEpoch || run.DeploymentMode != job.DeploymentMode || run.OfficialAcceptance != job.OfficialAcceptance {
		return fail(422, "run_bindings_mismatch", "Run result did not bind every immutable input and execution policy")
	}
	if run.Outcome == "infrastructure_fault" {
		if _, err = tx.Exec(ctx, `UPDATE runner_jobs SET status='queued',fence=fence+1,host_id=NULL,lease_expires_at=NULL,payload=jsonb_set(payload,'{excludedHostIds}',COALESCE(payload->'excludedHostIds','[]'::jsonb)||to_jsonb($2::text)) WHERE id=$1`, run.JobID, host.ID); err != nil {
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		respond(w, 202, map[string]any{"accepted": true, "retrying": true})
		return nil
	}
	isBuild := job.Purpose == "preflight" || job.Purpose == "artifact_prepare"
	if isBuild && run.Outcome == "valid" {
		if err = validateBuild(job, run); err != nil {
			return err
		}
		if job.Purpose == "preflight" {
			if err = validateFixtureEvidence(job, run, host); err != nil {
				return err
			}
		}
	}
	terminal := map[string]bool{"valid": true, "hard_gate_failed": true, "invalid_output": true, "resource_limit": true, "declared_timeout": true, "nondeterministic": true, "malicious": true, "challenge_fault": true}
	if !terminal[run.Outcome] {
		return fail(422, "run_outcome_invalid", "Unknown competitive outcome requires operator resolution")
	}
	if run.Outcome == "valid" && !isBuild {
		if len(run.Gates) != len(job.Manifest.HardGates) {
			return fail(422, "gates_invalid", "Every locked gate must be present exactly once")
		}
		for _, g := range job.Manifest.HardGates {
			if !run.Gates[g] {
				return fail(422, "gates_invalid", "A valid result cannot fail a hard gate")
			}
		}
		if _, err = protocol.CompareTicks(run.ScoreTicks, run.ScoreTicks); err != nil {
			return fail(422, "score_invalid", "Score ticks must be a canonical integer")
		}
		if m := job.Manifest.Metric; m.DomainMinTicks != "" && !crosses(run.ScoreTicks, m.DomainMinTicks, "maximize") {
			return fail(422, "score_domain", "Score ticks are below the locked domain")
		}
		if m := job.Manifest.Metric; m.DomainMaxTicks != "" && !crosses(run.ScoreTicks, m.DomainMaxTicks, "minimize") {
			return fail(422, "score_domain", "Score ticks exceed the locked domain")
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO runner_results(job_id,host_id,digest,envelope,result) VALUES($1,$2,$3,$4,$5)`, job.ID, host.ID, digest, raw(in.Envelope), raw(run)); err != nil {
		return err
	}
	var receiptOwner string
	if job.Purpose == "artifact_prepare" {
		if err = tx.QueryRow(ctx, `SELECT i.owner_id FROM runner_jobs j JOIN submission_intents i ON i.id=j.intent_id WHERE j.id=$1`, job.ID).Scan(&receiptOwner); err != nil {
			return err
		}
	} else if job.SubmissionID != "" {
		if err = tx.QueryRow(ctx, `SELECT owner_id FROM submissions WHERE id=$1`, job.SubmissionID).Scan(&receiptOwner); err != nil {
			return err
		}
	} else {
		if err = tx.QueryRow(ctx, `SELECT c.owner_id FROM challenge_versions v JOIN challenges c ON c.id=v.challenge_id WHERE v.id=$1`, version).Scan(&receiptOwner); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO receipts(digest,payload,owner_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, digest, raw(run), receiptOwner); err != nil {
		return err
	}
	if err = enqueue(ctx, tx, "sign_receipt", digest); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE runner_jobs SET status='complete',lease_expires_at=NULL WHERE id=$1`, job.ID); err != nil {
		return err
	}
	if isBuild {
		if err = s.commitBuild(ctx, tx, job, run, digest, version); err != nil {
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		respond(w, 200, map[string]any{"accepted": true, "final": true})
		return nil
	}
	if run.Outcome == "challenge_fault" {
		if _, err = tx.Exec(ctx, `UPDATE submissions SET status='challenge_fault',outcome='pending' WHERE id=$1`, job.SubmissionID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE challenge_versions SET intake_status='paused' WHERE id=$1`, version); err != nil {
			return err
		}
		if err = audit(ctx, tx, version, "challenge.incident", map[string]any{"versionId": version, "reason": "A locked checker fault requires version-wide resolution; receipt ordering remains blocked."}); err != nil {
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		respond(w, 200, map[string]any{"accepted": true, "final": false, "incident": true})
		return nil
	}
	outcome, score := run.Outcome, run.ScoreTicks
	final := true
	if run.Outcome == "valid" && job.Purpose == "submission" {
		confirmation := job
		confirmation.ID = ID()
		confirmation.CreatedAt = time.Now().UTC()
		confirmation.Purpose = "confirmation"
		confirmation.RequiredHostGroup = ""
		var excludedGroup any
		confirmation.ExcludedHostIDs = nil
		if protocol.JobVerificationPolicy(job) == protocol.VerificationIndependent {
			confirmation.ExcludedHostIDs = []string{host.ID}
			excludedGroup = host.Group
		}
		confirmation.FencingToken = 1
		if _, err = tx.Exec(ctx, `INSERT INTO runner_jobs(id,purpose,version_id,submission_id,payload,excluded_group) VALUES($1,'confirmation',$2,$3,$4,$5)`, confirmation.ID, version, job.SubmissionID, raw(confirmation), excludedGroup); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE submissions SET status='confirmation_running' WHERE id=$1`, job.SubmissionID); err != nil {
			return err
		}
		final = false
	} else if job.Purpose == "confirmation" {
		var primary []byte
		var primaryGroup string
		err = tx.QueryRow(ctx, `SELECT rr.result,rr.result->>'hostGroup' FROM runner_results rr JOIN runner_jobs j ON j.id=rr.job_id WHERE j.submission_id=$1 AND j.purpose='submission'`, job.SubmissionID).Scan(&primary, &primaryGroup)
		if err != nil {
			return err
		}
		if protocol.JobVerificationPolicy(job) == protocol.VerificationIndependent && primaryGroup == host.Group {
			return fail(422, "confirmation_not_independent", "Confirmation host group must differ from the primary group")
		}
		var first protocol.RunReceipt
		if err = json.Unmarshal(primary, &first); err != nil {
			return err
		}
		if first.JobID == run.JobID || first.ID == run.ID || first.CreatedAt.After(run.CreatedAt) {
			return fail(422, "confirmation_not_fresh", "Confirmation must be a separate fresh isolated attempt after the primary")
		}
		if run.Outcome != first.Outcome {
			outcome = "nondeterministic"
			score = ""
		} else if run.Outcome == "valid" {
			score, err = ConfirmedScore(first.ScoreTicks, run.ScoreTicks, job.Manifest.Metric)
			if err != nil {
				outcome = "nondeterministic"
				score = ""
			}
		}
	}
	if final {
		var tick any
		if score != "" && outcome == "valid" {
			tick = score
		}
		if _, err = tx.Exec(ctx, `UPDATE submissions SET status='validated',outcome=$2,score_ticks=$3 WHERE id=$1`, job.SubmissionID, outcome, tick); err != nil {
			return err
		}
		if outcome == "nondeterministic" {
			if _, err = tx.Exec(ctx, `UPDATE challenge_versions SET intake_status='paused' WHERE id=$1`, version); err != nil {
				return err
			}
		}
		if err = enqueue(ctx, tx, "adjudicate", job.SubmissionID); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	respond(w, 200, map[string]any{"accepted": true, "final": final})
	return nil
}

// The database inventory supplies operational routing, while a production
// runner's signing authority must also be delegated by the pinned offline root.
func (s *Server) verifyHostDelegation(host runnerIdentity, at time.Time) error {
	if s.Config.DeploymentMode != "production" {
		return nil
	}
	if s.TrustHistory == nil {
		return fail(503, "runner_delegation_missing", "Production runner identities require root-signed delegation")
	}
	key, err := parseRunnerKey(host.PublicKey)
	if err != nil {
		return err
	}
	actual, err := logaudit.Fingerprint(key)
	if err != nil {
		return err
	}
	permitted := s.TrustHistory.KeysAt("validation-run", at)
	want, err := logaudit.Fingerprint(permitted[host.ID])
	if err != nil || want != actual {
		return fail(403, "runner_delegation_invalid", "Runner signing key is not actively delegated")
	}
	for _, d := range s.TrustHistory.Delegations {
		if d.KeyID == host.ID && d.HostID == host.ID {
			return nil
		}
	}
	return fail(403, "runner_host_binding_invalid", "Delegated runner key belongs to a different host")
}
