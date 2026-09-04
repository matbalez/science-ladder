package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestScoreComparisonAndConfirmation(t *testing.T) {
	m := protocol.Metric{Direction: "maximize", ToleranceTicks: "2", MinimumDeltaTicks: "3"}
	for _, tc := range []struct {
		a, b, want string
		err        bool
	}{{"100", "102", "100", false}, {"102", "100", "100", false}, {"100", "103", "", true}, {"999999999999999999999999", "999999999999999999999998", "999999999999999999999998", false}} {
		got, err := ConfirmedScore(tc.a, tc.b, m)
		if got != tc.want || (err != nil) != tc.err {
			t.Fatalf("confirmation %s/%s: got %s %v", tc.a, tc.b, got, err)
		}
	}
	m.Direction = "minimize"
	if v, e := ConfirmedScore("-10", "-12", m); e != nil || v != "-10" {
		t.Fatal(v, e)
	}
	if meaningful("102", "100", protocol.Metric{Direction: "maximize", MinimumDeltaTicks: "3"}) {
		t.Fatal("insignificant record accepted")
	}
	if !crosses("100000000000000000000", "99999999999999999999", "maximize") {
		t.Fatal("big integer ordering lost precision")
	}
}
func TestSourceNetworkPolicy(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1", "10.1.2.3", "192.168.1.1", "169.254.169.254", "100.100.100.200", "100.64.0.1", "198.18.0.1", "0.1.2.3", "224.0.0.1", "fc00::1", "::ffff:127.0.0.1"} {
		if publicIP(net.ParseIP(ip)) {
			t.Errorf("unsafe source IP accepted: %s", ip)
		}
	}
	if !publicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public source rejected")
	}
}
func testDB(t *testing.T) *Server {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not configured")
	}
	ctx := context.Background()
	base, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + strings.ReplaceAll(ID(), "-", "")
	if _, err = base.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{DB: db, Config: Config{PublicOrigin: "http://localhost:3000", ActiveLimit: 10, DeploymentMode: "local"}}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); base.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`); base.Close() })
	return s
}
func seed(t *testing.T, s *Server) (*User, string) {
	t.Helper()
	ctx := context.Background()
	u := &User{ID: ID(), GitHubID: 42, Login: "test-user", Invited: true, Role: "member", Quota: 20}
	candidate, ch, version := ID(), ID(), ID()
	m := protocol.Manifest{APIVersion: protocol.APIVersion, Kind: "Challenge", ID: ID(), EconomicMode: "none", Slug: "test-challenge", Title: "Test", Metric: protocol.Metric{Direction: "maximize", BaselineTicks: "0", Quantum: "1", MinimumDeltaTicks: "1", ToleranceTicks: "0"}, Resources: protocol.Resources{Class: "cpu-small"}, Milestones: []protocol.Milestone{{ID: "m1", Title: "First", ThresholdTicks: "10"}, {ID: "m2", Title: "Second", ThresholdTicks: "20"}}}
	commands := []struct {
		sql  string
		args []any
	}{{`INSERT INTO users(id,github_id,login,invited,validation_quota) VALUES($1,42,'test-user',true,20)`, []any{u.ID}}, {`INSERT INTO candidates(id,owner_id,digest,document,status) VALUES($1,$2,'candidate','{}','ready')`, []any{candidate, u.ID}}, {`INSERT INTO challenges(id,slug,owner_id,candidate_id) VALUES($1,'test-challenge',$2,$3)`, []any{ch, u.ID, candidate}}, {`INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,status,intake_status,deadline,lock_digest) VALUES($1,$2,'test/repo',1,$3,'source',$4,'published','open',now()+interval '1 day','lock')`, []any{version, ch, strings.Repeat("a", 40), raw(m)}}, {`INSERT INTO milestone_tiers(id,version_id,title,threshold_ticks) VALUES('m1',$1,'First',10),('m2',$1,'Second',20)`, []any{version}}, {`UPDATE capacity SET maximum_units=100 WHERE id=1`, nil}, {`INSERT INTO runner_hosts(id,host_group,public_key,certificate_fingerprint,enabled) VALUES('r1','group1','key1','fp1',true),('r2','group2','key2','fp2',true)`, nil}}
	for _, cmd := range commands {
		if _, err := s.DB.Exec(ctx, cmd.sql, cmd.args...); err != nil {
			t.Fatal(err)
		}
	}
	return u, version
}
func readyIntent(t *testing.T, s *Server, u *User, version string, n int) string {
	t.Helper()
	id := ID()
	artifact := fmt.Sprintf("sha256:%064d", n)
	disk := fmt.Sprintf("sha256:%064d", n+1000)
	ctx := context.Background()
	for _, d := range []string{artifact, disk} {
		if _, err := s.DB.Exec(ctx, `INSERT INTO artifacts(digest,blob_digest,size,media_type,owner_id) VALUES($1,$1,1,'application/json',$2)`, d, u.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO submission_intents(id,version_id,owner_id,repository,ref,artifact_digest,disk_digest,status,request) VALUES($1,$2,$3,'test/repo',$4,$5,$6,'ready',$7)`, id, version, u.ID, fmt.Sprintf("%040x", n), artifact, disk, raw(IntentRequest{VersionID: version, License: "MIT", Attribution: map[string]any{}})); err != nil {
		t.Fatal(err)
	}
	return id
}
func accept(t *testing.T, s *Server, u *User, id, key string) (int, map[string]any, error) {
	r := httptest.NewRequest("POST", "/v1/submission-intents/"+id+"/accept", strings.NewReader("{}"))
	r.SetPathValue("id", id)
	r.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	err := s.acceptIntent(w, r, u)
	result := map[string]any{}
	json.Unmarshal(w.Body.Bytes(), &result)
	return w.Code, result, err
}
func TestAdmissionConcurrencyAndRetry(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s)
	ids := []string{readyIntent(t, s, u, v, 1), readyIntent(t, s, u, v, 2), readyIntent(t, s, u, v, 3)}
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			_, _, err := accept(t, s, u, id, fmt.Sprintf("accept-key-%d", i))
			errs <- err
		}(i, id)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count, minseq, maxseq, quota, units int
	if err := s.DB.QueryRow(context.Background(), `SELECT count(*),min(sequence),max(sequence) FROM submissions`).Scan(&count, &minseq, &maxseq); err != nil {
		t.Fatal(err)
	}
	if count != 3 || minseq != 1 || maxseq != 3 {
		t.Fatalf("non-contiguous sequences %d %d %d", count, minseq, maxseq)
	}
	if _, _, err := accept(t, s, u, ids[0], "accept-key-0"); err != nil {
		t.Fatal(err)
	}
	s.DB.QueryRow(context.Background(), `SELECT validation_quota FROM users WHERE id=$1`, u.ID).Scan(&quota)
	s.DB.QueryRow(context.Background(), `SELECT reserved_units FROM capacity`).Scan(&units)
	if quota != 17 || units != 6 {
		t.Fatalf("retry spent quota/capacity twice: %d %d", quota, units)
	}
}
func TestOrderedAllCrossedMilestones(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s)
	ids := []string{readyIntent(t, s, u, v, 10), readyIntent(t, s, u, v, 11)}
	subs := []string{}
	for i, id := range ids {
		_, body, err := accept(t, s, u, id, fmt.Sprintf("order-key-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		subs = append(subs, body["submissionId"].(string))
	}
	ctx := context.Background()
	s.DB.Exec(ctx, `UPDATE submissions SET status='validated',outcome='valid',score_ticks=100 WHERE id=$1`, subs[1])
	if err := s.adjudicate(ctx, v); err != nil {
		t.Fatal(err)
	}
	var watermark int
	s.DB.QueryRow(ctx, `SELECT watermark FROM challenge_versions WHERE id=$1`, v).Scan(&watermark)
	if watermark != 0 {
		t.Fatal("later completion overtook earlier receipt")
	}
	s.DB.Exec(ctx, `UPDATE submissions SET status='validated',outcome='valid',score_ticks=30 WHERE id=$1`, subs[0])
	if err := s.adjudicate(ctx, v); err != nil {
		t.Fatal(err)
	}
	var claimed int
	s.DB.QueryRow(ctx, `SELECT count(*) FROM milestone_claims WHERE submission_id=$1`, subs[0]).Scan(&claimed)
	s.DB.QueryRow(ctx, `SELECT watermark FROM challenge_versions WHERE id=$1`, v).Scan(&watermark)
	if claimed != 2 || watermark != 2 {
		t.Fatalf("earliest score failed all crossed claims: claims=%d watermark=%d", claimed, watermark)
	}
	if err := s.adjudicate(ctx, v); err != nil {
		t.Fatal(err)
	}
	s.DB.QueryRow(ctx, `SELECT count(*) FROM milestone_claims`).Scan(&claimed)
	if claimed != 2 {
		t.Fatal("retry duplicated claims")
	}
	var public bool
	s.DB.QueryRow(ctx, `SELECT public FROM submissions WHERE id=$1`, subs[0]).Scan(&public)
	if !public {
		t.Fatal("milestone winner did not become public atomically")
	}
	s.DB.QueryRow(ctx, `SELECT public FROM submissions WHERE id=$1`, subs[1]).Scan(&public)
	if public {
		t.Fatal("private non-winning record was disclosed")
	}
}
func TestImmutableLockAndAuditRollback(t *testing.T) {
	s := testDB(t)
	_, version := seed(t, s)
	ctx := context.Background()
	if _, err := s.DB.Exec(ctx, `UPDATE challenge_versions SET manifest='{}' WHERE id=$1`, version); err == nil {
		t.Fatal("locked contract mutated")
	}
	for i := 0; i < 3; i++ {
		tx, err := s.DB.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err = audit(ctx, tx, version, "test", map[string]any{"b": 2, "a": i}); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			tx.Rollback(ctx)
		} else if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var count, maxseq int
	s.DB.QueryRow(ctx, `SELECT count(*),max(sequence) FROM audit_events`).Scan(&count, &maxseq)
	if count != 2 || maxseq != 2 {
		t.Fatal("audit rollback created sequence gap")
	}
	if _, err := s.DB.Exec(ctx, `DELETE FROM audit_events`); err == nil {
		t.Fatal("audit history deleted")
	}
}
func TestAuthorizationAndOrigin(t *testing.T) {
	s := testDB(t)
	u, _ := seed(t, s)
	token := "session-test-token"
	if _, err := s.DB.Exec(context.Background(), `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)`, hash(token), u.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	handler := s.Handler()
	for _, tc := range []struct {
		origin string
		cookie bool
		want   int
	}{{"https://evil.example", true, 403}, {"", true, 403}, {"http://localhost:3000", false, 403}} {
		r := httptest.NewRequest(http.MethodPost, "/v1/flags", strings.NewReader("{}"))
		r.Header.Set("Origin", tc.origin)
		if tc.cookie {
			r.AddCookie(&http.Cookie{Name: "sl_session", Value: token})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("origin=%s cookie=%v status=%d", tc.origin, tc.cookie, w.Code)
		}
	}
}
