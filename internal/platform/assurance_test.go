package platform

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func testFinishSignedRun(t *testing.T, s *Server, version, submission, purpose, score string, host runnerIdentity, key *ecdsa.PrivateKey) error {
	t.Helper()
	ctx := context.Background()
	var jobData []byte
	if err := s.DB.QueryRow(ctx, `SELECT payload FROM runner_jobs WHERE submission_id=$1 AND purpose=$2`, submission, purpose).Scan(&jobData); err != nil {
		t.Fatal(err)
	}
	var job protocol.RunnerJob
	if err := json.Unmarshal(jobData, &job); err != nil {
		t.Fatal(err)
	}
	job.ExpiresAt = time.Now().UTC().Add(time.Hour)
	job.RequiredHostGroup = host.Group
	token := ID()
	if _, err := s.DB.Exec(ctx, `UPDATE runner_jobs SET payload=$2,status='running',host_id=$3,result_token_hash=$4,lease_expires_at=$5 WHERE id=$1`, job.ID, raw(job), host.ID, hash(token), job.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	digest, _ := protocol.Digest(job)
	run := protocol.RunReceipt{VerificationPolicy: job.VerificationPolicy, APIVersion: protocol.APIVersion, Kind: "ValidationRunReceipt", ID: ID(), CreatedAt: time.Now().UTC(), Producer: host.ID, JobID: job.ID, JobDigest: digest, AcceptanceReceiptDigest: job.AcceptanceReceiptDigest, ChallengeLockDigest: job.ChallengeLockDigest, ArtifactDigest: job.ArtifactDigest, SuiteDigest: job.SuiteDigest, ExecutionProfileDigest: job.ExecutionProfileDigest, RunnerEpoch: job.RunnerEpoch, FencingToken: job.FencingToken, HostID: host.ID, HostGroup: host.Group, Official: true, CleanupAttested: true, DeploymentMode: job.DeploymentMode, OfficialAcceptance: job.OfficialAcceptance, Outcome: "valid", ScoreTicks: score, Gates: map[string]bool{}}
	for _, g := range job.Manifest.HardGates {
		run.Gates[g] = true
	}
	envelope, err := protocol.Sign(host.ID, key, run)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/internal/v1/runner/jobs/"+job.ID+"/result", strings.NewReader(string(raw(map[string]any{"envelope": envelope, "resultToken": token}))))
	req.SetPathValue("id", job.ID)
	return s.runnerResult(httptest.NewRecorder(), req, host)
}

func TestSingleHostPlatformVerificationPreservesReceiptOrdering(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s, protocol.VerificationPlatform)
	ctx := context.Background()
	key, pub := testKey(t)
	host := runnerIdentity{ID: "r1", Group: "group1", PublicKey: pub}
	if _, err := s.DB.Exec(ctx, `UPDATE runner_hosts SET enabled=false WHERE id='r2'`); err != nil {
		t.Fatal(err)
	}
	var submissions []string
	for i := 0; i < 2; i++ {
		_, body, err := accept(t, s, u, readyIntent(t, s, u, v, 900+i), fmt.Sprintf("platform-accept-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		sid := body["submissionId"].(string)
		submissions = append(submissions, sid)
		if err = s.queueSubmission(ctx, sid); err != nil {
			t.Fatal(err)
		}
	}
	// Complete the later receipt first. It cannot claim a milestone yet.
	for _, i := range []int{1, 0} {
		sid := submissions[i]
		score := []string{"30", "40"}[i]
		if err := testFinishSignedRun(t, s, v, sid, "submission", score, host, key); err != nil {
			t.Fatal(err)
		}
		var excluded *string
		var data []byte
		if err := s.DB.QueryRow(ctx, `SELECT excluded_group,payload FROM runner_jobs WHERE submission_id=$1 AND purpose='confirmation'`, sid).Scan(&excluded, &data); err != nil {
			t.Fatal(err)
		}
		var job protocol.RunnerJob
		json.Unmarshal(data, &job)
		if excluded != nil || len(job.ExcludedHostIDs) != 0 {
			t.Fatal("platform confirmation unnecessarily excludes its single host")
		}
		if err := testFinishSignedRun(t, s, v, sid, "confirmation", score, host, key); err != nil {
			t.Fatal(err)
		}
		if err := s.adjudicate(ctx, v); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			var watermark int
			s.DB.QueryRow(ctx, `SELECT watermark FROM challenge_versions WHERE id=$1`, v).Scan(&watermark)
			if watermark != 0 {
				t.Fatal("later result passed unresolved earlier receipt")
			}
		}
	}
	var claims int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM milestone_claims WHERE submission_id=$1`, submissions[0]).Scan(&claims); err != nil {
		t.Fatal(err)
	}
	if claims != 2 {
		t.Fatalf("earliest qualifying receipt should win both crossed tiers: %d", claims)
	}
	rows, err := queryObjects(ctx, s.DB, submissionSQL+` WHERE s.version_id=$1 ORDER BY s.sequence`, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range rows {
		var result map[string]any
		json.Unmarshal(data, &result)
		if result["verificationPolicy"] != "platform" || result["verificationStatus"] != "platform_verified" || result["independentReplication"] != false {
			t.Fatalf("false assurance: %s", data)
		}
	}
	var data []byte
	if err = s.DB.QueryRow(ctx, challengeSQL+` WHERE v.id=$1`, v).Scan(&data); err != nil {
		t.Fatal(err)
	}
	var challenge map[string]any
	json.Unmarshal(data, &challenge)
	if challenge["verificationPolicy"] != "platform" {
		t.Fatal("public challenge omits frozen policy")
	}
}

func TestIndependentAndLegacyContractsStillRequireTwoHosts(t *testing.T) {
	for _, policy := range []string{"", protocol.VerificationIndependent} {
		t.Run("policy_"+policy, func(t *testing.T) {
			s := testDB(t)
			u, v := seed(t, s, policy)
			s.DB.Exec(context.Background(), `UPDATE runner_hosts SET enabled=false WHERE id='r2'`)
			if _, _, err := accept(t, s, u, readyIntent(t, s, u, v, 940), "independent-one-host"); err == nil {
				t.Fatal("old or independent contract accepted one host")
			}
			var count int
			s.DB.QueryRow(context.Background(), `SELECT count(*) FROM submissions`).Scan(&count)
			if count != 0 {
				t.Fatal("failed admission allocated receipt order")
			}
		})
	}
	s := testDB(t)
	u, v := seed(t, s, protocol.VerificationIndependent)
	_, body, err := accept(t, s, u, readyIntent(t, s, u, v, 960), "independent-two-hosts")
	if err != nil {
		t.Fatal(err)
	}
	sid := body["submissionId"].(string)
	if err = s.queueSubmission(context.Background(), sid); err != nil {
		t.Fatal(err)
	}
	key, pub := testKey(t)
	host := runnerIdentity{ID: "r1", Group: "group1", PublicKey: pub}
	if err = testFinishSignedRun(t, s, v, sid, "submission", "30", host, key); err != nil {
		t.Fatal(err)
	}
	if err = testFinishSignedRun(t, s, v, sid, "confirmation", "30", host, key); err == nil {
		t.Fatal("same physical group forged independent replication")
	}
	host2 := runnerIdentity{ID: "r2", Group: "group2", PublicKey: pub}
	if err = testFinishSignedRun(t, s, v, sid, "confirmation", "30", host2, key); err != nil {
		t.Fatal(err)
	}
	if err = s.adjudicate(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	var status string
	if err = s.DB.QueryRow(context.Background(), `SELECT r.payload->>'verificationStatus' FROM submissions s JOIN receipts r ON r.digest=s.adjudication_digest WHERE s.id=$1`, sid).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "independently_replicated" {
		t.Fatal("independent result missing assurance")
	}
}

func TestSignedFixtureRepeatsBoundToParent(t *testing.T) {
	key, pub := testKey(t)
	host := runnerIdentity{ID: "r1", Group: "group1", PublicKey: pub}
	job := protocol.RunnerJob{ID: ID(), CreatedAt: time.Now().UTC().Add(-time.Minute), VerificationPolicy: "platform", ExecutionProfileDigest: "profile", RunnerEpoch: "1", FencingToken: 1, DeploymentMode: "local"}
	parentDigest, _ := protocol.Digest(job)
	makeParent := func() protocol.RunReceipt {
		parent := protocol.RunReceipt{ID: ID(), BuildReport: &protocol.BuildReport{SuiteDigest: "suite", FreshVMRuns: 4}}
		for _, name := range []string{"baseline", "valid"} {
			fixture := protocol.FixtureReport{Name: name, Stage: "isolated_execution", FreshVMRuns: 2, Outcome: "valid", ScoreTicks: "1"}
			for i := 0; i < 2; i++ {
				child := protocol.RunReceipt{APIVersion: protocol.APIVersion, Kind: "ValidationRunReceipt", ID: ID(), JobID: ID(), JobDigest: protocol.DigestBytes([]byte(ID())), ParentJobDigest: parentDigest, ArtifactDigest: protocol.DigestBytes([]byte(name)), SuiteDigest: "suite", CreatedAt: time.Now().UTC(), VerificationPolicy: "platform", ExecutionProfileDigest: "profile", RunnerEpoch: "1", FencingToken: 1, HostID: host.ID, HostGroup: host.Group, Official: true, CleanupAttested: true, DeploymentMode: "local", Outcome: "valid", ScoreTicks: "1"}
				env, e := protocol.Sign(host.ID, key, child)
				if e != nil {
					t.Fatal(e)
				}
				fixture.RunReceipts = append(fixture.RunReceipts, env)
			}
			parent.BuildReport.Fixtures = append(parent.BuildReport.Fixtures, fixture)
		}
		parent.CreatedAt = time.Now().UTC()
		return parent
	}
	valid := makeParent()
	if err := validateFixtureEvidence(job, valid, host); err != nil {
		t.Fatal(err)
	}
	replay := makeParent()
	replay.BuildReport.Fixtures[0].RunReceipts[1] = replay.BuildReport.Fixtures[0].RunReceipts[0]
	if err := validateFixtureEvidence(job, replay, host); err == nil {
		t.Fatal("one signed attempt replayed as two fresh VMs")
	}
	modified := job
	modified.ID = ID()
	if err := validateFixtureEvidence(modified, valid, host); err == nil {
		t.Fatal("signed child replayed into different parent")
	}
	noProof := makeParent()
	noProof.BuildReport.Fixtures[0].RunReceipts = nil
	if err := validateFixtureEvidence(job, noProof, host); err == nil {
		t.Fatal("unsubstantiated fresh-VM assertion accepted")
	}
	otherKey, otherPub := testKey(t)
	_ = otherKey
	otherHost := host
	otherHost.PublicKey = otherPub
	if err := validateFixtureEvidence(job, valid, otherHost); err == nil {
		t.Fatal("untrusted fixture signature accepted")
	}
}

// Synthetic build metadata is committed only in an isolated test schema. The
// separate signature tests exercise the required child execution evidence.
func TestPreflightPolicyAndSignedLock(t *testing.T) {
	for _, policy := range []string{protocol.VerificationPlatform, protocol.VerificationIndependent} {
		t.Run(policy, func(t *testing.T) {
			s := testDB(t)
			u, old := seed(t, s)
			ctx := context.Background()
			key, _ := testKey(t)
			s.ReceiptSigner = key
			s.Config.ReceiptKeyID = "test-platform"
			s.DB.Exec(ctx, `UPDATE challenge_versions SET status='closed',intake_status='closed' WHERE id=$1`, old)
			source := protocol.DigestBytes([]byte("test-only-source"))
			version := ID()
			m := protocol.Manifest{VerificationPolicy: policy, APIVersion: protocol.APIVersion, Kind: "Challenge", EconomicMode: "none", Deadline: time.Now().Add(time.Hour), Validator: protocol.Validator{RuntimeImageDigest: protocol.DigestBytes([]byte("runtime"))}, Suite: protocol.Suite{Visibility: "public"}, Fixtures: []protocol.Fixture{{Name: "baseline", ExpectedOutcome: "valid", ExpectedTicks: "1"}, {Name: "valid", ExpectedOutcome: "valid", ExpectedTicks: "1"}}}
			if _, err := s.DB.Exec(ctx, `INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,status,review_status,deadline) SELECT $2,challenge_id,repository,repository_id,source_commit,$3,$4,'machine_preflight','automated_pass',$5 FROM challenge_versions WHERE id=$1`, old, version, source, raw(m), m.Deadline); err != nil {
				t.Fatal(err)
			}
			preflight := ID()
			job := protocol.RunnerJob{ID: ID(), Purpose: "preflight", VerificationPolicy: policy, Manifest: m, SourceSnapshot: &protocol.ObjectRef{Digest: source}, ExecutionProfileDigest: "profile", AdvisorySnapshotDigest: protocol.DigestBytes([]byte("advisory")), RuntimeInventoryDigest: protocol.DigestBytes([]byte("inventory")), DeploymentMode: "local"}
			md, _ := protocol.Digest(m)
			validator := protocol.ObjectRef{Digest: protocol.DigestBytes([]byte("validator")), Size: 1}
			suite := protocol.ObjectRef{Digest: protocol.DigestBytes([]byte("suite")), Size: 1}
			sbom := protocol.ObjectRef{Digest: protocol.DigestBytes([]byte("sbom")), Size: 1}
			b := protocol.BuildReport{Passed: true, ScansPassed: true, OfflineRebuild: true, HostileCorpusPassed: true, ManifestDigest: md, SourceSnapshotDigest: source, ExecutionProfileDigest: "profile", ValidatorDisk: &validator, ChallengeDisk: &validator, SuiteDisk: &suite, SBOM: &sbom, ValidatorDiskDigest: validator.Digest, RebuiltValidatorDiskDigest: validator.Digest, ValidatorImageDigest: m.Validator.RuntimeImageDigest, SuiteDigest: suite.Digest, Fixtures: []protocol.FixtureReport{{Name: "baseline", Passed: true, ExpectedOutcome: "valid", Outcome: "valid", ScoreTicks: "1"}, {Name: "valid", Passed: true, ExpectedOutcome: "valid", Outcome: "valid", ScoreTicks: "1"}}, VulnerabilityScan: &protocol.VulnerabilityScan{Status: "pass", PolicyVersion: "offline-advisory-v1", PackagesChecked: 1, AdvisorySnapshotDigest: job.AdvisorySnapshotDigest, RuntimeInventoryDigest: job.RuntimeInventoryDigest, SBOMDigest: sbom.Digest, ScannedAt: time.Now().UTC()}}
			if _, err := s.DB.Exec(ctx, `INSERT INTO preflights(id,version_id) VALUES($1,$2)`, preflight, version); err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB.Exec(ctx, `INSERT INTO runner_jobs(id,purpose,version_id,preflight_id,payload) VALUES($1,'preflight',$2,$3,$4)`, job.ID, version, preflight, raw(job)); err != nil {
				t.Fatal(err)
			}
			for role, ref := range map[string]protocol.ObjectRef{"validatorDisk": validator, "challengeDisk": validator, "suiteDisk": suite, "sbom": sbom} {
				if _, err := s.DB.Exec(ctx, `INSERT INTO runner_uploads(job_id,role,digest,size,verified) VALUES($1,$2,$3,$4,true)`, job.ID, role, ref.Digest, ref.Size); err != nil {
					t.Fatal(err)
				}
			}
			run := protocol.RunReceipt{VerificationPolicy: policy, HostID: "r1", HostGroup: "group1", Outcome: "valid", BuildReport: &b}
			tx, err := s.DB.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.commitBuild(ctx, tx, job, run, protocol.DigestBytes([]byte("synthetic-build-receipt")), version); err != nil {
				tx.Rollback(ctx)
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			var count int
			var machine, status string
			if err = s.DB.QueryRow(ctx, `SELECT COALESCE(machine_receipt_digest,''),status FROM preflights WHERE id=$1`, preflight).Scan(&machine, &status); err != nil {
				t.Fatal(err)
			}
			s.DB.QueryRow(ctx, `SELECT count(*) FROM runner_jobs WHERE preflight_id=$1`, preflight).Scan(&count)
			if policy == protocol.VerificationIndependent {
				if count != 2 || status != "independent_confirmation" {
					t.Fatal("independent preflight completed without a second host")
				}
				var excluded string
				if err = s.DB.QueryRow(ctx, `SELECT excluded_group FROM runner_jobs WHERE preflight_id=$1 AND id<>$2`, preflight, job.ID).Scan(&excluded); err != nil {
					t.Fatal(err)
				}
				if excluded != "group1" {
					t.Fatal("independent preflight failed to exclude the first host group")
				}
				return
			}
			if count != 1 || status != "pass" {
				t.Fatal("platform preflight incorrectly required second host")
			}
			request := func(key string) *http.Request {
				r := httptest.NewRequest("POST", "/v1/challenge-versions/"+version+"/lock", strings.NewReader("{}"))
				r.SetPathValue("id", version)
				r.Header.Set("Idempotency-Key", key)
				return r
			}
			if err = s.lockChallenge(httptest.NewRecorder(), request("unsigned-machine-lock"), u); err == nil {
				t.Fatal("unsigned conformance permitted lock")
			}
			if err = s.signReceipt(ctx, machine); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			if err = s.lockChallenge(w, request("signed-platform-lock"), u); err != nil {
				t.Fatal(err)
			}
			var response map[string]any
			json.Unmarshal(w.Body.Bytes(), &response)
			lockDigest := response["lockDigest"].(string)
			if err = s.signReceipt(ctx, lockDigest); err != nil {
				t.Fatal(err)
			}
			if err = s.publishChallenge(httptest.NewRecorder(), request("publish-platform-version"), u); err != nil {
				t.Fatal(err)
			}
			var policy string
			if err = s.DB.QueryRow(ctx, `SELECT document->>'verificationPolicy' FROM locks WHERE digest=$1`, lockDigest).Scan(&policy); err != nil {
				t.Fatal(err)
			}
			if policy != "platform" {
				t.Fatal("new lock failed to freeze policy")
			}
			if _, err = s.DB.Exec(ctx, `UPDATE challenge_versions SET manifest=jsonb_set(manifest,'{verificationPolicy}','"independent"') WHERE id=$1`, version); err == nil {
				t.Fatal("published policy was mutable")
			}
		})
	}
}
