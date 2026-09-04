package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"log/slog"
	"time"
)

func (s *Server) RunWorker(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.recoverRunnerLeases(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("runner lease recovery failed", "error", err)
		}
		if err := s.checkpointTick(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("checkpoint maintenance failed", "error", err)
		}
		if err := s.workOne(ctx); err != nil && !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, context.Canceled) {
			slog.Error("worker operation failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
func (s *Server) workOne(ctx context.Context) error {
	var id, kind, resource string
	var attempts int
	lease := secret()
	err := s.DB.QueryRow(ctx, `WITH picked AS (SELECT id FROM jobs WHERE (status IN('queued','retry') AND available_at<=now()) OR (status='running' AND lease_expires_at<now()) ORDER BY available_at FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE jobs j SET status='running',attempts=attempts+1,lease_expires_at=now()+interval '10 minutes',lease_token=$1 FROM picked WHERE j.id=picked.id RETURNING j.id,j.kind,j.resource_id,j.attempts`, lease).Scan(&id, &kind, &resource, &attempts)
	if err != nil {
		return err
	}
	jobctx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	switch kind {
	case "resolve_candidate":
		err = s.resolveCandidate(jobctx, resource)
	case "fetch_submission":
		err = s.fetchSubmission(jobctx, resource)
	case "sign_receipt":
		err = s.signReceipt(jobctx, resource)
	case "preflight":
		err = s.preflight(jobctx, resource)
	case "complete_migration":
		err = s.completeMigration(jobctx, resource)
	case "scientific_review":
		err = s.scientificReview(jobctx, resource)
	case "validate_submission":
		err = s.queueSubmission(jobctx, resource)
	case "adjudicate":
		var version string
		err = s.DB.QueryRow(jobctx, `SELECT version_id FROM submissions WHERE id=$1`, resource).Scan(&version)
		if err == nil {
			err = s.adjudicate(jobctx, version)
		}
	default:
		err = fmt.Errorf("unsupported persisted job kind: %s", kind)
	}
	if err == nil {
		_, e := s.DB.Exec(ctx, `UPDATE jobs SET status='complete',lease_expires_at=NULL,last_error=NULL WHERE id=$1 AND lease_token=$2`, id, lease)
		return e
	}
	delay := time.Duration(min(attempts*attempts, 300)) * time.Second
	status := "retry"
	if attempts >= 8 {
		status = "attention_required"
	}
	message := err.Error()
	if len(message) > 1000 {
		message = message[:1000]
	}
	_, e := s.DB.Exec(ctx, `UPDATE jobs SET status=$3,last_error=$4,available_at=$5,lease_expires_at=NULL WHERE id=$1 AND lease_token=$2`, id, lease, status, message, time.Now().Add(delay))
	if e != nil {
		return e
	}
	return err
}
func (s *Server) fetchSubmission(ctx context.Context, id string) error {
	if s.Store == nil {
		return errors.New("immutable artifact storage unavailable")
	}
	var request, manifest []byte
	var owner, status string
	err := s.DB.QueryRow(ctx, `SELECT i.request,v.manifest,i.owner_id,i.status FROM submission_intents i JOIN challenge_versions v ON v.id=i.version_id WHERE i.id=$1`, id).Scan(&request, &manifest, &owner, &status)
	if err != nil {
		return err
	}
	if status != "github_fetch" {
		return nil
	}
	var in IntentRequest
	var m protocol.Manifest
	if err = json.Unmarshal(request, &in); err != nil {
		return err
	}
	if err = json.Unmarshal(manifest, &m); err != nil {
		return err
	}
	snapshot, err := s.fetchSnapshot(ctx, in.Repository, in.Ref, &m.Submission)
	if err != nil {
		return s.failIntent(ctx, id, err)
	}
	if snapshot.Private {
		var githubID int64
		if err = s.DB.QueryRow(ctx, `SELECT github_id FROM users WHERE id=$1`, owner).Scan(&githubID); err != nil {
			return err
		}
		if snapshot.OwnerGitHubID != githubID {
			return s.failIntent(ctx, id, errors.New("private submissions currently require a repository owned by your authenticated GitHub account; organization access must be separately verified"))
		}
	}
	b, digest, err := artifactBytes(snapshot, m.Submission)
	if err != nil {
		return s.failIntent(ctx, id, err)
	}
	if in.PreviewDigest != "" && in.PreviewDigest != digest {
		return s.failIntent(ctx, id, errors.New("CLI preview digest differs from independently constructed GitHub artifact"))
	}
	blob, err := s.Store.Put(ctx, b, "application/json")
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO artifacts(digest,blob_digest,size,media_type,owner_id) VALUES($1,$2,$3,'application/json',$4) ON CONFLICT DO NOTHING`, digest, blob, len(b), owner); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO artifacts(digest,blob_digest,size,media_type,owner_id) VALUES($1,$1,$2,'application/json',$3) ON CONFLICT DO NOTHING`, blob, len(b), owner); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE submission_intents SET source_commit=$2,repository_id=$3,artifact_digest=$4,status='quarantine_pending',findings=$5 WHERE id=$1 AND status='github_fetch'`, id, snapshot.Commit, snapshot.RepositoryID, digest, raw([]Finding{{"quarantine_required", "Canonical GitHub artifact stored. Waiting for isolated safety scanning and deterministic read-only disk preparation.", "pending", ""}})); err != nil {
		return err
	}
	if err = s.queuePreparation(ctx, tx, "artifact_prepare", id, in.VersionID, blob, digest, m); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Server) failIntent(ctx context.Context, id string, cause error) error {
	_, err := s.DB.Exec(ctx, `UPDATE submission_intents SET status='failed',findings=$2 WHERE id=$1`, id, raw([]Finding{{"artifact_rejected", cause.Error(), "error", ""}}))
	return err
}
func (s *Server) preflight(ctx context.Context, id string) error {
	var version string
	err := s.DB.QueryRow(ctx, `SELECT version_id FROM preflights WHERE id=$1`, id).Scan(&version)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE preflights SET status='waiting_for_quarantine',findings=$2 WHERE id=$1 AND status='queued'`, id, raw([]Finding{{"isolated_preflight_required", "Awaiting trusted quarantine builders: independent reproducible builds, full fixture/corpus runs, baseline replication, and immutable disk descriptors. No official machine result has been produced.", "pending", ""}}))
	if err != nil {
		return err
	}
	if err = enqueue(ctx, tx, "scientific_review", version); err != nil {
		return err
	}
	var document []byte
	var source string
	if err = tx.QueryRow(ctx, `SELECT manifest,source_digest FROM challenge_versions WHERE id=$1`, version).Scan(&document, &source); err != nil {
		return err
	}
	var m protocol.Manifest
	if err = json.Unmarshal(document, &m); err != nil {
		return err
	}
	if err = s.queuePreparation(ctx, tx, "preflight", id, version, source, source, m); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
