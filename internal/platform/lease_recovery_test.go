package platform

import (
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/internal/storage"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

func TestLeaseRecoveryPreservesVerificationPolicy(t *testing.T) {
	for _, test := range []struct {
		name, policy, wantStatus string
		exclusions, wantExcluded []string
		attempts                 int
	}{
		{"platform retry", protocol.VerificationPlatform, "queued", []string{"explicit-host"}, []string{"explicit-host"}, 1},
		{"platform existing self exclusion", protocol.VerificationPlatform, "queued", []string{"r1"}, []string{"r1"}, 1},
		{"platform attempt limit", protocol.VerificationPlatform, "attention_required", []string{"explicit-host"}, []string{"explicit-host"}, 8},
		{"independent retry", protocol.VerificationIndependent, "queued", []string{"explicit-host"}, []string{"explicit-host", "r1"}, 1},
		{"legacy retry", "", "queued", []string{"explicit-host"}, []string{"explicit-host", "r1"}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := testDB(t)
			_, version := seed(t, s)
			ctx := context.Background()
			job := protocol.RunnerJob{ID: ID(), VerificationPolicy: test.policy, ExcludedHostIDs: test.exclusions}
			if _, err := s.DB.Exec(ctx, `INSERT INTO runner_jobs(id,purpose,version_id,payload,status,host_id,result_token_hash,lease_expires_at,attempts) VALUES($1,'submission',$2,$3,'running','r1','old-token',now()-interval '1 second',$4)`, job.ID, version, raw(job), test.attempts); err != nil {
				t.Fatal(err)
			}
			if err := s.recoverRunnerLeases(ctx); err != nil {
				t.Fatal(err)
			}
			var status string
			var fence, attempts int
			var payload []byte
			var host, token *string
			var deadline *time.Time
			if err := s.DB.QueryRow(ctx, `SELECT status,fence,attempts,payload,host_id,result_token_hash,lease_expires_at FROM runner_jobs WHERE id=$1`, job.ID).Scan(&status, &fence, &attempts, &payload, &host, &token, &deadline); err != nil {
				t.Fatal(err)
			}
			var recovered protocol.RunnerJob
			if err := json.Unmarshal(payload, &recovered); err != nil {
				t.Fatal(err)
			}
			if status != test.wantStatus || fence != 2 || attempts != test.attempts || host != nil || token != nil || deadline != nil || !reflect.DeepEqual(recovered.ExcludedHostIDs, test.wantExcluded) {
				t.Fatalf("incorrect recovery: status=%s fence=%d attempts=%d exclusions=%v", status, fence, attempts, recovered.ExcludedHostIDs)
			}
		})
	}
}

