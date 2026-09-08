package platform

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/http/httptest"
	"strings"
	"testing"
)

func reviewV2Manifest() protocol.Manifest {
	m := reviewTestManifest()
	m.APIVersion = protocol.ManifestV2
	m.Validator.Profile = "native-evaluator-v2"
	m.Metric.Name = "checks"
	m.Evaluation = &protocol.EvaluationContract{Version: protocol.EvaluationVersion, Mode: "artifact", ComparisonID: "test-scientific-v2", Executor: protocol.ExecutorRequirements{OS: "linux", Architecture: "amd64", Accelerator: "none", Features: []string{"isolated-checker"}}, Measurements: []protocol.MeasurementDefinition{
		{Name: "checks", Type: "integer", Unit: m.Metric.Unit, Role: "primary", Interpretation: "direct", Definition: "An explicitly synthetic test score."},
		{Name: "certificate", Type: "integer", Unit: "boolean", Role: "constraint", Interpretation: "direct", Definition: "Test evidence for the exact milestone predicate.", Minimum: "0", Maximum: "1"},
	}, Rationale: protocol.MetricRationale{Objective: "Test typed evidence through the full platform pipeline.", ImprovementMeaning: "This fixture tests software and makes no scientific claim.", EvidenceURLs: []string{m.Evidence[0].URL}, PreservedConditions: []string{"Every accepted signed result retains its named measurements."}, BaselineReason: "Synthetic values test exact platform behavior only.", MeaningfulDelta: "One test unit has no scientific significance.", ProxyAttacks: []string{"A primary score alone cannot claim a certificate milestone."}, PermittedClaim: "The specified platform integration test passed.", ExcludedClaims: []string{"This fixture demonstrates no actual scientific result."}}}
	return m
}

func TestV2ReviewIncludesScientificModulesAndPublicCases(t *testing.T) {
	m := reviewV2Manifest()
	if err := protocol.ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
	snapshot := reviewTestSnapshot(m)
	for name, text := range map[string]string{"validator/elasticity.py": "# CORE PHYSICAL CHECK", "validator/test_science.py": "# ANALYTIC SCIENCE TEST", "suite/public.json": `{"case":"PUBLIC CASE"}`, "requirements.lock": "# LOCKED DEPENDENCIES", "validator/private/key.py": "DO NOT SEND PRIVATE KEY"} {
		snapshot.Files[name] = []byte(text)
	}
	document := snapshotBytes(snapshot)
	got, err := selectReviewEvidence(document, protocol.DigestBytes(document), snapshot.Commit, 1, m)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw(got))
	for _, value := range []string{"CORE PHYSICAL CHECK", "ANALYTIC SCIENCE TEST", "PUBLIC CASE", "LOCKED DEPENDENCIES"} {
		if !strings.Contains(text, value) {
			t.Fatal("missing scientific evidence", value)
		}
	}
	if strings.Contains(text, "DO NOT SEND") {
		t.Fatal("private instructions reached reviewer")
	}
}

