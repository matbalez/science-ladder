package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func admissionFixture(t *testing.T, s *Server, v string, n int) claimRequest {
	t.Helper()
	p, e := loadFrontierPolicy(context.Background(), s.DB, v)
	if e != nil {
		t.Fatal(e)
	}

	return claimRequest{Repository: "test/repo", Ref: fmt.Sprintf("%040x", n), Claim: protocol.FrontierClaim{APIVersion: protocol.FrontierClaimVersion, VersionID: v, LockDigest: p.LockDigest, ArtifactDigest: protocol.DigestBytes([]byte(fmt.Sprint(n))), Mode: "frontier", Result: &protocol.ValidatorResult{APIVersion: protocol.APIVersion, Kind: "ValidatorResult", Score: "20", Gates: map[string]bool{}}}}
}
func admissionRequest(s *Server, u *User, body any, key string, handler func(http.ResponseWriter, *http.Request, *User) error) (map[string]any, error) {
	r := httptest.NewRequest("POST", "/v1/test-admission", strings.NewReader(string(raw(body))))
	r.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	e := handler(w, r, u)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out, e
}
func TestFrontierAdmissionRejectsBeforeWork(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s, protocol.VerificationPlatform)
	in := admissionFixture(t, s, v, 1)
	in.Claim.Result.Score = "0"
	if _, e := admissionRequest(s, u, in, "loser-claim-key", s.createFrontierTicket); e == nil {
		t.Fatal("nonfrontier admitted")
	}
	intent := IntentRequest{VersionID: v, Repository: in.Repository, Ref: in.Ref}
	if _, e := admissionRequest(s, u, intent, "missing-ticket-key", s.createIntent); e == nil {
		t.Fatal("legacy bypass admitted")
	}
	for _, table := range []string{"jobs", "frontier_tickets", "submission_intents", "preparation_budgets"} {
		var n int
		if e := s.DB.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); e != nil || n != 0 {
			t.Fatal(table, n, e)
		}
	}
}
func TestFrontierTicketBindingsAndReplay(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s, protocol.VerificationPlatform)
	in := admissionFixture(t, s, v, 1)
	ticket, e := admissionRequest(s, u, in, "issue-ticket-key", s.createFrontierTicket)
	if e != nil {
		t.Fatal(e)
	}
	again, e := admissionRequest(s, u, in, "issue-second-key", s.createFrontierTicket)
	if e != nil || ticket["ticketId"] != again["ticketId"] {
		t.Fatal(again, e)
	}
	original := IntentRequest{AdmissionTicket: ticket["ticketId"].(string), VersionID: v, Repository: in.Repository, Ref: in.Ref, PreviewDigest: in.Claim.ArtifactDigest}
	for i, field := range []string{"ref", "repository", "digest", "owner", "expiry"} {
		bad := original
		who := u
		switch field {
		case "ref":
			bad.Ref = strings.Repeat("b", 40)
		case "repository":
			bad.Repository = "test/other"
		case "digest":
			bad.PreviewDigest = protocol.DigestBytes([]byte("wrong"))
		case "owner":
			other := *u
			other.ID = ID()
			who = &other
			s.DB.Exec(context.Background(), `INSERT INTO users(id,github_id,login,invited,validation_quota) VALUES($1,43,'other',true,20)`, other.ID)
		case "expiry":
			s.DB.Exec(context.Background(), `UPDATE frontier_tickets SET expires_at=now()-interval '1 second' WHERE id=$1`, original.AdmissionTicket)
		}
		if _, e = admissionRequest(s, who, bad, fmt.Sprintf("bad-binding-%d", i), s.createIntent); e == nil {
			t.Fatal("accepted wrong", field)
		}
	}
	s.DB.Exec(context.Background(), `UPDATE frontier_tickets SET expires_at=now()+interval '10 minutes' WHERE id=$1`, original.AdmissionTicket)
	first, e := admissionRequest(s, u, original, "consume-ticket-key", s.createIntent)
	if e != nil {
		t.Fatal(e)
	}
	replay, e := admissionRequest(s, u, original, "consume-again-key", s.createIntent)
	if e != nil || first["id"] != replay["id"] {
		t.Fatal(replay, e)
	}
	ticketAgain, e := admissionRequest(s, u, in, "resume-claim-key", s.createFrontierTicket)
	if e != nil || ticketAgain["resumeOnly"] != true {
		t.Fatal(ticketAgain, e)
	}
	changed := original
	changed.Ref = strings.Repeat("c", 40)
	if _, e = admissionRequest(s, u, changed, "reuse-other-key", s.createIntent); e == nil {
		t.Fatal("ticket reused")
	}
	var jobs, used int
	s.DB.QueryRow(context.Background(), `SELECT count(*) FROM jobs`).Scan(&jobs)
	s.DB.QueryRow(context.Background(), `SELECT used FROM preparation_budgets WHERE owner_id=$1`, u.ID).Scan(&used)
	if jobs != 1 || used != 1 {
		t.Fatal(jobs, used)
	}
}
func TestFrontierAdmissionConcurrentBudget(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s, protocol.VerificationPlatform)
	// The retired zero lifetime allowance must not block valid claims.
	s.DB.Exec(context.Background(), `UPDATE users SET validation_quota=0 WHERE id=$1`, u.ID)
	claims := make([]claimRequest, 10)
	for i := range claims {
		claims[i] = admissionFixture(t, s, v, i+1)
	}
	var wg sync.WaitGroup
	for i := range claims {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = admissionRequest(s, u, claims[i], fmt.Sprintf("concurrent-claim-%d", i), s.createFrontierTicket)
		}(i)
	}
	wg.Wait()
	var n int
	s.DB.QueryRow(context.Background(), `SELECT count(*) FROM frontier_tickets`).Scan(&n)
	if n != 3 {
		t.Fatal("active budget not atomic", n)
	}
	s.DB.Exec(context.Background(), `UPDATE frontier_tickets SET expires_at=now()-interval '1 second'`)
	for i := range claims {
		_, _ = admissionRequest(s, u, claims[i], fmt.Sprintf("daily-claim-%d", i), s.createFrontierTicket)
	}
	s.DB.QueryRow(context.Background(), `SELECT count(*) FROM frontier_tickets`).Scan(&n)
	if n != 5 {
		t.Fatal("daily budget exceeded", n)
	}
}
func TestFrontierMovesBeforeTicketConsumption(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s, protocol.VerificationPlatform)
	in := admissionFixture(t, s, v, 1)
	ticket, e := admissionRequest(s, u, in, "issue-stale-ticket", s.createFrontierTicket)
	if e != nil {
		t.Fatal(e)
	}
	prior := readyIntent(t, s, u, v, 90)
	_, accepted, e := accept(t, s, u, prior, "accept-prior-frontier")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(context.Background(), `UPDATE submissions SET score_ticks=21,public=true WHERE id=$1`, accepted["submissionId"]); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(context.Background(), `UPDATE challenge_versions SET public_frontier_id=$2 WHERE id=$1`, v, accepted["submissionId"]); e != nil {
		t.Fatal(e)
	}

	intent := IntentRequest{AdmissionTicket: ticket["ticketId"].(string), VersionID: v, Repository: in.Repository, Ref: in.Ref, PreviewDigest: in.Claim.ArtifactDigest}
	if _, e = admissionRequest(s, u, intent, "consume-stale-ticket", s.createIntent); e == nil {
		t.Fatal("stale noncompetitive claim queued")
	}
	var n int
	s.DB.QueryRow(context.Background(), `SELECT count(*) FROM jobs WHERE kind='fetch_submission'`).Scan(&n)
	if n != 0 {
		t.Fatal("stale claim queued work")
	}
}

