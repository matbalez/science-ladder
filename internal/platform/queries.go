package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strconv"
	"time"
)

const challengeSQL = `SELECT jsonb_build_object('id',c.id,'slug',c.slug,'creator',(SELECT jsonb_build_object('githubId',github_id,'login',login,'avatarUrl',avatar_url) FROM users WHERE id=c.owner_id),'title',v.manifest->>'title','summary',v.manifest->>'summary','domain','Mathematics & physics','status',v.status,'reviewStatus',v.review_status,'intakeStatus',v.intake_status,'economicMode',v.economic_mode,'verificationPolicy',CASE WHEN v.lock_digest IS NULL THEN COALESCE(NULLIF(v.manifest->>'verificationPolicy',''),'platform') ELSE (SELECT COALESCE(NULLIF(document->>'verificationPolicy',''),'independent') FROM locks WHERE digest=v.lock_digest) END,'versionId',v.id,'predecessorId',v.predecessor_id,'transitionKind',v.transition_kind,'migrationReceiptDigest',v.migration_receipt_digest,'repository',v.repository,'sourceCommit',v.source_commit,'createdAt',v.created_at,'deadline',v.deadline,'metric',(v.manifest->'metric')||jsonb_build_object('units',v.manifest->'metric'->>'unit'),'milestones',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',m.id,'label',m.title,'thresholdTicks',mv.threshold_ticks::text,'claimedBy',cl.submission_id,'claimedAt',cl.created_at) ORDER BY CASE WHEN v.manifest->'metric'->>'direction'='maximize' THEN mv.threshold_ticks ELSE -mv.threshold_ticks END) FROM milestone_tiers m JOIN milestone_version_mappings mv ON mv.milestone_id=m.id LEFT JOIN milestone_claims cl ON cl.milestone_id=m.id WHERE mv.version_id=v.id),'[]'::jsonb),'verifiedBest',(SELECT jsonb_build_object('submissionId',ss.id,'scoreTicks',ss.score_ticks::text) FROM submissions ss WHERE ss.id=v.verified_best_id),'publicFrontier',(SELECT jsonb_build_object('submissionId',ss.id,'scoreTicks',ss.score_ticks::text) FROM submissions ss WHERE ss.id=v.public_frontier_id),'badges',to_jsonb(v.badges),'manifest',v.manifest,'reviews',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',rr.id,'kind',rr.kind,'status',rr.status,'report',rr.report,'createdAt',rr.created_at)) FROM review_runs rr WHERE rr.version_id=v.id),'[]'::jsonb),'submissions','[]'::jsonb,'lockDigest',v.lock_digest,'researcherContext',` + researcherContextSQL + `) FROM challenges c JOIN challenge_versions v ON v.challenge_id=c.id`

