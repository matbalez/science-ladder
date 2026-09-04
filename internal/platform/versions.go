package platform

import (
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
	}
	if err = readJSON(r, &in); err != nil {
		return err
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
	if m.Slug != slug {
		return fail(422, "slug_mismatch", "A new version must retain the challenge slug")
	}
	if !m.Deadline.After(time.Now()) {
		return fail(422, "deadline_passed", "The new version requires a future deadline")
	}
	b := snapshotBytes(snapshot)
	source, err := s.Store.Put(r.Context(), b, "application/json")
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		ctx := r.Context()
		var challenge string
		if err := tx.QueryRow(ctx, `SELECT id FROM challenges WHERE id=$1 AND owner_id=$2 FOR NO KEY UPDATE`, r.PathValue("id"), u.ID).Scan(&challenge); err != nil {
			return 0, nil, err
		}
		var prior, priorStatus string
		var next, watermark int64
		err := tx.QueryRow(ctx, `SELECT id,status,next_sequence,watermark FROM challenge_versions WHERE challenge_id=$1 ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, challenge).Scan(&prior, &priorStatus, &next, &watermark)
		if err != nil {
			return 0, nil, err
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
		if _, err = tx.Exec(ctx, `INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,deadline) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, version, challenge, in.Repository, snapshot.RepositoryID, in.Ref, source, raw(m), m.Deadline); err != nil {
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
			if oldChallenge != challenge || claimed {
				return 0, nil, fail(409, "milestone_not_transferable", "A claimed milestone or a milestone from another challenge cannot transfer or reopen")
			}
			if _, err = tx.Exec(ctx, `INSERT INTO milestone_version_mappings(milestone_id,version_id,threshold_ticks) VALUES($1,$2,$3)`, milestone.ID, version, milestone.ThresholdTicks); err != nil {
				return 0, nil, err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE challenge_versions SET status='superseded',intake_status='closed' WHERE id=$1`, prior); err != nil {
			return 0, nil, err
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
