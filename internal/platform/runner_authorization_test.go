package platform

import (
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/internal/runner"
	"github.com/matbalez/science-ladder/internal/storage"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

func TestRequestedRunnerPurposes(t *testing.T) {
	for _, test := range []struct {
		body string
		want []string
		bad  bool
	}{
		{`{}`, []string{"submission", "confirmation"}, false},
		{`{"purposes":[]}`, []string{}, false},
		{`{"purposes":["preflight","submission"]}`, []string{"submission"}, false},
		{`{"purposes":["artifact_prepare"]}`, []string{}, false},
		{`{"purposes":["submission","submission"]}`, nil, true},
		{`{"purposes":["bogus"]}`, nil, true},
		{`{"purposes":null}`, nil, true},
		{`{"purposes":"submission"}`, nil, true},
		{`{"purposes":[1]}`, nil, true},
		{`{"purposes":[null]}`, nil, true},
		{`{"purposes":[],"extra":true}`, nil, true},
		{`{} {}`, nil, true},
		{`null`, nil, true},
	} {
		t.Run(test.body, func(t *testing.T) {
			got, err := requestedRunnerPurposes(httptest.NewRequest("POST", "/", strings.NewReader(test.body)), []string{"submission", "confirmation"})
			if (err != nil) != test.bad || !test.bad && !reflect.DeepEqual(got, test.want) {
				t.Fatalf("purposes=%v error=%v", got, err)
			}
		})
	}
}

func runnerTestTLS(t *testing.T, s *Server, hostID string) *tls.ConnectionState {
	t.Helper()
	// These tests inject the verified state supplied by Go's mTLS listener; no
	// request header or request body can populate TLS.VerifiedChains in service.
	cert := &x509.Certificate{Raw: []byte("test verified runner certificate " + hostID)}
	fingerprint := sha256.Sum256(cert.Raw)
	if _, err := s.DB.Exec(context.Background(), `UPDATE runner_hosts SET certificate_fingerprint=$2 WHERE id=$1`, hostID, hex.EncodeToString(fingerprint[:])); err != nil {
		t.Fatal(err)
	}
	return &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{cert}}, PeerCertificates: []*x509.Certificate{cert}}
}