func TestPlatformLeaseCanReclaimSameHostButRejectsStaleReceipt(t *testing.T) {
	s := testDB(t)
	u, version := seed(t, s, protocol.VerificationPlatform)
	ctx := context.Background()
	key, public := testKey(t)
	s.ReceiptSigner = key
	s.Config.ReceiptKeyID = "test-platform"
	// Presigning is local; this test never requests object bytes or cloud APIs.
	t.Setenv("AWS_ACCESS_KEY_ID", "test-only-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-only-secret-key")
	t.Setenv("AWS_SESSION_TOKEN", "")
	var err error
	s.Store, err = storage.New(ctx, "test-only-bucket", "us-east-1", "https://objects.invalid")
	if err != nil {
		t.Fatal(err)
	}
	_, accepted, err := accept(t, s, u, readyIntent(t, s, u, version, 1250), "same-host-lease-recovery")
	if err != nil {
		t.Fatal(err)
	}
	submission := accepted["submissionId"].(string)
	if err := s.queueSubmission(ctx, submission); err != nil {
		t.Fatal(err)
	}
	var payload []byte
	if err := s.DB.QueryRow(ctx, `SELECT payload FROM runner_jobs WHERE submission_id=$1 AND purpose='submission'`, submission).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var old protocol.RunnerJob
	if err := json.Unmarshal(payload, &old); err != nil {
		t.Fatal(err)
	}
	old.ExpiresAt = time.Now().UTC().Add(-time.Second)
	old.RequiredHostGroup = "group1"
	old.ExcludedHostIDs = []string{"explicit-host"}
	for _, ref := range []protocol.ObjectRef{old.ValidatorDisk, old.SubmissionDisk, old.SuiteDisk, old.ChallengeDisk} {
		if _, err := s.DB.Exec(ctx, `INSERT INTO artifacts(digest,blob_digest,size,media_type,owner_id) VALUES($1,$1,1,'application/octet-stream',$2) ON CONFLICT DO NOTHING`, ref.Digest, u.ID); err != nil {
			t.Fatal(err)
		}
	}
	oldToken := "expired-result-capability"
	if _, err := s.DB.Exec(ctx, `UPDATE runner_jobs SET payload=$2,status='running',host_id='r1',result_token_hash=$3,lease_expires_at=$4,attempts=1 WHERE id=$1`, old.ID, raw(old), hash(oldToken), old.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `UPDATE runner_hosts SET enabled=false WHERE id='r2'`); err != nil {
		t.Fatal(err)
	}
	if err := s.recoverRunnerLeases(ctx); err != nil {
		t.Fatal(err)
	}
	host := runnerIdentity{ID: "r1", Group: "group1", PublicKey: public, ExecutionProfile: "profile", Purposes: []string{"submission", "confirmation"}}
	w := httptest.NewRecorder()
	if err := s.claimRunnerJob(w, httptest.NewRequest("POST", "/internal/v1/runner/jobs/claim", strings.NewReader("{}")), host); err != nil {
		t.Fatal(err)
	}
	var claim struct {
		Job         *protocol.Envelope `json:"job"`
		ResultToken string             `json:"resultToken"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &claim); err != nil || claim.Job == nil {
		t.Fatalf("sole host could not reclaim recovered job: %v %s", err, w.Body.String())
	}
	data, err := protocol.Verify(*claim.Job, map[string]crypto.PublicKey{"test-platform": key.Public()})
	if err != nil {
		t.Fatal(err)
	}
	var fresh protocol.RunnerJob
	if err := json.Unmarshal(data, &fresh); err != nil {
		t.Fatal(err)
	}
	if fresh.ID != old.ID || fresh.FencingToken != 2 || !fresh.ExpiresAt.After(time.Now()) || claim.ResultToken == oldToken || !reflect.DeepEqual(fresh.ExcludedHostIDs, old.ExcludedHostIDs) {
		t.Fatal("reclaim did not produce a fresh fenced lease preserving exclusions")
	}
	send := func(job protocol.RunnerJob, token string) error {
		digest, _ := protocol.Digest(job)
		run := protocol.RunReceipt{APIVersion: protocol.APIVersion, Kind: "ValidationRunReceipt", ID: ID(), CreatedAt: time.Now().UTC(), Producer: host.ID, JobID: job.ID, JobDigest: digest, AcceptanceReceiptDigest: job.AcceptanceReceiptDigest, ChallengeLockDigest: job.ChallengeLockDigest, ArtifactDigest: job.ArtifactDigest, SuiteDigest: job.SuiteDigest, ExecutionProfileDigest: job.ExecutionProfileDigest, RunnerEpoch: job.RunnerEpoch, FencingToken: job.FencingToken, HostID: host.ID, HostGroup: host.Group, Official: true, CleanupAttested: true, DeploymentMode: job.DeploymentMode, OfficialAcceptance: job.OfficialAcceptance, VerificationPolicy: job.VerificationPolicy, Outcome: "valid", ScoreTicks: "30", Gates: map[string]bool{}}
		envelope, err := protocol.Sign(host.ID, key, run)
		if err != nil {
			return err
		}
		r := httptest.NewRequest("POST", "/internal/v1/runner/jobs/"+job.ID+"/result", strings.NewReader(string(raw(map[string]any{"envelope": envelope, "resultToken": token}))))
		r.SetPathValue("id", job.ID)
		return s.runnerResult(httptest.NewRecorder(), r, host)
	}
	for _, test := range []struct{ token, code string }{{oldToken, "result_capability_invalid"}, {claim.ResultToken, "stale_runner_lease"}} {
		var api *apiError
		if err := send(old, test.token); !errors.As(err, &api) || api.Code != test.code {
			t.Fatalf("old signed receipt not rejected at %s: %v", test.code, err)
		}
	}
	var results, reserved int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM runner_results WHERE job_id=$1`, old.ID).Scan(&results); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow(ctx, `SELECT reserved_units FROM capacity WHERE id=1`).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if results != 0 || reserved != 2 {
		t.Fatal("stale receipt changed competitive results or released capacity")
	}
	if err := send(fresh, claim.ResultToken); err != nil {
		t.Fatalf("fresh same-host receipt rejected: %v", err)
	}
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM runner_results WHERE job_id=$1`, old.ID).Scan(&results); err != nil || results != 1 {
		t.Fatalf("fresh receipt was not stored exactly once: %d %v", results, err)
	}
}
