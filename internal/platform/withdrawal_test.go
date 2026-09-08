package platform

import (
	"context"
	"encoding/json"
	"testing"
)

func TestWithdrawalHidesChallengeClosesAdmissionAndPreservesLock(t *testing.T) {
	s := testDB(t)
	owner, version := seed(t, s, "platform")
	ctx := context.Background()
	// A public predecessor must not reappear after removing the latest version.
	if _, err := s.DB.Exec(ctx, `INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,status,intake_status,deadline,created_at) SELECT $2,challenge_id,repository,repository_id,source_commit,source_digest,manifest,'superseded','closed',deadline,created_at-interval '1 day' FROM challenge_versions WHERE id=$1`, version, ID()); err != nil {
		t.Fatal(err)
	}
	var before, after []byte
	snapshot := `SELECT jsonb_build_object('manifest',manifest,'lock',lock_digest,'source',source_commit,'document',(SELECT document FROM locks WHERE version_id=v.id)) FROM challenge_versions v WHERE id=$1`
	if err := s.DB.QueryRow(ctx, snapshot, version).Scan(&before); err != nil {
		t.Fatal(err)
	}
	token := researcherBrowser(t, s, "operator", "browser", 101)
	body := raw(map[string]any{"versionId": version, "action": "withdraw", "reason": "The reference does not represent the scientific frontier."})
	w := researcherRequest(s, "", "POST", "/v1/editor/decisions", "withdraw-test-unauthorized", body)
	if w.Code != 401 && w.Code != 403 {
		t.Fatalf("unauthenticated withdrawal: %d", w.Code)
	}
	for i := 0; i < 2; i++ {
		w = researcherRequest(s, token, "POST", "/v1/editor/decisions", "withdraw-test-replay", body)
		if w.Code != 201 {
			t.Fatalf("withdraw failed %d %s", w.Code, w.Body.String())
		}
	}
	w = researcherRequest(s, "", "GET", "/v1/challenges", "", nil)
	var list struct {
		Challenges []any `json:"challenges"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &list) != nil || len(list.Challenges) != 0 {
		t.Fatal("withdrawn challenge/predecessor still listed", w.Body.String())
	}
	for _, auth := range []string{"", token} {
		w = researcherRequest(s, auth, "GET", "/v1/challenges/test-challenge", "", nil)
		if w.Code != 410 {
			t.Fatal("withdrawn page still accessible", w.Code, w.Body.String())
		}
	}
	var intake string
	var decisions int
	if err := s.DB.QueryRow(ctx, `SELECT intake_status,(SELECT count(*) FROM editorial_decisions WHERE version_id=$1 AND action='withdraw') FROM challenge_versions WHERE id=$1`, version).Scan(&intake, &decisions); err != nil {
		t.Fatal(err)
	}
	if intake != "closed" || decisions != 1 {
		t.Fatal("withdrawal not closed/idempotent")
	}
	if err := s.DB.QueryRow(ctx, snapshot, version).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("immutable source or lock changed")
	}
	intent := readyIntent(t, s, owner, version, 999)
	if _, _, err := accept(t, s, owner, intent, "withdraw-accept-test"); err == nil {
		t.Fatal("removed challenge accepted a submission")
	}
}