func TestRunnerAuthorizationRenewalIsBoundedAndEnrolled(t *testing.T) {
	s := testDB(t)
	seed(t, s)
	key, public := testKey(t)
	s.ReceiptSigner = key
	s.Config.ReceiptKeyID = "platform-test"
	ctx := context.Background()
	profile := protocol.DigestBytes([]byte("approved profile"))
	digest := protocol.DigestBytes([]byte("approved config"))
	template := runner.HostAttestation{HostID: "r1", HostGroup: "group1", PhysicalHostID: "physical1", ExecutionProfileDigest: profile, ConfigDigest: digest, RunnerEpoch: "1", ExclusivePhysicalHost: true, EgressPolicyVerified: true, ExpiresAt: time.Now().UTC().Add(-time.Hour)}
	if _, err := s.DB.Exec(ctx, `UPDATE runner_hosts SET execution_profile_digest=$1,public_key=$2 WHERE id='r1'`, profile, public); err != nil {
		t.Fatal(err)
	}
	state := runnerTestTLS(t, s, "r1")
	request := func(body string, tlsState *tls.ConnectionState) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/internal/v1/runner/authorization/renew", strings.NewReader(body))
		r.TLS = tlsState
		w := httptest.NewRecorder()
		s.RunnerHandler().ServeHTTP(w, r)
		return w
	}
	body := string(raw(map[string]string{"configDigest": digest}))
	if w := request(body, nil); w.Code != 401 {
		t.Fatalf("renewal without mTLS: %d %s", w.Code, w.Body.String())
	}
	if w := request(body, state); w.Code != 403 {
		t.Fatalf("unenrolled renewal: %d %s", w.Code, w.Body.String())
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO runner_authorization_enrollments(host_id,config_digest,template,enabled,approval_reason) VALUES('r1',$1,$2,true,'Test operator approved exact commissioned host configuration')`, digest, raw(template)); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC()
	w := request(body, state)
	if w.Code != 200 {
		t.Fatalf("enrolled renewal: %d %s", w.Code, w.Body.String())
	}
	var response struct {
		Attestation protocol.Envelope `json:"attestation"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	payload, err := protocol.Verify(response.Attestation, map[string]crypto.PublicKey{"platform-test": key.Public()})
	if err != nil {
		t.Fatal(err)
	}
	var renewed runner.HostAttestation
	if err := protocol.DecodeStrict(payload, &renewed); err != nil {
		t.Fatal(err)
	}
	if renewed.ExpiresAt.Before(before.Add(24*time.Hour)) || renewed.ExpiresAt.After(time.Now().Add(24*time.Hour)) {
		t.Fatal("renewal was not bounded to 24 hours")
	}
	renewed.ExpiresAt = template.ExpiresAt
	if renewed != template {
		t.Fatal("renewal changed previously approved host assertions")
	}
	var count, events int
	if err := s.DB.QueryRow(ctx, `SELECT (SELECT count(*) FROM runner_authorization_renewals),(SELECT count(*) FROM audit_events WHERE kind='runner.authorization_renewed')`).Scan(&count, &events); err != nil || count != 1 || events != 1 {
		t.Fatalf("renewal evidence missing: %d/%d %v", count, events, err)
	}
	if _, err := s.DB.Exec(ctx, `UPDATE runner_authorization_enrollments SET template=jsonb_set(template,'{runnerEpoch}','"2"')`); err == nil {
		t.Fatal("approved template was mutable")
	}
	if _, err := s.DB.Exec(ctx, `DELETE FROM runner_authorization_renewals`); err == nil {
		t.Fatal("renewal evidence was mutable")
	}
	for _, bad := range []string{`{}`, `null`, `{"configDigest":"bad"}`, body + `{}`, `{"configDigest":"` + digest + `","physicalHostId":"injected"}`} {
		if w := request(bad, state); w.Code != 400 {
			t.Fatalf("malformed renewal allowed: %s %d", bad, w.Code)
		}
	}
	if w := request(string(raw(map[string]string{"configDigest": protocol.DigestBytes([]byte("different config"))})), state); w.Code != 403 {
		t.Fatal("caller selected an unenrolled config")
	}
	if _, err := s.DB.Exec(ctx, `UPDATE runner_hosts SET host_group='moved-group' WHERE id='r1'`); err != nil {
		t.Fatal(err)
	}
	if w := request(body, state); w.Code != 403 {
		t.Fatal("changed host identity renewed")
	}
	if _, err := s.DB.Exec(ctx, `UPDATE runner_hosts SET host_group='group1',enabled=false WHERE id='r1'`); err != nil {
		t.Fatal(err)
	}
	if w := request(body, state); w.Code != 403 {
		t.Fatal("disabled host renewed")
	}
	if _, err := s.DB.Exec(ctx, `UPDATE runner_hosts SET enabled=true WHERE id='r1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `UPDATE runner_authorization_enrollments SET enabled=false`); err != nil {
		t.Fatal(err)
	}
	if w := request(body, state); w.Code != 403 {
		t.Fatal("revoked config renewed")
	}
	publicRequest := httptest.NewRequest("POST", "/internal/v1/runner/authorization/renew", strings.NewReader(body))
	publicRequest.TLS = state
	publicResponse := httptest.NewRecorder()
	s.Handler().ServeHTTP(publicResponse, publicRequest)
	if publicResponse.Code != http.StatusNotFound {
		t.Fatalf("renewal exposed on public API: %d", publicResponse.Code)
	}
}

func TestRunnerFilteredClaimPreservesSolverFlow(t *testing.T) {
	s := testDB(t)
	u, version := seed(t, s, protocol.VerificationPlatform)
	ctx := context.Background()
	key, _ := testKey(t)
	s.ReceiptSigner = key
	s.Config.ReceiptKeyID = "platform-test"
	t.Setenv("AWS_ACCESS_KEY_ID", "test-only-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-only-secret-key")
	t.Setenv("AWS_SESSION_TOKEN", "")
	var err error
	s.Store, err = storage.New(ctx, "test-only", "us-east-1", "https://objects.invalid")
	if err != nil {
		t.Fatal(err)
	}
	state := runnerTestTLS(t, s, "r1")
	advisory, inventory := protocol.DigestBytes([]byte("old advisory")), protocol.DigestBytes([]byte("inventory"))
	if _, err := s.DB.Exec(ctx, `UPDATE runner_hosts SET purposes='{preflight,artifact_prepare,submission,confirmation}',advisory_snapshot_digest=$1,runtime_inventory_digest=$2 WHERE id='r1'`, advisory, inventory); err != nil {
		t.Fatal(err)
	}
	disk := protocol.DigestBytes([]byte("fixed test object"))
	if _, err := s.DB.Exec(ctx, `INSERT INTO artifacts(digest,blob_digest,size,media_type,owner_id) VALUES($1,$1,1,'application/octet-stream',$2)`, disk, u.ID); err != nil {
		t.Fatal(err)
	}
	ref := protocol.ObjectRef{Digest: disk}
	ids := map[string]string{}
	for i, purpose := range []string{"preflight", "artifact_prepare", "submission", "confirmation"} {
		job := protocol.RunnerJob{ID: ID(), Purpose: purpose, APIVersion: protocol.APIVersion, Kind: "ValidationJob", VerificationPolicy: protocol.VerificationPlatform, ExecutionProfileDigest: "profile", RunnerEpoch: "1", FencingToken: 1, SourceSnapshot: &ref, ValidatorDisk: ref, SubmissionDisk: ref, SuiteDisk: ref, ChallengeDisk: ref}
		ids[purpose] = job.ID
		if _, err := s.DB.Exec(ctx, `INSERT INTO runner_jobs(id,purpose,version_id,payload,created_at) VALUES($1,$2,$3,$4,$5)`, job.ID, purpose, version, raw(job), time.Now().Add(time.Duration(i-10)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	claim := func(body string) *protocol.RunnerJob {
		r := httptest.NewRequest("POST", "/internal/v1/runner/jobs/claim", strings.NewReader(body))
		r.TLS = state
		w := httptest.NewRecorder()
		s.RunnerHandler().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("claim: %d %s", w.Code, w.Body.String())
		}
		var result struct {
			Job *protocol.Envelope `json:"job"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Job == nil {
			return nil
		}
		data, err := protocol.Verify(*result.Job, map[string]crypto.PublicKey{"platform-test": key.Public()})
		if err != nil {
			t.Fatal(err)
		}
		var job protocol.RunnerJob
		if err := protocol.DecodeStrict(data, &job); err != nil {
			t.Fatal(err)
		}
		return &job
	}
	if claim(`{"purposes":[]}`) != nil {
		t.Fatal("explicit empty purposes claimed a job")
	}
	for _, purpose := range []string{"artifact_prepare", "submission", "confirmation"} {
		job := claim(`{"purposes":["artifact_prepare","submission","confirmation"]}`)
		if job == nil || job.ID != ids[purpose] || job.Purpose != purpose {
			t.Fatalf("solver flow did not preserve %s: %#v", purpose, job)
		}
	}
	var status string
	var attempts int
	if err := s.DB.QueryRow(ctx, `SELECT status,attempts FROM runner_jobs WHERE id=$1`, ids["preflight"]).Scan(&status, &attempts); err != nil || status != "queued" || attempts != 0 {
		t.Fatalf("excluded preflight consumed a lease: %s %d %v", status, attempts, err)
	}
	if _, err := s.DB.Exec(ctx, `UPDATE runner_hosts SET purposes='{submission}' WHERE id='r1'`); err != nil {
		t.Fatal(err)
	}
	if claim(`{"purposes":["preflight"]}`) != nil {
		t.Fatal("caller broadened enrolled purposes")
	}
	if _, err := s.DB.Exec(ctx, `UPDATE runner_hosts SET purposes='{preflight}' WHERE id='r1'`); err != nil {
		t.Fatal(err)
	}
	if job := claim(`{}`); job == nil || job.ID != ids["preflight"] {
		t.Fatal("legacy omitted purpose filter no longer uses enrollment")
	}
}
