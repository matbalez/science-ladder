package platform

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"io"
	"net/http"
	"strings"
	"time"
)

// A version transition closes old intake only after its last receipt resolves.
// Existing claims stay attached to their globally unique milestone identities.
func (s *Server) createVersion(w http.ResponseWriter, r *http.Request, u *User) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	var in struct {
		Repository        string `json:"repository"`
		Ref               string `json:"ref"`
		AdoptionStatement string `json:"adoptionStatement"`
		TransitionKind    string `json:"transitionKind"`
	}
	if err = readJSON(r, &in); err != nil {
		return err
	}
	if in.TransitionKind == "" {
		in.TransitionKind = "season"
	}
	if in.TransitionKind != "season" && in.TransitionKind != "security_migration" {
		return fail(422, "transition_invalid", "Use season or security_migration")
	}
	var priorID, priorIntake, priorFrontier string
	var oldNext, oldWatermark int64
	if err = s.DB.QueryRow(r.Context(), `SELECT v.id,v.intake_status,v.next_sequence,v.watermark,COALESCE(ss.artifact_digest,'') FROM challenge_versions v LEFT JOIN submissions ss ON ss.id=v.public_frontier_id JOIN challenges c ON c.id=v.challenge_id WHERE c.id=$1 AND c.owner_id=$2 ORDER BY v.created_at DESC LIMIT 1`, r.PathValue("id"), u.ID).Scan(&priorID, &priorIntake, &oldNext, &oldWatermark, &priorFrontier); err != nil {
		return err
	}
	if oldNext != oldWatermark {
		return fail(409, "prior_receipts_unresolved", "Resolve all accepted receipts before preparing a successor")
	}
	if in.TransitionKind == "security_migration" && priorIntake != "paused" && priorIntake != "closed" {
		return fail(409, "migration_pause_required", "An editor must pause the predecessor before a security migration")
	}
	var slug string
	if err = s.DB.QueryRow(r.Context(), `SELECT slug FROM challenges WHERE id=$1 AND owner_id=$2`, r.PathValue("id"), u.ID).Scan(&slug); err != nil {
		return err
	}
	if s.Store == nil {
		return fail(503, "storage_unavailable", "Immutable artifact storage must be configured")
	}
	if len(in.AdoptionStatement) < 20 {
		return fail(422, "adoption_required", "A revised creator adoption statement is required")
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
		return fail(422, "public_repository_required", "Challenge source must be public")
	}
	m, err := protocol.ParseManifest(snapshot.Files["science-ladder.yaml"])
	if err != nil {
		return fail(422, "manifest_invalid", err.Error())
	}
	if err = s.requireSuiteOwner(r.Context(), m, u); err != nil {
		return err
	}
	if m.Slug != slug {
		return fail(422, "slug_mismatch", "A new version must retain the challenge slug")
	}
	if !m.Deadline.After(time.Now()) {
		return fail(422, "deadline_passed", "The new version requires a future deadline")
	}
	if in.TransitionKind == "security_migration" && priorFrontier != "" {
		if err = verifyFrontierFixture(snapshot.Files, m, priorFrontier); err != nil {
			return err
		}
	}
	b := snapshotBytes(snapshot)
	source, err := s.Store.Put(r.Context(), b, "application/json")
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		ctx := r.Context()
		if err := lockSuiteOwner(ctx, tx, m, u); err != nil {
			return 0, nil, err
		}
		var challenge string
		if err := tx.QueryRow(ctx, `SELECT id FROM challenges WHERE id=$1 AND owner_id=$2 FOR NO KEY UPDATE`, r.PathValue("id"), u.ID).Scan(&challenge); err != nil {
			return 0, nil, err
		}
		var prior, priorStatus, intake, currentFrontier string
		var next, watermark int64
		err := tx.QueryRow(ctx, `SELECT v.id,v.status,v.next_sequence,v.watermark,v.intake_status,COALESCE((SELECT artifact_digest FROM submissions WHERE id=v.public_frontier_id),'') FROM challenge_versions v WHERE challenge_id=$1 ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, challenge).Scan(&prior, &priorStatus, &next, &watermark, &intake, &currentFrontier)
		if err != nil {
			return 0, nil, err
		}
		if prior != priorID || currentFrontier != priorFrontier {
			return 0, nil, fail(409, "predecessor_changed", "Predecessor changed while fetching source; restart the transition")
		}
		if in.TransitionKind == "security_migration" && intake != "paused" && intake != "closed" {
			return 0, nil, fail(409, "migration_pause_required", "Predecessor must remain paused")
		}
		if next != watermark {
			return 0, nil, fail(409, "prior_receipts_unresolved", "Resolve every accepted receipt before creating a successor version")
		}
		if priorStatus == "machine_preflight" {
			return 0, nil, fail(409, "preflight_running", "Wait for the current preflight to resolve before creating a successor")
		}
		version := ID()
		if _, err = tx.Exec(ctx, `INSERT INTO artifacts(digest,blob_digest,size,media_type,owner_id) VALUES($1,$1,$2,'application/json',$3) ON CONFLICT DO NOTHING`, source, len(b), u.ID); err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,deadline,predecessor_id,transition_kind,prior_frontier_digest) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, version, challenge, in.Repository, snapshot.RepositoryID, in.Ref, source, raw(m), m.Deadline, prior, in.TransitionKind, priorFrontier); err != nil {
			return 0, nil, err
		}
		for _, milestone := range m.Milestones {
			var oldChallenge string
			var claimed bool
			err = tx.QueryRow(ctx, `SELECT v.challenge_id,EXISTS(SELECT 1 FROM milestone_claims WHERE milestone_id=t.id) FROM milestone_tiers t JOIN challenge_versions v ON v.id=t.version_id WHERE t.id=$1`, milestone.ID).Scan(&oldChallenge, &claimed)
			if err == pgx.ErrNoRows {
				if _, err = tx.Exec(ctx, `INSERT INTO milestone_tiers(id,version_id,title,threshold_ticks) VALUES($1,$2,$3,$4)`, milestone.ID, version, milestone.Title, milestone.ThresholdTicks); err != nil {
					return 0, nil, err
				}
				continue
			}
			if err != nil {
				return 0, nil, err
			}
			if in.TransitionKind != "security_migration" {
				return 0, nil, fail(409, "new_milestone_ids_required", "Ordinary seasons require new global milestone IDs")
			}
			if oldChallenge != challenge || claimed {
				return 0, nil, fail(409, "milestone_not_transferable", "A claimed milestone or a milestone from another challenge cannot transfer or reopen")
			}
			// Existing milestone IDs transfer only after signed migration evidence
			// and successor conformance are ready, immediately before intake opens.
			var mapped bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM milestone_version_mappings WHERE milestone_id=$1 AND version_id=$2)`, milestone.ID, prior).Scan(&mapped); err != nil {
				return 0, nil, err
			}
			if !mapped {
				return 0, nil, fail(409, "milestone_predecessor_mismatch", "Transferred milestone must belong to the immediate predecessor")
			}

		}
		if in.TransitionKind == "season" {
			if _, err = tx.Exec(ctx, `UPDATE challenge_versions SET status='superseded',intake_status='closed' WHERE id=$1`, prior); err != nil {
				return 0, nil, err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE challenges SET adoption_statement=$2 WHERE id=$1`, challenge, in.AdoptionStatement); err != nil {
			return 0, nil, err
		}
		if err = audit(ctx, tx, version, "challenge.version_created", map[string]any{"versionId": version, "previousVersionId": prior, "previousWatermark": watermark, "economicMode": "none"}); err != nil {
			return 0, nil, err
		}
		return 201, map[string]any{"id": challenge, "slug": slug, "versionId": version, "status": "draft"}, nil
	})
}

func verifyFrontierFixture(files map[string][]byte, m protocol.Manifest, expected string) error {
	var fixture *protocol.Fixture
	for i := range m.Fixtures {
		if m.Fixtures[i].Name == "previous_frontier" {
			fixture = &m.Fixtures[i]
		}
	}
	if fixture == nil || fixture.ExpectedOutcome != "valid" || fixture.ExpectedTicks == "" {
		return fail(422, "frontier_fixture_required", "Security migration must reproduce a previous_frontier fixture with declared ticks")
	}
	selected := map[string][]byte{}
	prefix := strings.TrimSuffix(fixture.Path, "/") + "/"
	for path, b := range files {
		if strings.HasPrefix(path, prefix) {
			selected[strings.TrimPrefix(path, prefix)] = b
		}
	}
	_, digest, err := protocol.ArtifactFromFiles(selected, m.Submission)
	if err != nil || digest != expected {
		return fail(422, "frontier_fixture_mismatch", "previous_frontier fixture must contain the exact immutable public frontier artifact")
	}
	return nil
}

func (s *Server) stageMigration(r *http.Request, tx pgx.Tx, u *User, id string, m protocol.Manifest) (string, error) {
	ctx := r.Context()
	var prior, frontier, lock string
	if err := tx.QueryRow(ctx, `SELECT predecessor_id,COALESCE(prior_frontier_digest,''),lock_digest FROM challenge_versions WHERE id=$1`, id).Scan(&prior, &frontier, &lock); err != nil {
		return "", err
	}
	var next, watermark int64
	var intake, currentFrontier string
	if err := tx.QueryRow(ctx, `SELECT next_sequence,watermark,intake_status,COALESCE((SELECT artifact_digest FROM submissions WHERE id=v.public_frontier_id),'') FROM challenge_versions v WHERE id=$1 FOR UPDATE`, prior).Scan(&next, &watermark, &intake, &currentFrontier); err != nil {
		return "", err
	}
	if next != watermark || (intake != "paused" && intake != "closed") || frontier != currentFrontier {
		return "", fail(409, "migration_predecessor_changed", "Predecessor must be paused, fully resolved, and retain the checked public frontier")
	}
	var conformance string
	if err := tx.QueryRow(ctx, `SELECT machine_receipt_digest FROM preflights WHERE version_id=$1 AND status='pass' ORDER BY created_at DESC LIMIT 1`, id).Scan(&conformance); err != nil {
		return "", err
	}
	receipt := protocol.Receipt{APIVersion: protocol.APIVersion, Kind: "ChallengeMigrationReceipt", ID: ID(), CreatedAt: time.Now().UTC(), Producer: "science-ladder", DeploymentMode: s.Config.DeploymentMode, EconomicMode: "none", SubjectDigest: lock, Data: map[string]any{"versionId": id, "previousVersionId": prior, "oldFinalWatermark": formatInt(watermark), "priorFrontierDigest": frontier, "conformanceReceiptDigest": conformance, "milestones": m.Milestones, "policy": "pause-drain-reproduce-lock-transfer-v1"}}
	digest, err := protocol.Digest(receipt)
	if err != nil {
		return "", err
	}
	if err = saveReceipt(r, tx, digest, receipt, u.ID, false); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE challenge_versions SET status='migration_signing',migration_receipt_digest=$2 WHERE id=$1`, id, digest); err != nil {
		return "", err
	}
	return digest, enqueue(ctx, tx, "complete_migration", id)
}

