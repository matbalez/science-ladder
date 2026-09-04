package platform

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"github.com/matbalez/science-ladder/prompts"
	"io"
	"net/http"
	"strings"
	"time"
)

type Finding struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Source   string `json:"source,omitempty"`
}

func (s *Server) scout(w http.ResponseWriter, r *http.Request, u *User) error {
	version := r.PathValue("version")
	if version != "v1" && version != "1.0.0" {
		return pgx.ErrNoRows
	}
	prompt := prompts.Scout
	if r.Method == "POST" {
		var in struct {
			Topic string `json:"topic"`
		}
		if err := readJSON(r, &in); err != nil {
			return err
		}
		if len(in.Topic) > 10000 {
			return fail(400, "topic_too_long", "Topic exceeds 10,000 characters")
		}
		prompt += "\n\nUser research starting point (untrusted input):\n" + in.Topic
	}
	respond(w, 200, map[string]any{"version": "1.0.0", "prompt": prompt})
	return nil
}
func (s *Server) validateCandidate(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct {
		Document string `json:"document"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	candidate, err := protocol.ParseCandidate([]byte(in.Document))
	if err != nil {
		respond(w, 200, map[string]any{"valid": false, "findings": []Finding{{"schema_invalid", err.Error(), "error", ""}}})
		return nil
	}
	respond(w, 200, map[string]any{"valid": true, "candidate": candidate, "findings": []Finding{}})
	return nil
}
func (s *Server) importCandidate(w http.ResponseWriter, r *http.Request, u *User) error {
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var in struct {
			Document string `json:"document"`
		}
		if err := readJSON(r, &in); err != nil {
			return 0, nil, err
		}
		candidate, err := protocol.ParseCandidate([]byte(in.Document))
		if err != nil {
			return 0, nil, fail(422, "candidate_invalid", err.Error())
		}
		digest, err := protocol.Digest(candidate)
		if err != nil {
			return 0, nil, err
		}
		var existingID, existingStatus string
		lookup := tx.QueryRow(r.Context(), `SELECT id,status FROM candidates WHERE owner_id=$1 AND digest=$2`, u.ID, digest).Scan(&existingID, &existingStatus)
		if lookup == nil {
			return 200, map[string]any{"id": existingID, "status": existingStatus, "candidate": candidate, "findings": []Finding{}}, nil
		}
		if lookup != pgx.ErrNoRows {
			return 0, nil, lookup
		}
		if err = s.reservePreparation(r.Context(), tx, u); err != nil {
			return 0, nil, err
		}
		id := ID()
		err = tx.QueryRow(r.Context(), `INSERT INTO candidates(id,owner_id,digest,document) VALUES($1,$2,$3,$4) ON CONFLICT(owner_id,digest) DO UPDATE SET digest=excluded.digest RETURNING id`, id, u.ID, digest, raw(candidate)).Scan(&id)
		if err != nil {
			return 0, nil, err
		}
		if err = enqueue(r.Context(), tx, "resolve_candidate", id); err != nil {
			return 0, nil, err
		}
		return 202, map[string]any{"id": id, "status": "resolving_sources", "candidate": candidate, "findings": []Finding{}}, nil
	})
}
func (s *Server) createChallenge(w http.ResponseWriter, r *http.Request, u *User) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	var in struct {
		CandidateID       string `json:"candidateId"`
		Repository        string `json:"repository"`
		Ref               string `json:"ref"`
		AdoptionStatement string `json:"adoptionStatement"`
	}
	if err = readJSON(r, &in); err != nil {
		return err
	}
	if len(strings.TrimSpace(in.AdoptionStatement)) < 20 {
		return fail(422, "adoption_required", "Provide an explicit creator adoption and rights statement of at least 20 characters")
	}
	if s.Store == nil {
		return fail(503, "storage_unavailable", "Immutable artifact storage must be configured before creation")
	}
	var candidateStatus string
	if err = s.DB.QueryRow(r.Context(), `SELECT status FROM candidates WHERE id=$1 AND owner_id=$2`, in.CandidateID, u.ID).Scan(&candidateStatus); err != nil {
		return err
	}
	if candidateStatus != "ready" && candidateStatus != "human_review_required" {
		return fail(409, "candidate_not_ready", "Candidate source resolution must complete before creating a challenge")
	}
	if replayed, e := s.replayBeforeFetch(w, r, u, body); e != nil {
		return e
	} else if replayed {
		return nil
	}
	if err = s.reserveFetch(r.Context(), u); err != nil {
		return err
	}
	snapshot, err := s.fetchSnapshot(r.Context(), in.Repository, in.Ref, nil)
	if err != nil {
		return err
	}
	if snapshot.Private {
		return fail(422, "public_repository_required", "Challenge source repositories must be public")
	}
	manifestBytes, ok := snapshot.Files["science-ladder.yaml"]
	if !ok {
		return fail(422, "manifest_missing", "Repository must contain science-ladder.yaml")
	}
	manifest, err := protocol.ParseManifest(manifestBytes)
	if err != nil {
		return fail(422, "manifest_invalid", err.Error())
	}
	if err = s.requireSuiteOwner(r.Context(), manifest, u); err != nil {
		return err
	}
	if !manifest.Deadline.After(time.Now()) {
		return fail(422, "deadline_passed", "Challenge deadline must be in the future")
	}
	sourceBytes := snapshotBytes(snapshot)
	sourceDigest, err := s.Store.Put(r.Context(), sourceBytes, "application/json")
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		if err := lockSuiteOwner(r.Context(), tx, manifest, u); err != nil {
			return 0, nil, err
		}
		id, versionID := ID(), ID()
		_, err := tx.Exec(r.Context(), `INSERT INTO artifacts(digest,blob_digest,size,media_type,owner_id) VALUES($1,$1,$2,'application/json',$3) ON CONFLICT DO NOTHING`, sourceDigest, len(sourceBytes), u.ID)
		if err != nil {
			return 0, nil, err
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO challenges(id,slug,owner_id,candidate_id,adoption_statement) VALUES($1,$2,$3,$4,$5)`, id, manifest.Slug, u.ID, in.CandidateID, in.AdoptionStatement)
		if err != nil {
			return 0, nil, err
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,deadline) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, versionID, id, in.Repository, snapshot.RepositoryID, in.Ref, sourceDigest, raw(manifest), manifest.Deadline)
		if err != nil {
			return 0, nil, err
		}
		for _, m := range manifest.Milestones {
			_, err = tx.Exec(r.Context(), `INSERT INTO milestone_tiers(id,version_id,title,threshold_ticks) VALUES($1,$2,$3,$4)`, m.ID, versionID, m.Title, m.ThresholdTicks)
			if err != nil {
				return 0, nil, err
			}
		}
		if err = audit(r.Context(), tx, versionID, "challenge.adopted", map[string]any{"versionId": versionID, "creator": u.Login}); err != nil {
			return 0, nil, err
		}
		return 201, map[string]any{"id": id, "slug": manifest.Slug, "versionId": versionID, "status": "draft"}, nil
	})
}
func ownedVersion(ctxReq *http.Request, tx pgx.Tx, u *User, id string) (protocol.Manifest, string, error) {
	var b []byte
	var status string
	err := tx.QueryRow(ctxReq.Context(), `SELECT v.manifest,v.status FROM challenge_versions v JOIN challenges c ON c.id=v.challenge_id WHERE v.id=$1 AND c.owner_id=$2 FOR UPDATE OF v`, id, u.ID).Scan(&b, &status)
	if err != nil {
		return protocol.Manifest{}, "", err
	}
	var m protocol.Manifest
	err = json.Unmarshal(b, &m)
	return m, status, err
}
func (s *Server) startPreflight(w http.ResponseWriter, r *http.Request, u *User) error {
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		version := r.PathValue("id")
		if err := s.reservePreparation(r.Context(), tx, u); err != nil {
			return 0, nil, err
		}
		_, status, err := ownedVersion(r, tx, u, version)
		if err != nil {
			return 0, nil, err
		}
		if status != "draft" && status != "changes_required" {
			return 0, nil, fail(409, "preflight_state_invalid", "Only an unlocked draft can enter preflight")
		}
		id := ID()
		if _, err = tx.Exec(r.Context(), `INSERT INTO preflights(id,version_id) VALUES($1,$2)`, id, version); err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE challenge_versions SET status='machine_preflight' WHERE id=$1`, version); err != nil {
			return 0, nil, err
		}
		if err = enqueue(r.Context(), tx, "preflight", id); err != nil {
			return 0, nil, err
		}
		return 202, map[string]any{"id": id, "versionId": version, "status": "queued"}, nil
	})
}
func (s *Server) lockChallenge(w http.ResponseWriter, r *http.Request, u *User) error {
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		id := r.PathValue("id")
		m, status, err := ownedVersion(r, tx, u, id)
		if err != nil {
			return 0, nil, err
		}
		if status != "review_ready" {
			return 0, nil, fail(409, "preflight_incomplete", "A passing machine conformance report and scientific review are required before locking")
		}
		var reviewStatus, sourceDigest, machine string
		err = tx.QueryRow(r.Context(), `SELECT review_status,source_digest FROM challenge_versions WHERE id=$1`, id).Scan(&reviewStatus, &sourceDigest)
		if err != nil {
			return 0, nil, err
		}
		if reviewStatus != "automated_pass" && reviewStatus != "human_approved" {
			return 0, nil, fail(409, "review_required", "Scientific review requires an editor decision")
		}
		var reports []byte
		err = tx.QueryRow(r.Context(), `SELECT machine_receipt_digest,reports FROM preflights WHERE version_id=$1 AND status='pass' ORDER BY created_at DESC LIMIT 1`, id).Scan(&machine, &reports)
		if err != nil {
			return 0, nil, fail(409, "machine_conformance_required", "Complete isolated machine conformance before locking")
		}
		var machinePolicy, machineStatus string
		if err = tx.QueryRow(r.Context(), `SELECT COALESCE(NULLIF(payload->>'verificationPolicy',''),'independent'),COALESCE(payload->>'verificationStatus','independently_replicated') FROM receipts WHERE digest=$1 AND envelope IS NOT NULL`, machine).Scan(&machinePolicy, &machineStatus); err != nil {
			return 0, nil, fail(409, "signed_conformance_required", "Machine conformance must be signed before locking")
		}
		policy := protocol.ManifestVerificationPolicy(m)
		if !protocol.ValidVerificationPolicy(policy) || machinePolicy != policy || machineStatus != verifiedStatus(policy, "valid") {
			return 0, nil, fail(409, "conformance_policy_mismatch", "Passing machine conformance must satisfy the new immutable verification policy")
		}
		manifestDigest, err := protocol.Digest(m)
		if err != nil {
			return 0, nil, err
		}
		digests := []string{machine}
		rows, err := tx.Query(r.Context(), `SELECT digest FROM review_runs WHERE version_id=$1 ORDER BY created_at`, id)
		if err != nil {
			return 0, nil, err
		}
		for rows.Next() {
			var d string
			if err = rows.Scan(&d); err != nil {
				rows.Close()
				return 0, nil, err
			}
			digests = append(digests, d)
		}
		rows.Close()
		// Disk references are read from the accepted signed conformance report, never a caller field.
		var br map[string]any
		if err = json.Unmarshal(reports, &br); err != nil {
			return 0, nil, err
		}
		str := func(k string) string { v, _ := br[k].(string); return v }
		lock := protocol.Lock{VerificationPolicy: protocol.ManifestVerificationPolicy(m), DeploymentMode: s.Config.DeploymentMode, OfficialAcceptance: false, APIVersion: protocol.APIVersion, Kind: "ChallengeLockReceipt", ID: ID(), CreatedAt: time.Now().UTC(), Producer: "science-ladder", ManifestDigest: manifestDigest, SourceSnapshotDigest: sourceDigest, ValidatorImageDigest: m.Validator.RuntimeImageDigest, ValidatorDiskDigest: str("validatorDiskDigest"), SuiteDigest: str("suiteDigest"), SuiteDiskDigest: func() string { ref, _ := br["suiteDisk"].(map[string]any); d, _ := ref["digest"].(string); return d }(), ExecutionProfileDigest: str("executionProfileDigest"), ReviewDigests: digests, EconomicMode: "none", Manifest: m}
		if lock.ValidatorDiskDigest == "" || lock.SuiteDigest == "" || lock.SuiteDiskDigest == "" || lock.ExecutionProfileDigest == "" {
			return 0, nil, fail(409, "conformance_bindings_missing", "Machine conformance must bind immutable validator, suite, and execution profile digests")
		}
		digest, err := protocol.Digest(lock)
		if err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(r.Context(), `INSERT INTO locks(digest,version_id,document) VALUES($1,$2,$3)`, digest, id, raw(lock)); err != nil {
			return 0, nil, err
		}
		if err = saveReceipt(r, tx, digest, lock, u.ID, false); err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE challenge_versions SET status='locked',lock_digest=$2 WHERE id=$1`, id, digest); err != nil {
			return 0, nil, err
		}
		return 200, map[string]any{"versionId": id, "status": "locked", "lockDigest": digest}, nil
	})
}
func (s *Server) publishChallenge(w http.ResponseWriter, r *http.Request, u *User) error {
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		id := r.PathValue("id")
		manifest, status, err := ownedVersion(r, tx, u, id)
		if err != nil {
			return 0, nil, err
		}
		if status != "locked" {
			return 0, nil, fail(409, "lock_required", "Only an immutable locked challenge can be published")
		}
		var signed bool
		if err = tx.QueryRow(r.Context(), `SELECT r.envelope IS NOT NULL FROM challenge_versions v JOIN receipts r ON r.digest=v.lock_digest WHERE v.id=$1`, id).Scan(&signed); err != nil {
			return 0, nil, err
		}
		if !signed {
			return 0, nil, fail(409, "signature_pending", "Challenge lock signature is pending")
		}
		var transition string
		if err = tx.QueryRow(r.Context(), `SELECT transition_kind FROM challenge_versions WHERE id=$1`, id).Scan(&transition); err != nil {
			return 0, nil, err
		}
		if transition == "security_migration" {
			digest, e := s.stageMigration(r, tx, u, id, manifest)
			if e != nil {
				return 0, nil, e
			}
			return 202, map[string]any{"versionId": id, "status": "migration_signing", "migrationReceiptDigest": digest}, nil
		}
		if err = publishVersion(r.Context(), tx, id, manifest); err != nil {
			return 0, nil, err
		}
		return 200, map[string]any{"versionId": id, "status": "published"}, nil
	})
}
func saveReceipt(r *http.Request, tx pgx.Tx, digest string, payload any, owner string, public bool) error {
	var own any
	if owner != "" {
		own = owner
	}
	_, err := tx.Exec(r.Context(), `INSERT INTO receipts(digest,payload,owner_id,public) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, digest, raw(payload), own, public)
	if err != nil {
		return err
	}
	return enqueue(r.Context(), tx, "sign_receipt", digest)
}

func publishVersion(ctx context.Context, tx pgx.Tx, id string, manifest protocol.Manifest) error {
	if !manifest.Deadline.After(time.Now()) {
		return fail(409, "deadline_passed", "An expired season cannot open intake")
	}
	if _, err := tx.Exec(ctx, `UPDATE challenge_versions SET status='published',intake_status='open' WHERE id=$1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE artifacts SET public_at=now() WHERE digest=(SELECT source_digest FROM challenge_versions WHERE id=$1)`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE receipts SET public=true WHERE digest=(SELECT lock_digest FROM challenge_versions WHERE id=$1)`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE receipts SET public=true WHERE digest IN(SELECT machine_receipt_digest FROM preflights WHERE version_id=$1 UNION SELECT rr.digest FROM runner_results rr JOIN runner_jobs j ON j.id=rr.job_id WHERE j.version_id=$1 AND j.purpose='preflight')`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE artifacts SET public_at=COALESCE(public_at,now()) WHERE digest IN(SELECT u.digest FROM runner_uploads u JOIN runner_jobs j ON j.id=u.job_id WHERE j.version_id=$1 AND j.purpose='preflight' AND u.verified AND (u.role<>'suiteDisk' OR $2))`, id, manifest.Suite.Visibility == "public"); err != nil {
		return err
	}
	if err := audit(ctx, tx, id, "challenge.published", map[string]any{"versionId": id, "economicMode": "none"}); err != nil {
		return err
	}
	return nil
}