func TestV2SignedConfirmationAdjudicationAndExport(t *testing.T) {
	s := testDB(t)
	u, old := seed(t, s, protocol.VerificationPlatform)
	version := ID()
	ctx := context.Background()
	key, pub := testKey(t)
	host := runnerIdentity{ID: "r1", Group: "group1", PublicKey: pub}
	m := reviewV2Manifest()
	m.VerificationPolicy = protocol.VerificationPlatform
	m.Milestones = []protocol.Milestone{{ID: "v2-m1", Title: "First", ThresholdTicks: "10"}, {ID: "v2-m2", Title: "Certificate", ThresholdTicks: "20", Requires: []protocol.MeasurementPredicate{{Measurement: "certificate", Operator: "eq", Value: "1"}}}}
	c := protocol.ExecutorCapabilities{OS: "linux", Architecture: "amd64", Accelerator: "none", RuntimeImageDigest: m.Validator.RuntimeImageDigest, Features: m.Evaluation.Executor.Features, MaxVCPU: 4, MaxMemoryMB: 8192, MaxSessionSeconds: 600, MaxJobSeconds: 7200}
	lock := protocol.Lock{VerificationPolicy: protocol.VerificationPlatform, Manifest: m, ExecutionProfileDigest: "sha256:1111111111111111111111111111111111111111111111111111111111111111", ValidatorDiskDigest: protocol.DigestBytes([]byte("validator")), SuiteDigest: protocol.DigestBytes([]byte("suite")), SuiteDiskDigest: protocol.DigestBytes([]byte("suite"))}
	digest, err := protocol.Digest(lock)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE challenge_versions SET status='superseded',intake_status='closed' WHERE id=$1`, []any{old}},
		{`INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,status,intake_status,deadline,lock_digest) SELECT $1,challenge_id,repository,repository_id,source_commit,source_digest,$2,'published','open',deadline,$3 FROM challenge_versions WHERE id=$4`, []any{version, raw(m), digest, old}},
		{`INSERT INTO locks(digest,version_id,document) VALUES($1,$2,$3)`, []any{digest, version, raw(lock)}},
		{`INSERT INTO milestone_tiers(id,version_id,title,threshold_ticks) VALUES('v2-m1',$1,'First',10),('v2-m2',$1,'Certificate',20)`, []any{version}},
		{`UPDATE runner_hosts SET purposes='{submission,confirmation}',public_key=$1 WHERE id='r1'`, []any{pub}},
		{`UPDATE runner_hosts SET enabled=false WHERE id='r2'`, nil},
		{`INSERT INTO runner_authorization_enrollments(host_id,config_digest,template,enabled,approval_reason) VALUES('r1','sha256:2222222222222222222222222222222222222222222222222222222222222222',$1,true,'Test-only commissioned fixture')`, []any{raw(map[string]any{"hostId": "r1", "configDigest": "sha256:2222222222222222222222222222222222222222222222222222222222222222", "executionProfileDigest": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "capabilities": c})}},
		{`INSERT INTO runner_profiles(host_id,execution_profile_digest,config_digest,capabilities,advisory_snapshot_digest,runtime_inventory_digest,enabled) VALUES('r1','sha256:1111111111111111111111111111111111111111111111111111111111111111','sha256:2222222222222222222222222222222222222222222222222222222222222222',$1,'sha256:3333333333333333333333333333333333333333333333333333333333333333','sha256:4444444444444444444444444444444444444444444444444444444444444444',true)`, []any{raw(c)}},
	} {
		if _, err = s.DB.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	_, body, err := accept(t, s, u, readyIntent(t, s, u, version, 777), "typed-pipeline-accept")
	if err != nil {
		t.Fatal(err)
	}
	sid := body["submissionId"].(string)
	if err = s.queueSubmission(ctx, sid); err != nil {
		t.Fatal(err)
	}
	if err = testFinishSignedRun(t, s, version, sid, "submission", "25", host, key, map[string]string{"checks": "999", "certificate": "1"}); err == nil {
		t.Fatal("forged primary measurement accepted")
	}
	if err = testFinishSignedRun(t, s, version, sid, "submission", "25", host, key, map[string]string{"checks": "25", "certificate": "1"}); err != nil {
		t.Fatal(err)
	}
	if err = testFinishSignedRun(t, s, version, sid, "confirmation", "25", host, key, map[string]string{"checks": "25", "certificate": "0"}); err != nil {
		t.Fatal(err)
	}
	if err = s.adjudicate(ctx, version); err != nil {
		t.Fatal(err)
	}
	var claim string
	var count int
	if err = s.DB.QueryRow(ctx, `SELECT count(*),min(milestone_id) FROM milestone_claims WHERE submission_id=$1`, sid).Scan(&count, &claim); err != nil || count != 1 || claim != "v2-m1" {
		t.Fatalf("scalar score bypassed missing confirmation predicate: %d %s %v", count, claim, err)
	}
	// Sign the persisted records through the same signer path used by the worker.
	rows, err := s.DB.Query(ctx, `SELECT digest FROM receipts WHERE envelope IS NULL`)
	if err != nil {
		t.Fatal(err)
	}
	var digests []string
	for rows.Next() {
		var d string
		rows.Scan(&d)
		digests = append(digests, d)
	}
	rows.Close()
	s.ReceiptSigner = key
	s.Config.ReceiptKeyID = "test-platform"
	for _, d := range digests {
		if err = s.signReceipt(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest("GET", "/v1/exports/challenge-versions/"+version, nil)
	request.SetPathValue("id", version)
	w := httptest.NewRecorder()
	if err = s.exportChallenge(w, request, nil); err != nil {
		t.Fatal(err)
	}
	var exported struct {
		Receipts    []protocol.Envelope `json:"receipts"`
		Submissions []json.RawMessage   `json:"submissions"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, envelope := range exported.Receipts {
		var run protocol.RunReceipt
		if json.Unmarshal(decodeTypedTestEnvelope(t, envelope), &run) == nil && run.Kind == "ValidationRunReceipt" {
			if run.ValidatorResult == nil || run.ValidatorResult.ComparisonID != m.Evaluation.ComparisonID {
				t.Fatal("export lost typed evidence")
			}
			found++
		}
	}
	if found != 2 || len(exported.Submissions) != 1 {
		t.Fatalf("export lost confirmed submission: runs=%d submissions=%d", found, len(exported.Submissions))
	}
}

func decodeTypedTestEnvelope(t *testing.T, e protocol.Envelope) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(e.Payload)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