func (s *Server) completeMigration(ctx context.Context, id string) error {
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var predecessor string
	if err = tx.QueryRow(ctx, `SELECT predecessor_id FROM challenge_versions WHERE id=$1`, id).Scan(&predecessor); err != nil {
		return err
	}
	var next, watermark int64
	var intake, frontier string
	if err = tx.QueryRow(ctx, `SELECT next_sequence,watermark,intake_status,COALESCE((SELECT artifact_digest FROM submissions WHERE id=v.public_frontier_id),'') FROM challenge_versions v WHERE id=$1 FOR UPDATE`, predecessor).Scan(&next, &watermark, &intake, &frontier); err != nil {
		return err
	}
	var status, priorFrontier, receipt string
	var manifest []byte
	var signed bool
	if err = tx.QueryRow(ctx, `SELECT status,COALESCE(prior_frontier_digest,''),manifest,migration_receipt_digest FROM challenge_versions WHERE id=$1 FOR UPDATE`, id).Scan(&status, &priorFrontier, &manifest, &receipt); err != nil {
		return err
	}
	if status == "published" {
		return nil
	}
	if status != "migration_signing" {
		return fail(409, "migration_state_invalid", "Successor is not ready to complete migration")
	}
	if next != watermark || (intake != "paused" && intake != "closed") || frontier != priorFrontier {
		return fail(409, "migration_predecessor_changed", "Predecessor state changed before transfer")
	}
	if err = tx.QueryRow(ctx, `SELECT envelope IS NOT NULL FROM receipts WHERE digest=$1`, receipt).Scan(&signed); err != nil {
		return err
	}
	if !signed {
		return fail(409, "migration_signature_pending", "Migration payload requires its committed signature before transfer")
	}
	var signedWatermark, signedPrior, signedSuccessor, signedFrontier string
	if err = tx.QueryRow(ctx, `SELECT payload->'data'->>'oldFinalWatermark',payload->'data'->>'previousVersionId',payload->'data'->>'versionId',payload->'data'->>'priorFrontierDigest' FROM receipts WHERE digest=$1`, receipt).Scan(&signedWatermark, &signedPrior, &signedSuccessor, &signedFrontier); err != nil {
		return err
	}
	if signedWatermark != formatInt(watermark) || signedPrior != predecessor || signedSuccessor != id || signedFrontier != frontier {
		return fail(409, "migration_evidence_changed", "Transfer state differs from the signed migration payload")
	}
	var m protocol.Manifest
	if err = json.Unmarshal(manifest, &m); err != nil {
		return err
	}
	for _, milestone := range m.Milestones {
		var origin string
		var claimed bool
		if err = tx.QueryRow(ctx, `SELECT version_id,EXISTS(SELECT 1 FROM milestone_claims WHERE milestone_id=t.id) FROM milestone_tiers t WHERE id=$1 FOR UPDATE`, milestone.ID).Scan(&origin, &claimed); err != nil {
			return err
		}
		if claimed {
			return fail(409, "milestone_already_claimed", "Claimed milestones cannot transfer")
		}
		if origin != id {
			var mapped bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM milestone_version_mappings WHERE milestone_id=$1 AND version_id=$2)`, milestone.ID, predecessor).Scan(&mapped); err != nil {
				return err
			}
			if !mapped {
				return fail(409, "milestone_predecessor_mismatch", "Transferred milestone does not belong to predecessor")
			}
			if _, err = tx.Exec(ctx, `INSERT INTO milestone_version_mappings(milestone_id,version_id,threshold_ticks) VALUES($1,$2,$3)`, milestone.ID, id, milestone.ThresholdTicks); err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE challenge_versions SET status='superseded',intake_status='closed' WHERE id=$1`, predecessor); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE receipts SET public=true WHERE digest=$1`, receipt); err != nil {
		return err
	}
	if err = publishVersion(ctx, tx, id, m); err != nil {
		return err
	}
	if err = audit(ctx, tx, id, "challenge.migrated", map[string]any{"versionId": id, "previousVersionId": predecessor, "migrationReceiptDigest": receipt, "previousWatermark": formatInt(watermark)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
