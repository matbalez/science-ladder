package platform

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/http"
	"time"
)

func (s *Server) queuePreparation(ctx context.Context, tx pgx.Tx, purpose, id, version, source, artifact string, m protocol.Manifest) error {
	job := protocol.RunnerJob{DeploymentMode: s.Config.DeploymentMode, OfficialAcceptance: false, APIVersion: protocol.APIVersion, Kind: "ValidationJob", ID: ID(), CreatedAt: time.Now().UTC(), Producer: "science-ladder", Purpose: purpose, ArtifactDigest: artifact, RunnerEpoch: "1", FencingToken: 1, Manifest: m, SourceSnapshot: &protocol.ObjectRef{Digest: source}}
	if purpose == "preflight" {
		_, err := tx.Exec(ctx, `INSERT INTO runner_jobs(id,purpose,version_id,preflight_id,payload) SELECT $1,'preflight',$2,$3,$4 WHERE NOT EXISTS(SELECT 1 FROM runner_jobs WHERE preflight_id=$3)`, job.ID, version, id, raw(job))
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO runner_jobs(id,purpose,version_id,intent_id,payload) SELECT $1,'artifact_prepare',$2,$3,$4 WHERE NOT EXISTS(SELECT 1 FROM runner_jobs WHERE intent_id=$3)`, job.ID, version, id, raw(job))
	return err
}
func (s *Server) runnerObject(w http.ResponseWriter, r *http.Request, host runnerIdentity) error {
	if s.Store == nil {
		return fail(503, "storage_unavailable", "Immutable object storage unavailable")
	}
	var in struct {
		Role        string `json:"role"`
		Digest      string `json:"digest"`
		Size        int64  `json:"size"`
		ResultToken string `json:"resultToken"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.Role != "validatorDisk" && in.Role != "challengeDisk" && in.Role != "suiteDisk" && in.Role != "submissionDisk" {
		return fail(422, "object_role_invalid", "Prepared object role is not allowed")
	}
	if in.Size <= 0 || in.Size > 1<<30 {
		return fail(422, "object_size_invalid", "Prepared object exceeds policy")
	}
	var expected, purpose string
	var fence int64
	err := s.DB.QueryRow(r.Context(), `SELECT result_token_hash,purpose,fence FROM runner_jobs WHERE id=$1 AND host_id=$2 AND status='running' AND lease_expires_at>now()`, r.PathValue("id"), host.ID).Scan(&expected, &purpose, &fence)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(hash(in.ResultToken)), []byte(expected)) {
		return fail(403, "result_capability_invalid", "Upload capability invalid")
	}
	if purpose != "preflight" && purpose != "artifact_prepare" {
		return fail(403, "object_upload_forbidden", "Competitive runs cannot upload arbitrary objects")
	}
	if purpose == "artifact_prepare" && in.Role != "submissionDisk" {
		return fail(403, "object_role_forbidden", "Artifact preparation can upload only a submission disk")
	}
	url, err := s.Store.SignedWrite(r.Context(), in.Digest, in.Size, 10*time.Minute)
	if err != nil {
		return err
	}
	tag, err := s.DB.Exec(r.Context(), `INSERT INTO runner_uploads(job_id,role,digest,size) VALUES($1,$2,$3,$4) ON CONFLICT(job_id,role) DO UPDATE SET digest=excluded.digest,size=excluded.size WHERE runner_uploads.digest=excluded.digest AND runner_uploads.size=excluded.size`, r.PathValue("id"), in.Role, in.Digest, in.Size)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fail(409, "object_conflict", "This job role already declared a different immutable object")
	}
	respond(w, 200, map[string]any{"url": url, "headers": map[string]string{"If-None-Match": "*"}})
	return nil
}
func (s *Server) verifyBuildObjects(ctx context.Context, jobID string, build *protocol.BuildReport) error {
	if build == nil {
		return fail(422, "build_report_required", "Preparation requires a typed signed build report")
	}
	refs := map[string]*protocol.ObjectRef{"validatorDisk": build.ValidatorDisk, "challengeDisk": build.ChallengeDisk, "suiteDisk": build.SuiteDisk, "submissionDisk": build.SubmissionDisk}
	for role, ref := range refs {
		if ref == nil {
			continue
		}
		var digest string
		var size int64
		if err := s.DB.QueryRow(ctx, `SELECT digest,size FROM runner_uploads WHERE job_id=$1 AND role=$2`, jobID, role).Scan(&digest, &size); err != nil {
			return err
		}
		if ref.Digest != digest || ref.Size != size {
			return fail(422, "uploaded_object_mismatch", "Build output does not match the job upload capability")
		}
		if err := s.Store.Verify(ctx, digest, size); err != nil {
			return err
		}
		if _, err := s.DB.Exec(ctx, `UPDATE runner_uploads SET verified=true WHERE job_id=$1 AND role=$2`, jobID, role); err != nil {
			return err
		}
	}
	return nil
}
func validateBuild(job protocol.RunnerJob, run protocol.RunReceipt) error {
	b := run.BuildReport
	if b == nil || !b.Passed || !b.ScansPassed {
		return fail(422, "conformance_failed", "Build and safety checks must pass")
	}
	md, err := protocol.Digest(job.Manifest)
	if err != nil {
		return err
	}
	if b.ManifestDigest != md || job.SourceSnapshot == nil || b.SourceSnapshotDigest != job.SourceSnapshot.Digest || b.ExecutionProfileDigest != job.ExecutionProfileDigest {
		return fail(422, "build_bindings_invalid", "Build report does not bind its signed manifest, source and execution profile")
	}
	if job.Purpose == "artifact_prepare" {
		if b.SubmissionDisk == nil || b.SubmissionDisk.Digest == "" {
			return fail(422, "submission_disk_missing", "Preparation must produce a final read-only submission disk")
		}
		return nil
	}
	if b.ValidatorDisk == nil || b.ChallengeDisk == nil || b.SuiteDisk == nil || b.ValidatorDisk.Digest != b.ValidatorDiskDigest || b.SuiteDisk.Digest != b.SuiteDigest || b.ValidatorDiskDigest != b.RebuiltValidatorDiskDigest || !b.OfflineRebuild || !b.HostileCorpusPassed {
		return fail(422, "reproducibility_failed", "Preflight requires matching offline rebuilds, immutable disk bindings, and the hostile corpus")
	}
	if b.ValidatorImageDigest != job.Manifest.Validator.RuntimeImageDigest {
		return fail(422, "runtime_image_mismatch", "Preflight runtime differs from the locked validator runtime")
	}
	fixtures := map[string]protocol.FixtureReport{}
	for _, f := range b.Fixtures {
		if _, exists := fixtures[f.Name]; exists {
			return fail(422, "fixture_duplicate", "Fixture receipts must be unique")
		}
		fixtures[f.Name] = f
	}
	for _, f := range job.Manifest.Fixtures {
		actual, ok := fixtures[f.Name]
		if !ok || !actual.Passed || actual.ExpectedOutcome != f.ExpectedOutcome || actual.Outcome != f.ExpectedOutcome {
			return fail(422, "fixture_failed", "Every manifest fixture must have a passing outcome")
		}
		if f.ExpectedTicks != "" && actual.ScoreTicks != f.ExpectedTicks {
			return fail(422, "fixture_score_mismatch", "Fixture score differs from expected ticks")
		}
	}
	return nil
}
func (s *Server) commitBuild(ctx context.Context, tx pgx.Tx, job protocol.RunnerJob, run protocol.RunReceipt, digest, version string) error {
	if run.BuildReport == nil {
		return fail(422, "build_report_required", "A signed build report is required")
	}
	if !run.BuildReport.Passed || run.Outcome != "valid" {
		findings := []Finding{}
		for _, f := range run.BuildReport.Findings {
			findings = append(findings, Finding{"preflight_failed", f, "error", ""})
		}
		if job.Purpose == "artifact_prepare" {
			_, err := tx.Exec(ctx, `UPDATE submission_intents SET status='failed',findings=$2 WHERE id=(SELECT intent_id FROM runner_jobs WHERE id=$1)`, job.ID, raw(findings))
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE preflights SET status='fail_with_actionable_findings',findings=$2,reports=$3 WHERE id=(SELECT preflight_id FROM runner_jobs WHERE id=$1)`, job.ID, raw(findings), raw(run.BuildReport)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE challenge_versions SET status='changes_required' WHERE id=$1 AND lock_digest IS NULL`, version)
		return err
	}
	if err := validateBuild(job, run); err != nil {
		return err
	}
	b := run.BuildReport
	var owner string
	var intent, preflight *string
	if err := tx.QueryRow(ctx, `SELECT c.owner_id,j.intent_id,j.preflight_id FROM runner_jobs j JOIN challenge_versions v ON v.id=j.version_id JOIN challenges c ON c.id=v.challenge_id WHERE j.id=$1`, job.ID).Scan(&owner, &intent, &preflight); err != nil {
		return err
	}
	if intent != nil {
		if err := tx.QueryRow(ctx, `SELECT owner_id FROM submission_intents WHERE id=$1`, *intent).Scan(&owner); err != nil {
			return err
		}
	}
	refs := []*protocol.ObjectRef{b.ValidatorDisk, b.ChallengeDisk, b.SuiteDisk, b.SubmissionDisk}
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		var verified bool
		if err := tx.QueryRow(ctx, `SELECT verified FROM runner_uploads WHERE job_id=$1 AND digest=$2 AND size=$3 LIMIT 1`, job.ID, ref.Digest, ref.Size).Scan(&verified); err != nil {
			return err
		}
		if !verified {
			return fail(409, "object_verification_pending", "Uploaded disk must be independently verified")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO artifacts(digest,blob_digest,size,media_type,owner_id) VALUES($1,$1,$2,'application/vnd.squashfs',$3) ON CONFLICT DO NOTHING`, ref.Digest, ref.Size, owner); err != nil {
			return err
		}
	}
	if job.Purpose == "artifact_prepare" {
		if intent == nil {
			return errors.New("preparation intent missing")
		}
		_, err := tx.Exec(ctx, `UPDATE submission_intents SET disk_digest=$2,status='ready',findings='[]' WHERE id=$1 AND status='quarantine_pending'`, *intent, b.SubmissionDisk.Digest)
		return err
	}
	if preflight == nil {
		return errors.New("preflight id missing")
	}
	var previous []byte
	var previousDigest string
	err := tx.QueryRow(ctx, `SELECT rr.result,rr.digest FROM runner_results rr JOIN runner_jobs j ON j.id=rr.job_id WHERE j.preflight_id=$1 AND j.id<>$2 ORDER BY rr.created_at LIMIT 1`, *preflight, job.ID).Scan(&previous, &previousDigest)
	if errors.Is(err, pgx.ErrNoRows) {
		next := job
		next.ID = ID()
		next.CreatedAt = time.Now().UTC()
		next.RequiredHostGroup = ""
		next.ExcludedHostIDs = []string{run.HostID}
		next.FencingToken = 1
		if _, err = tx.Exec(ctx, `INSERT INTO runner_jobs(id,purpose,version_id,preflight_id,payload,excluded_group) VALUES($1,'preflight',$2,$3,$4,$5)`, next.ID, version, *preflight, raw(next), run.HostGroup); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE preflights SET status='independent_confirmation' WHERE id=$1`, *preflight)
		return err
	}
	if err != nil {
		return err
	}
	var first protocol.RunReceipt
	if err = json.Unmarshal(previous, &first); err != nil {
		return err
	}
	if first.HostGroup == run.HostGroup || first.BuildReport == nil || first.BuildReport.ValidatorDiskDigest != b.ValidatorDiskDigest || first.BuildReport.SuiteDigest != b.SuiteDigest || first.ExecutionProfileDigest != run.ExecutionProfileDigest {
		return fail(422, "independent_build_disagreement", "Independent preflight hosts disagree on immutable build outputs")
	}
	receipt := protocol.Receipt{DeploymentMode: s.Config.DeploymentMode, OfficialAcceptance: false, APIVersion: protocol.APIVersion, Kind: "MachineConformanceReceipt", ID: ID(), CreatedAt: time.Now().UTC(), Producer: "science-ladder", SubjectDigest: job.SourceSnapshot.Digest, EconomicMode: "none", Data: map[string]any{"versionId": version, "buildReport": b, "runDigests": []string{previousDigest, digest}, "independentHostGroups": []string{first.HostGroup, run.HostGroup}}}
	receiptDigest, err := protocol.Digest(receipt)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO receipts(digest,payload,owner_id) VALUES($1,$2,$3)`, receiptDigest, raw(receipt), owner); err != nil {
		return err
	}
	if err = enqueue(ctx, tx, "sign_receipt", receiptDigest); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE preflights SET status='pass',findings='[]',reports=$2,machine_receipt_digest=$3 WHERE id=$1`, *preflight, raw(b), receiptDigest); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE challenge_versions SET status='review_ready' WHERE id=$1 AND lock_digest IS NULL`, version)
	return err
}
