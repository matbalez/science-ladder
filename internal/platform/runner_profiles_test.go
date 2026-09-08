package platform

import (
	"context"
	"encoding/json"
	"github.com/matbalez/science-ladder/internal/runner"
	"github.com/matbalez/science-ladder/internal/storage"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestMultipleProfilesAreEnrolledAndRoutedOnOneHost(t *testing.T) {
	s := testDB(t)
	u, version := seed(t, s, protocol.VerificationPlatform)
	ctx := context.Background()
	key, public := testKey(t)
	s.ReceiptSigner = key
	s.Config.ReceiptKeyID = "test"
	profile, config := protocol.DigestBytes([]byte("native profile")), protocol.DigestBytes([]byte("native config"))
	advisory, inventory := protocol.DigestBytes([]byte("native advisory")), protocol.DigestBytes([]byte("native inventory"))
	c := protocol.ExecutorCapabilities{OS: "linux", Architecture: "amd64", Accelerator: "none", RuntimeImageDigest: protocol.DigestBytes([]byte("native runtime")), MaxVCPU: 4, MaxMemoryMB: 8192, MaxSessionSeconds: 600, Features: []string{"isolated-candidate"}}
	a := runner.HostAttestation{HostID: "r1", HostGroup: "group1", PhysicalHostID: "physical1", ExecutionProfileDigest: profile, ConfigDigest: config, RunnerEpoch: "1", ExclusivePhysicalHost: true, EgressPolicyVerified: true, ExpiresAt: time.Now().Add(time.Hour), Capabilities: &c}
	if _, err := s.DB.Exec(ctx, `UPDATE runner_hosts SET public_key=$1,purposes='{preflight,artifact_prepare,submission,confirmation}' WHERE id='r1'`, public); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO runner_authorization_enrollments(host_id,config_digest,template,enabled,approval_reason) VALUES('r1',$1,$2,true,'Test operator commissioned this native profile')`, config, raw(a)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO runner_profiles(host_id,execution_profile_digest,config_digest,capabilities,advisory_snapshot_digest,runtime_inventory_digest,enabled) VALUES('r1',$1,$2,$3,$4,$5,true)`, profile, config, raw(c), advisory, inventory); err != nil {
		t.Fatal(err)
	}
	state := runnerTestTLS(t, s, "r1")
	identity := runnerIdentity{ID: "r1", Group: "group1", PublicKey: public, ExecutionProfile: "profile"}
	native, err := s.selectRunnerProfile(ctx, identity, profile)
	if err != nil || native.Capabilities == nil || native.ExecutionProfile != profile {
		t.Fatalf("select: %+v %v", native, err)
	}
	if _, err = s.selectRunnerProfile(ctx, identity, protocol.DigestBytes([]byte("injected profile"))); err == nil {
		t.Fatal("unenrolled profile selected")
	}
	other := identity
	other.ID = "r2"
	if _, err = s.selectRunnerProfile(ctx, other, profile); err == nil {
		t.Fatal("another host borrowed profile")
	}
	m := protocol.Manifest{Validator: protocol.Validator{RuntimeImageDigest: c.RuntimeImageDigest}, Resources: protocol.Resources{VCPU: 1, MemoryMB: 2048, TimeoutSeconds: 120}, Suite: protocol.Suite{Visibility: "public"}, Evaluation: &protocol.EvaluationContract{Executor: protocol.ExecutorRequirements{OS: "linux", Architecture: "amd64", Accelerator: "none", Features: []string{"isolated-candidate"}}}}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.requireExecutor(ctx, tx, m, profile); err != nil {
		t.Fatal(err)
	}
	m.Evaluation.Executor.OS = "darwin"
	m.Evaluation.Executor.Accelerator = "metal"
	if err = s.requireExecutor(ctx, tx, m, profile); err == nil {
		t.Fatal("Linux silently satisfied Metal challenge")
	}
	m.Evaluation.Executor.OS = "linux"
	m.Evaluation.Executor.Accelerator = "none"
	tx.Rollback(ctx)
	t.Setenv("AWS_ACCESS_KEY_ID", "test-only-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-only-secret-key")
	t.Setenv("AWS_SESSION_TOKEN", "")
	s.Store, err = storage.New(ctx, "test-only", "us-east-1", "https://objects.invalid")
	if err != nil {
		t.Fatal(err)
	}
	disk := protocol.DigestBytes([]byte("native test object"))
	if _, err = s.DB.Exec(ctx, `INSERT INTO artifacts(digest,blob_digest,size,media_type,owner_id) VALUES($1,$1,1,'application/octet-stream',$2)`, disk, u.ID); err != nil {
		t.Fatal(err)
	}
	job := protocol.RunnerJob{ID: ID(), APIVersion: protocol.APIVersion, Kind: "ValidationJob", Purpose: "artifact_prepare", VerificationPolicy: protocol.VerificationPlatform, Manifest: m, SourceSnapshot: &protocol.ObjectRef{Digest: disk}}
	if _, err = s.DB.Exec(ctx, `INSERT INTO runner_jobs(id,purpose,version_id,payload) VALUES($1,'artifact_prepare',$2,$3)`, job.ID, version, raw(job)); err != nil {
		t.Fatal(err)
	}
	claim := func(profile string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/internal/v1/runner/jobs/claim?profile="+url.QueryEscape(profile), strings.NewReader(`{"purposes":["artifact_prepare"]}`))
		r.TLS = state
		w := httptest.NewRecorder()
		s.RunnerHandler().ServeHTTP(w, r)
		return w
	}
	w := claim("")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("legacy claimed native: %d %s", w.Code, w.Body)
	}
	w = claim(profile)
	if w.Code != 200 || strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("native not routed: %d %s", w.Code, w.Body)
	}
	var response map[string]json.RawMessage
	if json.Unmarshal(w.Body.Bytes(), &response) != nil {
		t.Fatal("invalid response")
	}
	r := httptest.NewRequest("POST", "/internal/v1/runner/authorization/renew", strings.NewReader(string(raw(map[string]string{"configDigest": config}))))
	r.TLS = state
	w = httptest.NewRecorder()
	s.RunnerHandler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("native renewal: %d %s", w.Code, w.Body)
	}
	if _, err = s.DB.Exec(ctx, `UPDATE runner_profiles SET capabilities='{}'`); err == nil {
		t.Fatal("profile capabilities mutable")
	}
	if _, err = s.DB.Exec(ctx, `UPDATE runner_profiles SET enabled=false`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.selectRunnerProfile(ctx, identity, profile); err == nil {
		t.Fatal("revoked profile selected")
	}
	w = claim(profile)
	if w.Code != 403 {
		t.Fatal("revoked profile claimed work")
	}
	r = httptest.NewRequest("POST", "/internal/v1/runner/authorization/renew", strings.NewReader(string(raw(map[string]string{"configDigest": config}))))
	r.TLS = state
	w = httptest.NewRecorder()
	s.RunnerHandler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("revoked profile renewed: %d", w.Code)
	}
}
