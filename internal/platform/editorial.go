package platform

import (
	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/http"
	"strings"
	"time"
)

func editor(u *User) bool { return u != nil && (u.Role == "editor" || u.Role == "operator") }
func (s *Server) createFlag(w http.ResponseWriter, r *http.Request, u *User) error {
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var in struct {
			VersionID   string `json:"versionId"`
			Category    string `json:"category"`
			Message     string `json:"message"`
			EvidenceURL string `json:"evidenceUrl"`
		}
		if err := readJSON(r, &in); err != nil {
			return 0, nil, err
		}
		if len(in.Message) < 20 || len(in.Message) > 10000 {
			return 0, nil, fail(422, "flag_invalid", "Describe the concern in 20–10,000 characters")
		}
		if !strings.Contains(" science metric safety rights reproducibility integrity ", " "+in.Category+" ") {
			return 0, nil, fail(422, "category_invalid", "Select science, metric, safety, rights, reproducibility, or integrity")
		}
		id := ID()
		_, err := tx.Exec(r.Context(), `INSERT INTO flags(id,version_id,owner_id,category,message,evidence_url) VALUES($1,$2,$3,$4,$5,$6)`, id, in.VersionID, u.ID, in.Category, in.Message, in.EvidenceURL)
		return 201, map[string]any{"id": id, "status": "open"}, err
	})
}
func (s *Server) editorQueue(w http.ResponseWriter, r *http.Request, u *User) error {
	if !editor(u) {
		return fail(403, "editor_required", "An editor role is required")
	}
	flags, err := queryObjects(r.Context(), s.DB, `SELECT to_jsonb(f) FROM flags f WHERE status='open' ORDER BY created_at`)
	if err != nil {
		return err
	}
	reviews, err := queryObjects(r.Context(), s.DB, `SELECT jsonb_build_object('versionId',v.id,'title',v.manifest->>'title','status',v.review_status,'createdAt',v.created_at) FROM challenge_versions v WHERE v.review_status='human_review_required' ORDER BY v.created_at`)
	if err != nil {
		return err
	}
	candidates, err := queryObjects(r.Context(), s.DB, candidateSQL+` WHERE status='human_review_required' ORDER BY created_at`)
	if err != nil {
		return err
	}
	respond(w, 200, map[string]any{"flags": flags, "reviews": reviews, "candidates": candidates})
	return nil
}
func (s *Server) editorDecision(w http.ResponseWriter, r *http.Request, u *User) error {
	if !editor(u) {
		return fail(403, "editor_required", "An editor role is required")
	}
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var in struct {
			VersionID string `json:"versionId"`
			Action    string `json:"action"`
			Reason    string `json:"reason"`
		}
		if err := readJSON(r, &in); err != nil {
			return 0, nil, err
		}
		if len(strings.TrimSpace(in.Reason)) < 20 {
			return 0, nil, fail(422, "reason_required", "A public decision reason of at least 20 characters is required")
		}
		var status string
		if err := tx.QueryRow(r.Context(), `SELECT status FROM challenge_versions WHERE id=$1 FOR UPDATE`, in.VersionID).Scan(&status); err != nil {
			return 0, nil, err
		}
		sql := ""
		switch in.Action {
		case "approve_review":
			sql = `UPDATE challenge_versions SET review_status='human_approved' WHERE id=$1 AND review_status='human_review_required'`
		case "changes_required":
			sql = `UPDATE challenge_versions SET review_status='changes_required',status='changes_required' WHERE id=$1 AND lock_digest IS NULL`
		case "reject":
			sql = `UPDATE challenge_versions SET review_status='rejected',intake_status='closed' WHERE id=$1`
		case "feature":
			sql = `UPDATE challenge_versions SET badges=array_append(array_remove(badges,'Featured'),'Featured') WHERE id=$1`
		case "unfeature":
			sql = `UPDATE challenge_versions SET badges=array_remove(badges,'Featured') WHERE id=$1`
		case "human_reviewed":
			sql = `UPDATE challenge_versions SET badges=array_append(array_remove(badges,'Human-reviewed'),'Human-reviewed') WHERE id=$1 AND review_status IN('automated_pass','human_approved')`
		case "pause":
			sql = `UPDATE challenge_versions SET intake_status='paused' WHERE id=$1 AND status='published'`
		case "resume":
			sql = `UPDATE challenge_versions SET intake_status='open' WHERE id=$1 AND status='published' AND intake_status='paused' AND deadline>now()`
		case "resolve_unscorable":
			var faults int
			if err := tx.QueryRow(r.Context(), `SELECT count(*) FROM submissions WHERE version_id=$1 AND status='challenge_fault'`, in.VersionID).Scan(&faults); err != nil {
				return 0, nil, err
			}
			if faults == 0 {
				return 0, nil, fail(409, "checker_fault_required", "A version-wide unscorable resolution requires an unresolved locked-checker fault")
			}
			sql = `UPDATE challenge_versions SET status='compromised',intake_status='closed' WHERE id=$1`
		case "compromise":
			sql = `UPDATE challenge_versions SET status='compromised',intake_status='closed' WHERE id=$1`
		default:
			return 0, nil, fail(422, "action_invalid", "Unknown editorial action")
		}
		tag, err := tx.Exec(r.Context(), sql, in.VersionID)
		if err != nil {
			return 0, nil, err
		}
		if tag.RowsAffected() != 1 {
			return 0, nil, fail(409, "action_not_applicable", "This decision is not valid for the current challenge state")
		}
		id := ID()
		if _, err = tx.Exec(r.Context(), `INSERT INTO editorial_decisions(id,version_id,editor_id,action,reason) VALUES($1,$2,$3,$4,$5)`, id, in.VersionID, u.ID, in.Action, in.Reason); err != nil {
			return 0, nil, err
		}
		if in.Action == "resolve_unscorable" {
			rows, err := tx.Query(r.Context(), `UPDATE submissions SET status='validated',outcome='challenge_unscorable',score_ticks=NULL WHERE version_id=$1 AND status<>'finalized' RETURNING id`, in.VersionID)
			if err != nil {
				return 0, nil, err
			}
			ids := []string{}
			for rows.Next() {
				var sid string
				if err = rows.Scan(&sid); err != nil {
					rows.Close()
					return 0, nil, err
				}
				ids = append(ids, sid)
			}
			rows.Close()
			if _, err = tx.Exec(r.Context(), `UPDATE runner_jobs SET status='cancelled',fence=fence+1,lease_expires_at=NULL WHERE version_id=$1 AND status IN('queued','running','attention_required')`, in.VersionID); err != nil {
				return 0, nil, err
			}
			for _, sid := range ids {
				if err = enqueue(r.Context(), tx, "adjudicate", sid); err != nil {
					return 0, nil, err
				}
			}
			receipt := protocol.Receipt{DeploymentMode: s.Config.DeploymentMode, OfficialAcceptance: false, APIVersion: protocol.APIVersion, Kind: "ChallengeResolutionReceipt", ID: ID(), CreatedAt: time.Now().UTC(), Producer: "science-ladder", EconomicMode: "none", Data: map[string]any{"versionId": in.VersionID, "decisionId": id, "resolution": "challenge_unscorable", "reason": in.Reason, "editor": u.Login}}
			digest, err := protocol.Digest(receipt)
			if err != nil {
				return 0, nil, err
			}
			if err = saveReceipt(r, tx, digest, receipt, u.ID, true); err != nil {
				return 0, nil, err
			}
		}
		if err = audit(r.Context(), tx, in.VersionID, "editorial."+in.Action, map[string]any{"decisionId": id, "editor": u.Login, "reason": in.Reason}); err != nil {
			return 0, nil, err
		}
		return 201, map[string]any{"id": id, "action": in.Action}, nil
	})
}
func (s *Server) invite(w http.ResponseWriter, r *http.Request, u *User) error {
	if u.Role != "operator" {
		return fail(403, "operator_required", "An operator role is required")
	}
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var in struct {
			GitHubID int64  `json:"githubId"`
			Role     string `json:"role"`
			Quota    int    `json:"validationQuota"`
		}
		if err := readJSON(r, &in); err != nil {
			return 0, nil, err
		}
		if in.GitHubID <= 0 || (in.Role != "member" && in.Role != "editor") || in.Quota < 0 || in.Quota > 10000 {
			return 0, nil, fail(422, "invitation_invalid", "Use an immutable numeric GitHub ID, member/editor role, and bounded validation quota")
		}
		_, err := tx.Exec(r.Context(), `INSERT INTO invitations(github_id,role,validation_quota,invited_by) VALUES($1,$2,$3,$4) ON CONFLICT(github_id) DO UPDATE SET role=excluded.role,validation_quota=excluded.validation_quota`, in.GitHubID, in.Role, in.Quota, u.ID)
		if err != nil {
			return 0, nil, err
		}
		_, err = tx.Exec(r.Context(), `UPDATE users SET invited=true,role=$2,validation_quota=$3 WHERE github_id=$1 AND role<>'operator'`, in.GitHubID, in.Role, in.Quota)
		return 201, in, err
	})
}
