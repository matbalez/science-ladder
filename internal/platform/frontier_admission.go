package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

type frontierPolicy struct {
	Manifest      protocol.Manifest `json:"manifest"`
	LockDigest    string            `json:"lockDigest"`
	FrontierTicks string            `json:"frontierTicks"`
	Mode          string            `json:"mode"`
}
type claimRequest struct {
	Repository string                 `json:"repository"`
	Ref        string                 `json:"ref"`
	Claim      protocol.FrontierClaim `json:"claim"`
}

func loadFrontierPolicy(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, version string) (frontierPolicy, error) {
	var p frontierPolicy
	var data []byte
	var public string
	var open bool
	err := q.QueryRow(ctx, `SELECT v.manifest,v.lock_digest,COALESCE(s.score_ticks::text,''),v.intake_status='open' AND v.deadline>now() FROM challenge_versions v LEFT JOIN submissions s ON s.id=v.public_frontier_id WHERE v.id=$1 AND v.status='published'`, version).Scan(&data, &p.LockDigest, &public, &open)
	if err != nil {
		return p, err
	}
	if !open {
		return p, fail(409, "intake_closed", "Challenge is not accepting submissions")
	}
	if err = json.Unmarshal(data, &p.Manifest); err != nil {
		return p, err
	}
	p.FrontierTicks = protocol.AdmissionFrontier(p.Manifest.Metric, public)
	p.Mode = protocol.AdmissionMode(p.Manifest)
	return p, nil
}
func (s *Server) getFrontierPolicy(w http.ResponseWriter, r *http.Request, u *User) error {
	p, err := loadFrontierPolicy(r.Context(), s.DB, r.PathValue("id"))
	if err != nil {
		return err
	}
	respond(w, 200, p)
	return nil
}
func checkFrontierClaim(c protocol.FrontierClaim, p frontierPolicy) error {
	if c.LockDigest != p.LockDigest {
		return fail(409, "claim_lock_mismatch", "Revalidate against the frozen challenge version")
	}
	ticks, err := c.Validate(p.Manifest)
	if err != nil {
		return fail(422, "local_claim_invalid", err.Error())
	}
	if !protocol.FrontierImprovement(ticks, p.FrontierTicks, p.Manifest.Metric) {
		return fail(409, "not_frontier_potential", "Local result must improve the current public frontier by the required meaningful delta; keep searching locally")
	}
	return nil
}
func (s *Server) createFrontierTicket(w http.ResponseWriter, r *http.Request, u *User) error {
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var in claimRequest
		if err := readJSON(r, &in); err != nil {
			return 0, nil, err
		}
		if !repoRE.MatchString(in.Repository) || !commitRE.MatchString(in.Ref) {
			return 0, nil, fail(422, "exact_commit_required", "Use owner/repository and the full remote Git commit SHA")
		}
		ctx := r.Context()
		// Repeating the exact consumed claim resumes its existing intent; it never authorizes another job.
		var resumedID, resumedMode string
		var resumedExpiry time.Time
		resumeErr := tx.QueryRow(ctx, `SELECT id,claim->>'mode',expires_at FROM frontier_tickets WHERE owner_id=$1 AND version_id=$2 AND repository=$3 AND ref=$4 AND claim=$5 AND consumed_at IS NOT NULL AND intent_id IS NOT NULL ORDER BY created_at DESC LIMIT 1`, u.ID, in.Claim.VersionID, in.Repository, in.Ref, raw(in.Claim)).Scan(&resumedID, &resumedMode, &resumedExpiry)
		if resumeErr == nil {
			return 200, map[string]any{"ticketId": resumedID, "expiresAt": resumedExpiry, "mode": resumedMode, "resumeOnly": true}, nil
		}
		if resumeErr != pgx.ErrNoRows {
			return 0, nil, resumeErr
		}
		p, err := loadFrontierPolicy(ctx, tx, in.Claim.VersionID)
		if err != nil {
			return 0, nil, err
		}
		if err = checkFrontierClaim(in.Claim, p); err != nil {
			return 0, nil, err
		}
		// Serializes issuance across API machines, including global and per-owner limits.
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(6842077295)`); err != nil {
			return 0, nil, err
		}
		var quota int
		if err = tx.QueryRow(ctx, `SELECT validation_quota FROM users WHERE id=$1`, u.ID).Scan(&quota); err != nil {
			return 0, nil, err
		}
		if quota <= 0 {
			return 0, nil, fail(429, "quota_exhausted", "No validation grants remain")
		}
		var duplicate bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM submissions WHERE version_id=$1 AND artifact_digest=$2) OR EXISTS(SELECT 1 FROM frontier_tickets WHERE owner_id=$3 AND version_id=$1 AND artifact_digest=$2 AND consumed_at IS NOT NULL)`, in.Claim.VersionID, in.Claim.ArtifactDigest, u.ID).Scan(&duplicate); err != nil {
			return 0, nil, err
		}
		if duplicate {
			return 0, nil, fail(409, "artifact_duplicate", "This candidate has already entered verification; inspect its existing intent or submission")
		}
		var id string
		var expires time.Time
		err = tx.QueryRow(ctx, `SELECT id,expires_at FROM frontier_tickets WHERE owner_id=$1 AND version_id=$2 AND repository=$3 AND ref=$4 AND claim=$5 AND consumed_at IS NULL AND expires_at>now() ORDER BY created_at DESC LIMIT 1`, u.ID, in.Claim.VersionID, in.Repository, in.Ref, raw(in.Claim)).Scan(&id, &expires)
		if err == nil {
			return 200, map[string]any{"ticketId": id, "expiresAt": expires, "mode": p.Mode}, nil
		}
		if err != pgx.ErrNoRows {
			return 0, nil, err
		}
		var daily, global, qualification, active int
		err = tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE owner_id=$1),count(*),count(*) FILTER(WHERE owner_id=$1 AND claim->>'mode'='qualification'),count(*) FILTER(WHERE owner_id=$1 AND consumed_at IS NULL AND expires_at>now()) FROM frontier_tickets WHERE created_at>now()-interval '24 hours'`, u.ID).Scan(&daily, &global, &qualification, &active)
		if err != nil {
			return 0, nil, err
		}
		if daily >= 5 || global >= 100 || active >= quota || active >= 3 || p.Mode == "qualification" && qualification >= 2 {
			return 0, nil, fail(429, "frontier_admission_budget", "Verification admission budget reached; continue local work and inspect existing submissions")
		}
		id = ID()
		err = tx.QueryRow(ctx, `INSERT INTO frontier_tickets(id,owner_id,version_id,repository,ref,artifact_digest,claim,frontier_ticks) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING expires_at`, id, u.ID, in.Claim.VersionID, in.Repository, in.Ref, in.Claim.ArtifactDigest, raw(in.Claim), p.FrontierTicks).Scan(&expires)
		if err != nil {
			return 0, nil, err
		}
		return 201, map[string]any{"ticketId": id, "expiresAt": expires, "mode": p.Mode}, nil
	})
}
func consumeFrontierTicket(ctx context.Context, tx pgx.Tx, u *User, in IntentRequest, intent string) error {
	if in.AdmissionTicket == "" {
		return fail(422, "frontier_claim_required", "Run local validation and obtain a frontier admission ticket before submitting")
	}
	var data []byte
	err := tx.QueryRow(ctx, `SELECT claim FROM frontier_tickets WHERE id=$1 AND owner_id=$2 AND version_id=$3 AND repository=$4 AND ref=$5 AND artifact_digest=$6 AND consumed_at IS NULL AND expires_at>now() FOR UPDATE`, in.AdmissionTicket, u.ID, in.VersionID, in.Repository, in.Ref, in.PreviewDigest).Scan(&data)
	if err == pgx.ErrNoRows {
		return fail(409, "admission_ticket_invalid", "Ticket expired, consumed or does not match this account, version, commit and artifact")
	}
	if err != nil {
		return err
	}
	var c protocol.FrontierClaim
	if err = json.Unmarshal(data, &c); err != nil {
		return err
	}
	p, err := loadFrontierPolicy(ctx, tx, in.VersionID)
	if err != nil {
		return err
	}
	if err = checkFrontierClaim(c, p); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE frontier_tickets SET consumed_at=now(),intent_id=$2,frontier_ticks=$3 WHERE id=$1`, in.AdmissionTicket, intent, p.FrontierTicks)
	return err
}
