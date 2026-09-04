package platform

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	logaudit "github.com/matbalez/science-ladder/internal/audit"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	k, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	d, e := x509.MarshalPKIXPublicKey(k.Public())
	if e != nil {
		t.Fatal(e)
	}
	return k, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: d}))
}
func TestCheckpointCanonicalizationAndPersistentChain(t *testing.T) {
	s := testDB(t)
	_, v := seed(t, s)
	key, _ := testKey(t)
	s.ReceiptSigner = key
	s.Config.ReceiptKeyID = "test-platform"
	ctx := context.Background()
	for _, payload := range []any{map[string]any{"z": "last", "a": 1}, map[string]any{"nested": map[string]any{"b": 2, "a": 1}}} {
		tx, err := s.DB.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err = audit(ctx, tx, v, "canonical.test", payload); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.checkpointTick(ctx); err != nil {
		t.Fatal(err)
	}
	var b []byte
	if err := s.DB.QueryRow(ctx, `SELECT envelope FROM audit_checkpoints`).Scan(&b); err != nil {
		t.Fatal(err)
	}
	var envelope protocol.Envelope
	if err := json.Unmarshal(b, &envelope); err != nil {
		t.Fatal(err)
	}
	events, err := readAuditEvents(ctx, s.DB, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = logaudit.VerifyCheckpoint(envelope, map[string]crypto.PublicKey{"test-platform": key.Public()}, events, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
}
func TestExpiredRunnerLeaseKeepsCapacityAndFencesResults(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s)
	id := readyIntent(t, s, u, v, 31)
	_, body, err := accept(t, s, u, id, "lease-test-key")
	if err != nil {
		t.Fatal(err)
	}
	sid := body["submissionId"].(string)
	jobID := ID()
	if _, err = s.DB.Exec(context.Background(), `INSERT INTO runner_jobs(id,purpose,version_id,submission_id,payload,status,host_id,result_token_hash,lease_expires_at,attempts) VALUES($1,'confirmation',$2,$3,'{"excludedHostIds":["r1"]}','running','r2','token',now()-interval '1 second',1)`, jobID, v, sid); err != nil {
		t.Fatal(err)
	}
	if err = s.recoverRunnerLeases(context.Background()); err != nil {
		t.Fatal(err)
	}
	var status string
	var fence, units int
	var payload []byte
	s.DB.QueryRow(context.Background(), `SELECT status,fence,payload FROM runner_jobs WHERE id=$1`, jobID).Scan(&status, &fence, &payload)
	s.DB.QueryRow(context.Background(), `SELECT reserved_units FROM capacity`).Scan(&units)
	if status != "queued" || fence != 2 || units != 2 || !strings.Contains(string(payload), "r2") {
		t.Fatalf("lease recovery lost fencing/capacity: %s %d %d %s", status, fence, units, payload)
	}
}
func TestRunnerReceiptBindingsAndIndependentConfirmation(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s)
	intent := readyIntent(t, s, u, v, 41)
	_, body, err := accept(t, s, u, intent, "result-test-key")
	if err != nil {
		t.Fatal(err)
	}
	sid := body["submissionId"].(string)
	key, pub := testKey(t)
	host := runnerIdentity{ID: "r1", Group: "group1", PublicKey: pub}
	job := protocol.RunnerJob{APIVersion: protocol.APIVersion, Kind: "ValidationJob", ID: ID(), CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().Add(time.Hour), Purpose: "submission", SubmissionID: sid, ChallengeLockDigest: "lock", ArtifactDigest: "artifact", SuiteDigest: "suite", ExecutionProfileDigest: "profile", RunnerEpoch: "1", FencingToken: 1, RequiredHostGroup: "group1", Manifest: protocol.Manifest{HardGates: []string{"valid"}, Metric: protocol.Metric{Direction: "maximize", ToleranceTicks: "0"}}, DeploymentMode: "local"}
	digest, err := protocol.Digest(job)
	if err != nil {
		t.Fatal(err)
	}
	token := "result-capability"
	if _, err = s.DB.Exec(context.Background(), `INSERT INTO runner_jobs(id,purpose,version_id,submission_id,payload,status,host_id,result_token_hash,lease_expires_at) VALUES($1,'submission',$2,$3,$4,'running','r1',$5,$6)`, job.ID, v, sid, raw(job), hash(token), job.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	run := protocol.RunReceipt{APIVersion: protocol.APIVersion, Kind: "ValidationRunReceipt", ID: ID(), CreatedAt: time.Now().UTC(), Producer: "r1", JobID: job.ID, JobDigest: digest, ChallengeLockDigest: "lock", ArtifactDigest: "artifact", SuiteDigest: "suite", ExecutionProfileDigest: "profile", RunnerEpoch: "wrong", FencingToken: 1, HostID: "r1", HostGroup: "group1", Official: true, Outcome: "valid", ScoreTicks: "30", Gates: map[string]bool{"valid": true}, CleanupAttested: true, DeploymentMode: "local"}
	send := func(run protocol.RunReceipt) error {
		env, e := protocol.Sign("r1", key, run)
		if e != nil {
			return e
		}
		r := httptest.NewRequest("POST", "/internal/v1/runner/jobs/"+job.ID+"/result", strings.NewReader(string(raw(map[string]any{"envelope": env, "resultToken": token}))))
		r.SetPathValue("id", job.ID)
		return s.runnerResult(httptest.NewRecorder(), r, host)
	}
	if err = send(run); err == nil {
		t.Fatal("mismatched runner epoch accepted")
	}
	run.RunnerEpoch = "1"
	if err = send(run); err != nil {
		t.Fatal(err)
	}
	if err = send(run); err != nil {
		t.Fatalf("identical retry failed: %v", err)
	}
	var excluded string
	var count int
	if err = s.DB.QueryRow(context.Background(), `SELECT excluded_group FROM runner_jobs WHERE submission_id=$1 AND purpose='confirmation'`, sid).Scan(&excluded); err != nil {
		t.Fatal(err)
	}
	s.DB.QueryRow(context.Background(), `SELECT count(*) FROM runner_results`).Scan(&count)
	if excluded != "group1" || count != 1 {
		t.Fatal("primary result failed independent confirmation/deduplication")
	}
	run.ScoreTicks = "31"
	if err = send(run); err == nil {
		t.Fatal("conflicting duplicate result accepted")
	}
}

func TestProtocolMaximumTicksPersistAndAdjudicate(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s)
	intent := readyIntent(t, s, u, v, 51)
	_, body, err := accept(t, s, u, intent, "large-tick-key")
	if err != nil {
		t.Fatal(err)
	}
	score := strings.Repeat("9", 160)
	if _, err = protocol.CompareTicks(score, score); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(context.Background(), `UPDATE submissions SET status='validated',outcome='valid',score_ticks=$2 WHERE id=$1`, body["submissionId"], score); err != nil {
		t.Fatal(err)
	}
	if err = s.adjudicate(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err = s.DB.QueryRow(context.Background(), `SELECT score_ticks::text FROM submissions WHERE id=$1`, body["submissionId"]).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != score {
		t.Fatal("protocol-valid ticks changed in persistence")
	}
}

func TestPreparationQuotaAndExactSnapshotDedup(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s)
	ctx := context.Background()
	s.DB.Exec(ctx, `UPDATE users SET validation_quota=0 WHERE id=$1`, u.ID)
	send := func(key string) (map[string]any, error) {
		input := IntentRequest{VersionID: v, Repository: "test/repo", Ref: strings.Repeat("a", 40), Attribution: map[string]any{}, Publish: false}
		r := httptest.NewRequest("POST", "/v1/submission-intents", strings.NewReader(string(raw(input))))
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		err := s.createIntent(w, r, u)
		out := map[string]any{}
		json.Unmarshal(w.Body.Bytes(), &out)
		return out, err
	}
	if _, err := send("prep-no-quota"); err == nil {
		t.Fatal("zero-quota account queued remote preparation")
	}
	var count int
	s.DB.QueryRow(ctx, `SELECT count(*) FROM jobs`).Scan(&count)
	if count != 0 {
		t.Fatal("failed admission queued external work")
	}
	s.DB.Exec(ctx, `UPDATE users SET validation_quota=10 WHERE id=$1`, u.ID)
	a, err := send("prep-first-key")
	if err != nil {
		t.Fatal(err)
	}
	b, err := send("prep-second-key")
	if err != nil {
		t.Fatal(err)
	}
	if a["id"] != b["id"] {
		t.Fatal("same exact remote snapshot created duplicate preparation")
	}
	var used int
	s.DB.QueryRow(ctx, `SELECT used FROM preparation_budgets WHERE owner_id=$1`, u.ID).Scan(&used)
	s.DB.QueryRow(ctx, `SELECT count(*) FROM jobs`).Scan(&count)
	if used != 1 || count != 1 {
		t.Fatalf("duplicate preparation spent budget or queued work: %d %d", used, count)
	}
}

func TestOperatorCLITokenCannotAdministerAccounts(t *testing.T) {
	s := testDB(t)
	u, _ := seed(t, s)
	ctx := context.Background()
	s.DB.Exec(ctx, `UPDATE users SET role='operator' WHERE id=$1`, u.ID)
	token := "scoped-cli-token"
	if _, err := s.DB.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,scopes,expires_at) VALUES($1,$2,'{cli}',now()+interval '1 hour')`, hash(token), u.ID); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v1/invites", "/v1/editor/decisions", "/v1/auth/cli-sessions/" + ID() + "/approve"} {
		r := httptest.NewRequest("POST", path, strings.NewReader("{}"))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Idempotency-Key", "cli-scope-test")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 403 || !strings.Contains(w.Body.String(), "token_scope_forbidden") {
			t.Fatalf("CLI gained account administration: %s %d %s", path, w.Code, w.Body.String())
		}
	}
}
