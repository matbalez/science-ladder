package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/http"
)

// Preparation consumes its own durable daily budget before any external fetch,
// scientific-review request, or quarantine work. It creates no competitive rank.
func (s *Server) reservePreparation(ctx context.Context, tx pgx.Tx, u *User) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(6842077294)`); err != nil {
		return err
	}
	var quota int
	var role string
	if err := tx.QueryRow(ctx, `SELECT validation_quota,role FROM users WHERE id=$1`, u.ID).Scan(&quota, &role); err != nil {
		return err
	}
	if quota <= 0 {
		return fail(429, "preparation_grant_required", "A free validation grant is required before remote preparation")
	}
	var active int
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM candidates WHERE owner_id=$1 AND status='resolving_sources')+(SELECT count(*) FROM submission_intents WHERE owner_id=$1 AND status IN('github_fetch','quarantine_pending','ready'))+(SELECT count(*) FROM preflights p JOIN challenge_versions v ON v.id=p.version_id JOIN challenges c ON c.id=v.challenge_id WHERE c.owner_id=$1 AND p.status IN('queued','waiting_for_quarantine','independent_confirmation'))+(SELECT count(*) FROM scientific_review_requests WHERE requested_by=$1 AND status='queued')`, u.ID).Scan(&active); err != nil {
		return err
	}
	if active >= 5 {
		return fail(429, "preparation_active_limit", "Resolve an existing remote preparation before starting another")
	}
	var total int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(used),0) FROM preparation_budgets WHERE day=current_date`).Scan(&total); err != nil {
		return err
	}
	if total >= 1000 {
		return fail(503, "preparation_capacity_exhausted", "Remote preparation daily capacity is reserved; try after the next UTC day")
	}
	limit := preparationDailyLimit(role, s.Config.OperatorPreparationDailyLimit)
	tag, err := tx.Exec(ctx, `INSERT INTO preparation_budgets(owner_id,used) VALUES($1,1) ON CONFLICT(owner_id,day) DO UPDATE SET used=preparation_budgets.used+1 WHERE preparation_budgets.used<$2`, u.ID, limit)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fail(429, "preparation_daily_limit", fmt.Sprintf("This account has used its %d daily remote-preparation requests", limit))
	}
	return nil
}
func (s *Server) reserveFetch(ctx context.Context, u *User) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = s.reservePreparation(ctx, tx, u); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Synchronous GitHub fetch endpoints replay completed requests before spending a
// new preparation budget. Failed network attempts remain intentionally metered.
func (s *Server) replayBeforeFetch(w http.ResponseWriter, r *http.Request, u *User, body []byte) (bool, error) {
	key := r.Header.Get("Idempotency-Key")
	if !idemRE.MatchString(key) {
		return false, fail(400, "idempotency_required", "Idempotency-Key must contain 8–128 safe characters")
	}
	var previous string
	var response []byte
	var status int
	err := s.DB.QueryRow(r.Context(), `SELECT request_hash,response,status_code FROM idempotency WHERE user_id=$1 AND key=$2`, u.ID, key).Scan(&previous, &response, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if previous != hash(r.Method+" "+r.URL.Path+"\n"+string(body)) {
		return false, fail(409, "idempotency_conflict", "This idempotency key belongs to different request content")
	}
	respond(w, status, json.RawMessage(response))
	return true, nil
}

// Operators may configure a larger, still metered allowance for release validation.
// Role comes from the database; ordinary users retain the twenty-request cap.
func preparationDailyLimit(role string, configured int) int {
	if role != "operator" || configured < 20 {
		return 20
	}
	if configured > 1000 {
		return 1000
	}
	return configured
}