func TestAcceptedClaimBindsPreparedBytesAndReceipt(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s, protocol.VerificationPlatform)
	in := admissionFixture(t, s, v, 1)
	ticket, e := admissionRequest(s, u, in, "receipt-claim-ticket", s.createFrontierTicket)
	if e != nil {
		t.Fatal(e)
	}
	intent := IntentRequest{AdmissionTicket: ticket["ticketId"].(string), VersionID: v, Repository: in.Repository, Ref: in.Ref, PreviewDigest: in.Claim.ArtifactDigest}
	created, e := admissionRequest(s, u, intent, "receipt-intent-key", s.createIntent)
	if e != nil {
		t.Fatal(e)
	}
	id := created["id"].(string)
	ctx := context.Background()
	for _, digest := range []string{in.Claim.ArtifactDigest, protocol.DigestBytes([]byte("wrong"))} {
		if _, e = s.DB.Exec(ctx, `INSERT INTO artifacts(digest,blob_digest,size,media_type,owner_id) VALUES($1,$1,1,'application/json',$2)`, digest, u.ID); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = s.DB.Exec(ctx, `UPDATE submission_intents SET status='ready',artifact_digest=$2,disk_digest=$2 WHERE id=$1`, id, protocol.DigestBytes([]byte("wrong"))); e != nil {
		t.Fatal(e)
	}
	if _, _, e = accept(t, s, u, id, "wrong-artifact-accept"); e == nil {
		t.Fatal("prepared bytes not bound to claim")
	}
	if _, e = s.DB.Exec(ctx, `UPDATE submission_intents SET artifact_digest=$2,disk_digest=$2 WHERE id=$1`, id, in.Claim.ArtifactDigest); e != nil {
		t.Fatal(e)
	}
	_, accepted, e := accept(t, s, u, id, "correct-artifact-accept")
	if e != nil {
		t.Fatal(e)
	}
	var evidence []byte
	if e = s.DB.QueryRow(ctx, `SELECT payload->'data'->'admission' FROM receipts WHERE digest=$1`, accepted["receiptDigest"]).Scan(&evidence); e != nil {
		t.Fatal(e)
	}
	var a map[string]any
	if e = json.Unmarshal(evidence, &a); e != nil {
		t.Fatal(e)
	}
	digest, _ := protocol.Digest(in.Claim)
	if a["claimDigest"] != digest || a["claimVerified"] != false {
		t.Fatal(a)
	}
}

