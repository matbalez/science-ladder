package platform

import (
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/http"
	"time"
)

type IntentRequest struct {
	VersionID            string         `json:"versionId"`
	Repository           string         `json:"repository"`
	Ref                  string         `json:"ref"`
	PreviewDigest        string         `json:"previewDigest,omitempty"`
	ParentFrontierDigest string         `json:"parentFrontierDigest,omitempty"`
	License              string         `json:"license"`
	Attribution          map[string]any `json:"attribution"`
	Publish              bool           `json:"publish"`
}

func (s *Server) createIntent(w http.ResponseWriter, r *http.Request, u *User) error {
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var in IntentRequest
		if err := readJSON(r, &in); err != nil {
			return 0, nil, err
		}
		if !repoRE.MatchString(in.Repository) || !commitRE.MatchString(in.Ref) {
			return 0, nil, fail(422, "exact_commit_required", "Use owner/repository and the full remote Git commit SHA")
		}
		var manifest []byte
		var open bool
		if err := tx.QueryRow(r.Context(), `SELECT manifest,intake_status='open' AND deadline>now() FROM challenge_versions WHERE id=$1 AND status='published'`, in.VersionID).Scan(&manifest, &open); err != nil {
			return 0, nil, err
		}
		if !open {
			return 0, nil, fail(409, "intake_closed", "Challenge is not accepting submissions")
		}
		var m protocol.Manifest
		if err := json.Unmarshal(manifest, &m); err != nil {
			return 0, nil, err
		}
		if in.License != m.Submission.License {
			return 0, nil, fail(422, "license_mismatch", "Submission must use the locked challenge artifact license")
		}
		for k, value := range in.Attribution {
			switch k {
			case "model", "harness", "disclosure":
				text, ok := value.(string)
				if !ok || len(text) > 2000 {
					return 0, nil, fail(422, "attribution_invalid", "Attribution values must be text of at most 2,000 characters")
				}
			case "platformSeeded":
				if _, ok := value.(bool); !ok {
					return 0, nil, fail(422, "attribution_invalid", "platformSeeded must be a boolean")
				}
			default:
				return 0, nil, fail(422, "attribution_invalid", "Use model, harness, disclosure, and platformSeeded attribution fields")
			}
		}
		if in.Attribution == nil {
			in.Attribution = map[string]any{}
		}
		if _, err := tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1,1))`, u.ID+in.VersionID+in.Repository+in.Ref); err != nil {
			return 0, nil, err
		}
		var existingID, existingStatus string
		existingErr := tx.QueryRow(r.Context(), `SELECT id,status FROM submission_intents WHERE owner_id=$1 AND version_id=$2 AND repository=$3 AND ref=$4`, u.ID, in.VersionID, in.Repository, in.Ref).Scan(&existingID, &existingStatus)
		if existingErr == nil {
			return 200, map[string]any{"id": existingID, "versionId": in.VersionID, "repository": in.Repository, "status": existingStatus, "findings": []Finding{}}, nil
		}
		if existingErr != pgx.ErrNoRows {
			return 0, nil, existingErr
		}
		if err := s.reservePreparation(r.Context(), tx, u); err != nil {
			return 0, nil, err
		}
		id := ID()
		_, err := tx.Exec(r.Context(), `INSERT INTO submission_intents(id,version_id,owner_id,repository,ref,request) VALUES($1,$2,$3,$4,$5,$6)`, id, in.VersionID, u.ID, in.Repository, in.Ref, raw(in))
		if err != nil {
			return 0, nil, err
		}
		if err = enqueue(r.Context(), tx, "fetch_submission", id); err != nil {
			return 0, nil, err
		}
		return 202, map[string]any{"id": id, "versionId": in.VersionID, "repository": in.Repository, "status": "github_fetch", "findings": []Finding{}}, nil
	})
}
func (s *Server) acceptIntent(w http.ResponseWriter, r *http.Request, u *User) error {
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		ctx := r.Context()
		if err := s.checkProductionAdmission(ctx, tx); err != nil {
			return 0, nil, err
		}
		id := r.PathValue("id")
		var version, artifact, disk, status string
		var request []byte
		var existing *string
		err := tx.QueryRow(ctx, `SELECT version_id,COALESCE(artifact_digest,''),COALESCE(disk_digest,''),status,request,submission_id FROM submission_intents WHERE id=$1 AND owner_id=$2 FOR UPDATE`, id, u.ID).Scan(&version, &artifact, &disk, &status, &request, &existing)
		if err != nil {
			return 0, nil, err
		}
		if existing != nil {
			return 200, map[string]any{"submissionId": *existing, "status": "accepted"}, nil
		}
		if status != "ready" || artifact == "" || disk == "" {
			return 0, nil, fail(409, "artifact_not_ready", "GitHub snapshot, safety scan, and isolated disk preparation must finish before acceptance")
		}
		var manifest []byte
		var lockDigest, intake string
		var deadline time.Time
		var sequence int64
		err = tx.QueryRow(ctx, `SELECT manifest,lock_digest,intake_status,deadline,next_sequence FROM challenge_versions WHERE id=$1 FOR UPDATE`, version).Scan(&manifest, &lockDigest, &intake, &deadline, &sequence)
		if err != nil {
			return 0, nil, err
		}
		if intake != "open" || !deadline.After(time.Now()) {
			return 0, nil, fail(409, "intake_closed", "The challenge intake is closed or its deadline has passed")
		}
		var quota int
		err = tx.QueryRow(ctx, `SELECT validation_quota FROM users WHERE id=$1 FOR NO KEY UPDATE`, u.ID).Scan(&quota)
		if err != nil {
			return 0, nil, err
		}
		if quota <= 0 {
			return 0, nil, fail(429, "quota_exhausted", "No validation grants remain for this account")
		}
		var active int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM submissions WHERE owner_id=$1 AND status<>'finalized'`, u.ID).Scan(&active); err != nil {
			return 0, nil, err
		}
		if active >= s.Config.ActiveLimit {
			return 0, nil, fail(429, "active_limit", "Resolve an active submission before accepting another")
		}
		var groups int
		if err = tx.QueryRow(ctx, `SELECT count(DISTINCT host_group) FROM runner_hosts WHERE enabled`).Scan(&groups); err != nil {
			return 0, nil, err
		}
		if groups < 2 {
			return 0, nil, fail(503, "independent_runners_unavailable", "Official acceptance requires two trusted independent runner groups")
		}
		var duplicate, owner string
		err = tx.QueryRow(ctx, `SELECT id,owner_id FROM submissions WHERE version_id=$1 AND artifact_digest=$2`, version, artifact).Scan(&duplicate, &owner)
		if err == nil {
			if owner == u.ID {
				return 200, map[string]any{"submissionId": duplicate, "status": "accepted"}, nil
			}
			return 0, nil, fail(409, "artifact_duplicate", "This artifact has already been accepted")
		}
		if err != pgx.ErrNoRows {
			return 0, nil, err
		}
		tag, err := tx.Exec(ctx, `UPDATE capacity SET reserved_units=reserved_units+2 WHERE id=1 AND reserved_units+2<=maximum_units`)
		if err != nil {
			return 0, nil, err
		}
		if tag.RowsAffected() != 1 {
			return 0, nil, fail(503, "capacity_unavailable", "Primary and independent confirmation capacity are not available; no sequence has been assigned")
		}
		var m protocol.Manifest
		if err = json.Unmarshal(manifest, &m); err != nil {
			return 0, nil, err
		}
		var in IntentRequest
		if err = json.Unmarshal(request, &in); err != nil {
			return 0, nil, err
		}
		grant, submission := ID(), ID()
		sequence++
		created := time.Now().UTC()
		salt := secret()
		commitment := hash(salt + "\x00" + artifact)
		receipt := protocol.Receipt{DeploymentMode: s.Config.DeploymentMode, OfficialAcceptance: false, APIVersion: protocol.APIVersion, Kind: "SubmissionAcceptanceReceipt", ID: ID(), CreatedAt: created, Producer: "science-ladder", SubjectDigest: artifact, EconomicMode: "none", Data: map[string]any{"submissionId": submission, "versionId": version, "challengeLockDigest": lockDigest, "sequence": formatInt(sequence), "grantId": grant, "artifactDigest": artifact, "submissionDiskDigest": disk, "resourceClass": m.Resources.Class, "parentFrontierDigest": in.ParentFrontierDigest, "attribution": in.Attribution}}
		digest, err := protocol.Digest(receipt)
		if err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO validation_grants(id,owner_id,version_id,artifact_digest,resource_class,units,status) VALUES($1,$2,$3,$4,$5,2,'reserved')`, grant, u.ID, version, artifact, m.Resources.Class); err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO submissions(id,version_id,owner_id,intent_id,artifact_digest,disk_digest,sequence,publish_requested,attribution,receipt_digest,commitment,commitment_salt,grant_id,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, submission, version, u.ID, id, artifact, disk, sequence, in.Publish, raw(in.Attribution), digest, commitment, salt, grant, created); err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO capacity_reservations(submission_id) VALUES($1)`, submission); err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE users SET validation_quota=validation_quota-1 WHERE id=$1`, u.ID); err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE challenge_versions SET next_sequence=$2 WHERE id=$1`, version, sequence); err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE submission_intents SET status='accepted',submission_id=$2 WHERE id=$1`, id, submission); err != nil {
			return 0, nil, err
		}
		if err = saveReceipt(r, tx, digest, receipt, u.ID, false); err != nil {
			return 0, nil, err
		}
		if err = enqueue(ctx, tx, "validate_submission", submission); err != nil {
			return 0, nil, err
		}
		if err = audit(ctx, tx, version, "submission.accepted", map[string]any{"sequence": formatInt(sequence), "commitment": commitment, "economicMode": "none"}); err != nil {
			return 0, nil, err
		}
		return 201, map[string]any{"submissionId": submission, "sequence": sequence, "receiptDigest": digest, "status": "accepted"}, nil
	})
}
func (s *Server) publishSubmission(w http.ResponseWriter, r *http.Request, u *User) error {
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var status, outcome, version string
		err := tx.QueryRow(r.Context(), `SELECT status,outcome,version_id FROM submissions WHERE id=$1 AND owner_id=$2 FOR UPDATE`, r.PathValue("id"), u.ID).Scan(&status, &outcome, &version)
		if err != nil {
			return 0, nil, err
		}
		if status != "finalized" || outcome != "valid" {
			return 0, nil, fail(409, "publication_not_ready", "Only a finalized valid artifact can be voluntarily published")
		}
		if _, err = tx.Exec(r.Context(), `UPDATE submissions SET public=true,publish_requested=true WHERE id=$1`, r.PathValue("id")); err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE artifacts SET public_at=COALESCE(public_at,now()) WHERE digest IN(SELECT artifact_digest FROM submissions WHERE id=$1 UNION SELECT disk_digest FROM submissions WHERE id=$1)`, r.PathValue("id")); err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE receipts SET public=true WHERE digest IN(SELECT receipt_digest FROM submissions WHERE id=$1 UNION SELECT adjudication_digest FROM submissions WHERE id=$1)`, r.PathValue("id")); err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE receipts SET public=true WHERE digest IN(SELECT rr.digest FROM runner_results rr JOIN runner_jobs j ON j.id=rr.job_id WHERE j.submission_id=$1)`, r.PathValue("id")); err != nil {
			return 0, nil, err
		}
		if err = audit(r.Context(), tx, version, "solution.published", map[string]any{"submissionId": r.PathValue("id")}); err != nil {
			return 0, nil, err
		}
		return 200, map[string]any{"id": r.PathValue("id"), "public": true}, nil
	})
}
