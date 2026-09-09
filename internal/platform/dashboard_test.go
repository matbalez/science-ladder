package platform

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestDashboardParticipationIsOwnerScopedAcrossVersions(t *testing.T) {
	s := testDB(t)
	u, v := seed(t, s)
	ctx := context.Background()
	s.DB.Exec(ctx, `UPDATE users SET validation_quota=0 WHERE id=$1`, u.ID)
	intent := readyIntent(t, s, u, v, 1)
	if _, _, err := accept(t, s, u, intent, "dashboard-accept"); err != nil {
		t.Fatal(err)
	}
	// An unaccepted attempt counts as pending, not as a submission.
	readyIntent(t, s, u, v, 2)
	next := ID()
	if _, err := s.DB.Exec(ctx, `INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,status,intake_status,deadline,created_at) SELECT $2,challenge_id,repository,repository_id,source_commit,source_digest,manifest,'draft','closed',deadline,now()+interval '1 minute' FROM challenge_versions WHERE id=$1`, v, next); err != nil {
		t.Fatal(err)
	}
	readyIntent(t, s, u, next, 3)
	read := func(user *User) map[string]json.RawMessage {
		t.Helper()
		w := httptest.NewRecorder()
		if err := s.dashboard(w, httptest.NewRequest("GET", "/v1/dashboard", nil), user); err != nil {
			t.Fatal(err)
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	result := read(u)
	var participation []struct {
		ID                            string
		Open                          bool
		SubmissionCount, PendingCount int
	}
	if err := json.Unmarshal(result["participation"], &participation); err != nil {
		t.Fatal(err)
	}
	if len(participation) != 1 || participation[0].SubmissionCount != 1 || participation[0].PendingCount != 2 || !participation[0].Open {
		t.Fatalf("incorrect version aggregation: %+v", participation)
	}
	if string(result["submissionCount"]) != "1" {
		t.Fatal(string(result["submissionCount"]))
	}
	var contexts []struct {
		VersionID      string `json:"versionId"`
		Title, Quantum string
	}
	if err := json.Unmarshal(result["submissionContexts"], &contexts); err != nil {
		t.Fatal(err)
	}
	if len(contexts) != 1 || contexts[0].VersionID != v || contexts[0].Quantum != "1" {
		t.Fatalf("incorrect exact-version context: %+v", contexts)
	}
	// A signed-in person with no work sees no other account's private activity.
	other := &User{ID: ID(), Invited: true}
	result = read(other)
	for _, key := range []string{"participation", "submissions", "submissionContexts", "intents", "challenges"} {
		if string(result[key]) != "[]" {
			t.Fatalf("%s leaked another owner's activity", key)
		}
	}
	if string(result["submissionCount"]) != "0" {
		t.Fatal("count leaked")
	}
	if _, err := s.DB.Exec(ctx, `UPDATE challenge_versions SET status='superseded',intake_status='closed' WHERE id=$1`, v); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `UPDATE challenge_versions SET status='published',intake_status='open' WHERE id=$1`, next); err != nil {
		t.Fatal(err)
	}
	result = read(u)
	json.Unmarshal(result["participation"], &participation)
	if !participation[0].Open {
		t.Fatal("open published challenge not counted")
	}
	if _, err := s.DB.Exec(ctx, `UPDATE challenge_versions SET deadline=now()-interval '1 minute' WHERE id=$1`, next); err != nil {
		t.Fatal(err)
	}
	result = read(u)
	json.Unmarshal(result["participation"], &participation)
	if participation[0].Open {
		t.Fatal("expired challenge counted as open")
	}
}