func TestHiddenQualificationHasStricterBudget(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s, protocol.VerificationPlatform)
	ctx := context.Background()
	p, e := loadFrontierPolicy(ctx, s.DB, v)
	if e != nil {
		t.Fatal(e)
	}
	p.Manifest.Suite.Visibility = "hidden"
	if _, e = s.DB.Exec(ctx, `UPDATE challenge_versions SET intake_status='closed' WHERE id=$1`, v); e != nil {
		t.Fatal(e)
	}
	hidden := ID()
	lock := protocol.Lock{VerificationPolicy: protocol.VerificationPlatform, Manifest: p.Manifest, ExecutionProfileDigest: "profile"}
	digest, _ := protocol.Digest(lock)
	if _, e = s.DB.Exec(ctx, `INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,status,intake_status,deadline,lock_digest) SELECT $2,challenge_id,repository,repository_id,source_commit,source_digest,$3,'published','open',deadline,$4 FROM challenge_versions WHERE id=$1`, v, hidden, raw(p.Manifest), digest); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(ctx, `INSERT INTO locks(digest,version_id,document) VALUES($1,$2,$3)`, digest, hidden, raw(lock)); e != nil {
		t.Fatal(e)
	}
	for i := 1; i <= 3; i++ {
		in := admissionFixture(t, s, hidden, i)
		in.Claim.Mode = "qualification"
		in.Claim.Result = nil
		in.Claim.Qualification = &protocol.LocalQualification{ChecksPassed: true, ReportDigest: protocol.DigestBytes([]byte("public qualification report")), EstimatedScoreTicks: "20", Reason: "Public tests pass; hidden test performance requires measurement."}
		_, e = admissionRequest(s, u, in, fmt.Sprintf("qualification-claim-%d", i), s.createFrontierTicket)
		if i < 3 && e != nil {
			t.Fatal(e)
		}
		if i == 3 && e == nil {
			t.Fatal("qualification budget bypassed")
		}
	}
	var jobs int
	s.DB.QueryRow(ctx, `SELECT count(*) FROM jobs`).Scan(&jobs)
	if jobs != 0 {
		t.Fatal("claim issuance executed work")
	}
}