func queryObjects(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, sql string, args ...any) ([]json.RawMessage, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func (s *Server) listChallenges(w http.ResponseWriter, r *http.Request, u *User) error {
	limit := 24
	if n, e := strconv.Atoi(r.URL.Query().Get("limit")); e == nil && n > 0 && n <= 100 {
		limit = n
	}
	search := r.URL.Query().Get("search")
	rows, err := queryObjects(r.Context(), s.DB, challengeSQL+` WHERE v.status IN('published','closed','superseded','compromised') AND NOT EXISTS(SELECT 1 FROM challenge_versions newer WHERE newer.challenge_id=c.id AND newer.status IN('published','closed','superseded','compromised') AND newer.created_at>v.created_at) AND ($1='' OR v.manifest->>'title' ILIKE '%'||$1||'%' OR v.manifest->>'summary' ILIKE '%'||$1||'%') ORDER BY v.created_at DESC LIMIT $2`, search, limit)
	if err != nil {
		return err
	}
	respond(w, 200, map[string]any{"challenges": rows})
	return nil
}
func (s *Server) getChallenge(w http.ResponseWriter, r *http.Request, u *User) error {
	uid := ""
	if u != nil {
		uid = u.ID
	}
	var b []byte
	err := s.DB.QueryRow(r.Context(), challengeSQL+` WHERE c.slug=$1 AND (v.status IN('published','closed','superseded','compromised') OR c.owner_id::text=$2) ORDER BY v.created_at DESC LIMIT 1`, r.PathValue("slug"), uid).Scan(&b)
	if err != nil {
		return err
	}
	var result map[string]any
	if err = json.Unmarshal(b, &result); err != nil {
		return err
	}
	subs, err := queryObjects(r.Context(), s.DB, submissionSQL+` WHERE s.version_id=$1 AND s.public ORDER BY s.sequence DESC LIMIT 100`, result["versionId"])
	if err != nil {
		return err
	}
	result["submissions"] = subs
	respond(w, 200, result)
	return nil
}

const candidateSQL = `SELECT jsonb_build_object('id',id,'status',status,'candidate',document,'findings',findings,'createdAt',created_at) FROM candidates`
const intentSQL = `SELECT jsonb_build_object('id',id,'versionId',version_id,'status',status,'repository',repository,'sourceCommit',source_commit,'artifactDigest',artifact_digest,'findings',findings,'submissionId',submission_id,'createdAt',created_at) FROM submission_intents`
const submissionSQL = `SELECT jsonb_build_object('id',s.id,'solver',(SELECT jsonb_build_object('githubId',github_id,'login',login,'avatarUrl',avatar_url) FROM users WHERE id=s.owner_id),'versionId',s.version_id,'sequence',s.sequence,'status',s.status,'outcome',s.outcome,'verificationPolicy',(SELECT COALESCE(NULLIF(payload->>'verificationPolicy',''),'independent') FROM receipts WHERE digest=s.receipt_digest),'verificationStatus',COALESCE((SELECT payload->>'verificationStatus' FROM receipts WHERE digest=s.adjudication_digest),''),'independentReplication',COALESCE((SELECT payload->>'verificationStatus'='independently_replicated' FROM receipts WHERE digest=s.adjudication_digest),false),'scoreTicks',s.score_ticks::text,'artifactDigest',s.artifact_digest,'commitment',s.commitment,'commitmentSalt',CASE WHEN s.public THEN s.commitment_salt ELSE NULL END,'repository',i.repository,'sourceCommit',i.source_commit,'public',s.public,'attribution',s.attribution,'createdAt',s.created_at,'receiptDigest',s.receipt_digest,'adjudicationDigest',s.adjudication_digest,'claims',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',mc.id,'milestoneId',mc.milestone_id,'createdAt',mc.created_at)) FROM milestone_claims mc WHERE mc.submission_id=s.id),'[]'::jsonb),'runs',COALESCE((SELECT jsonb_agg(rr.result||jsonb_build_object('receiptDigest',rr.digest,'originalHostEnvelope',rr.envelope,'envelope',(SELECT envelope FROM receipts WHERE digest=rr.digest))) FROM runner_results rr JOIN runner_jobs j ON j.id=rr.job_id WHERE j.submission_id=s.id),'[]'::jsonb)) FROM submissions s JOIN submission_intents i ON i.id=s.intent_id`

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request, u *User) error {
	c, err := queryObjects(r.Context(), s.DB, challengeSQL+` WHERE c.owner_id=$1 ORDER BY v.created_at DESC`, u.ID)
	if err != nil {
		return err
	}
	ca, err := queryObjects(r.Context(), s.DB, candidateSQL+` WHERE owner_id=$1 ORDER BY created_at DESC LIMIT 100`, u.ID)
	if err != nil {
		return err
	}
	subs, err := queryObjects(r.Context(), s.DB, submissionSQL+` WHERE s.owner_id=$1 ORDER BY s.created_at DESC LIMIT 100`, u.ID)
	if err != nil {
		return err
	}
	intents, err := queryObjects(r.Context(), s.DB, intentSQL+` WHERE owner_id=$1 ORDER BY created_at DESC LIMIT 100`, u.ID)
	if err != nil {
		return err
	}
	respond(w, 200, map[string]any{"challenges": c, "candidates": ca, "submissions": subs, "intents": intents})
	return nil
}
func (s *Server) getCandidate(w http.ResponseWriter, r *http.Request, u *User) error {
	var b []byte
	err := s.DB.QueryRow(r.Context(), candidateSQL+` WHERE id=$1 AND owner_id=$2`, r.PathValue("id"), u.ID).Scan(&b)
	if err != nil {
		return err
	}
	respond(w, 200, json.RawMessage(b))
	return nil
}
func (s *Server) getIntent(w http.ResponseWriter, r *http.Request, u *User) error {
	var b []byte
	err := s.DB.QueryRow(r.Context(), intentSQL+` WHERE id=$1 AND owner_id=$2`, r.PathValue("id"), u.ID).Scan(&b)
	if err != nil {
		return err
	}
	respond(w, 200, json.RawMessage(b))
	return nil
}
func (s *Server) getSubmission(w http.ResponseWriter, r *http.Request, u *User) error {
	uid := ""
	if u != nil {
		uid = u.ID
	}
	var b []byte
	err := s.DB.QueryRow(r.Context(), submissionSQL+` WHERE s.id=$1 AND (s.public OR s.owner_id::text=$2)`, r.PathValue("id"), uid).Scan(&b)
	if err != nil {
		return err
	}
	respond(w, 200, json.RawMessage(b))
	return nil
}
func (s *Server) getPreflight(w http.ResponseWriter, r *http.Request, u *User) error {
	var b []byte
	err := s.DB.QueryRow(r.Context(), `SELECT jsonb_build_object('id',p.id,'versionId',p.version_id,'status',p.status,'findings',p.findings,'reports',p.reports,'createdAt',p.created_at) FROM preflights p JOIN challenge_versions v ON v.id=p.version_id JOIN challenges c ON c.id=v.challenge_id WHERE p.id=$1 AND c.owner_id=$2`, r.PathValue("id"), u.ID).Scan(&b)
	if err != nil {
		return err
	}
	respond(w, 200, json.RawMessage(b))
	return nil
}
func (s *Server) getReceipt(w http.ResponseWriter, r *http.Request, u *User) error {
	uid := ""
	if u != nil {
		uid = u.ID
	}
	var b []byte
	err := s.DB.QueryRow(r.Context(), `SELECT envelope FROM receipts WHERE digest=$1 AND (public OR owner_id::text=$2)`, r.PathValue("digest"), uid).Scan(&b)
	if err != nil {
		return err
	}
	if b == nil {
		return fail(202, "signature_pending", "The committed receipt is waiting for its platform signature")
	}
	respond(w, 200, json.RawMessage(b))
	return nil
}
func (s *Server) getArtifact(w http.ResponseWriter, r *http.Request, u *User) error {
	if s.Store == nil {
		return fail(503, "storage_unavailable", "Artifact storage is not configured")
	}
	uid := ""
	if u != nil {
		uid = u.ID
	}
	var blob, media string
	var size int64
	err := s.DB.QueryRow(r.Context(), `SELECT blob_digest,media_type,size FROM artifacts WHERE digest=$1 AND (public_at IS NOT NULL OR owner_id::text=$2)`, r.PathValue("digest"), uid).Scan(&blob, &media, &size)
	if err != nil {
		return err
	}
	if size > 2<<20 {
		url, err := s.Store.SignedRead(r.Context(), blob, 5*time.Minute)
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "private, no-store")
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
		return nil
	}
	b, err := s.Store.Get(r.Context(), blob, size)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", media)
	w.Header().Set("Content-Disposition", "attachment; filename=artifact.json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Write(b)
	return nil
}
func (s *Server) exportChallenge(w http.ResponseWriter, r *http.Request, u *User) error {
	var challenge []byte
	err := s.DB.QueryRow(r.Context(), challengeSQL+` WHERE v.id=$1 AND v.status IN('published','closed','superseded','compromised')`, r.PathValue("id")).Scan(&challenge)
	if err != nil {
		return err
	}
	receipts, err := queryObjects(r.Context(), s.DB, `SELECT r.envelope FROM receipts r WHERE r.public AND r.envelope IS NOT NULL AND (r.payload->'data'->>'versionId'=(SELECT id::text FROM challenge_versions WHERE id=$1) OR r.digest IN(SELECT reveal_receipt_digest FROM hidden_suites WHERE commitment=(SELECT manifest->'suite'->>'commitment' FROM challenge_versions WHERE id=$1)) OR r.digest IN(SELECT lock_digest FROM challenge_versions WHERE id=$1 UNION SELECT machine_receipt_digest FROM preflights WHERE version_id=$1 UNION SELECT receipt_digest FROM submissions WHERE version_id=$1 AND public UNION SELECT adjudication_digest FROM submissions WHERE version_id=$1 AND public UNION SELECT rr.digest FROM runner_results rr JOIN runner_jobs j ON j.id=rr.job_id WHERE j.version_id=$1))`, r.PathValue("id"))
	if err != nil {
		return err
	}
	events, err := queryObjects(r.Context(), s.DB, `SELECT jsonb_build_object('sequence',sequence::text,'kind',kind,'payload',payload,'previousDigest',previous_digest,'digest',digest,'createdAt',created_at) FROM audit_events WHERE version_id=$1 ORDER BY sequence`, r.PathValue("id"))
	if err != nil {
		return err
	}
	subs, err := queryObjects(r.Context(), s.DB, submissionSQL+` WHERE s.version_id=$1 AND s.public ORDER BY s.sequence`, r.PathValue("id"))
	if err != nil {
		return err
	}
	artifacts, err := queryObjects(r.Context(), s.DB, `SELECT jsonb_build_object('digest',a.digest,'blobDigest',a.blob_digest,'size',a.size,'mediaType',a.media_type,'downloadPath','/v1/artifacts/'||a.digest) FROM artifacts a WHERE a.public_at IS NOT NULL AND a.digest IN(SELECT source_digest FROM challenge_versions WHERE id=$1 UNION SELECT artifact_digest FROM submissions WHERE version_id=$1 AND public UNION SELECT disk_digest FROM submissions WHERE version_id=$1 AND public UNION SELECT ru.digest FROM runner_uploads ru JOIN runner_jobs j ON j.id=ru.job_id WHERE j.version_id=$1 AND j.purpose='preflight' UNION SELECT r.payload->'data'->>'sourceDigest' FROM hidden_suites hs JOIN receipts r ON r.digest=hs.reveal_receipt_digest WHERE hs.commitment=(SELECT manifest->'suite'->>'commitment' FROM challenge_versions WHERE id=$1)) ORDER BY a.digest`, r.PathValue("id"))
	if err != nil {
		return err
	}
	researchers, err := queryObjects(r.Context(), s.DB, `SELECT `+researcherEditionJSON+` FROM challenge_researcher_editions re WHERE re.version_id=$1 ORDER BY re.edition_sequence`, r.PathValue("id"))
	if err != nil {
		return err
	}
	respond(w, 200, map[string]any{"apiVersion": "science-ladder/v1", "kind": "ChallengeExport", "challenge": json.RawMessage(challenge), "researcherHistory": researchers, "receipts": receipts, "submissions": subs, "artifacts": artifacts, "audit": events, "auditScope": "version projection; use global audit endpoint and witnessed checkpoints to verify the contiguous global chain", "globalAuditPath": "/v1/audit/events", "checkpointPath": "/v1/audit/checkpoints", "limitations": []string{"Private artifacts and hidden suites cannot be independently recomputed until disclosed.", "Independent witness quorum is a production launch gate; inspect deployment trust status."}})
	return nil
}
func (s *Server) events(w http.ResponseWriter, r *http.Request, u *User) error {
	var public bool
	if err := s.DB.QueryRow(r.Context(), `SELECT status IN('published','closed','superseded','compromised') FROM challenge_versions WHERE id=$1`, r.PathValue("id")).Scan(&public); err != nil {
		return err
	}
	if !public {
		return pgx.ErrNoRows
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fail(500, "stream_unavailable", "Response streaming is unavailable")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	last, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		rows, err := s.DB.Query(r.Context(), `SELECT sequence,kind,payload FROM audit_events WHERE version_id=$1 AND sequence>$2 ORDER BY sequence LIMIT 100`, r.PathValue("id"), last)
		if err != nil {
			return nil
		}
		for rows.Next() {
			var seq int64
			var kind string
			var b []byte
			if rows.Scan(&seq, &kind, &b) != nil {
				continue
			}
			fmt.Fprintf(w, "id: %d\nevent: update\ndata: %s\n\n", seq, raw(map[string]any{"kind": kind, "payload": json.RawMessage(b)}))
			last = seq
		}
		rows.Close()
		fmt.Fprint(w, ": heartbeat\n\n")
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return nil
		case <-ticker.C:
		}
	}
}
