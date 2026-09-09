package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInvitationUsernameLookup(t *testing.T) {
	for _, tc := range []struct {
		name, input, body, code string
		status                  int
	}{
		{"normalized", " @OcToCaT ", `{"id":43,"login":"octocat","type":"User"}`, "", 200},
		{"missing", "missing", `{}`, "github_user_not_found", 404},
		{"organization", "octocat", `{"id":43,"login":"octocat","type":"Organization"}`, "github_personal_account_required", 200},
		{"bot", "octocat", `{"id":43,"login":"octocat","type":"Bot"}`, "github_personal_account_required", 200},
		{"mismatch", "octocat", `{"id":43,"login":"someone-else","type":"User"}`, "github_identity_invalid", 200},
		{"no id", "octocat", `{"login":"octocat","type":"User"}`, "github_identity_invalid", 200},
		{"rate limit", "octocat", `{}`, "github_lookup_unavailable", 403},
		{"redirect", "octocat", `{}`, "github_lookup_unavailable", 301},
		{"bad response", "octocat", `not json`, "github_identity_invalid", 200},
		{"URL", "https://github.com/octocat", `{}`, "github_username_invalid", 0},
		{"path", "../octocat", `{}`, "github_username_invalid", 0},
		{"blank", " ", `{}`, "github_username_invalid", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := &Server{HTTP: &http.Client{Transport: reviewTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host != "api.github.com" || r.Method != "GET" || r.Header.Get("Authorization") != "" {
					t.Fatal("unexpected credential or request target")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: http.Header{}}, nil
			})}}
			identity, err := s.invitationIdentity(context.Background(), tc.input)
			if tc.code == "" {
				if err != nil || identity.ID != 43 || identity.Login != "octocat" {
					t.Fatal(identity, err)
				}
			} else if e, ok := err.(*apiError); !ok || e.Code != tc.code {
				t.Fatalf("want %s, got %v", tc.code, err)
			}
			if tc.status == 0 && calls != 0 {
				t.Fatal("invalid input triggered lookup")
			}
		})
	}
}

func TestUsernameInvitationPersistsResolvedIDAndReplaysWithoutLookup(t *testing.T) {
	s := testDB(t)
	operator, _ := seed(t, s)
	operator.Role = "operator"
	ctx := context.Background()
	recipient := ID()
	if _, err := s.DB.Exec(ctx, `INSERT INTO users(id,github_id,login,invited,validation_quota) VALUES($1,43,'old-login',false,0)`, recipient); err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.HTTP = &http.Client{Transport: reviewTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/users/OCTOCAT" {
			t.Fatal(r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"id":43,"login":"octocat","type":"User"}`))}, nil
	})}
	send := func(u *User, body, key string) (map[string]any, error) {
		r := httptest.NewRequest("POST", "/v1/invites", strings.NewReader(body))
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		err := s.invite(w, r, u)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return out, err
	}
	input := `{"githubUsername":" @OCTOCAT ","role":"member"}`
	first, err := send(operator, input, "invite-username-once")
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := send(operator, input, "invite-username-once")
	if err != nil || calls != 1 || repeat["githubId"] != first["githubId"] || first["githubUsername"] != "octocat" {
		t.Fatal(first, repeat, err, calls)
	}
	var id int64
	var invited bool
	if err = s.DB.QueryRow(ctx, `SELECT github_id FROM invitations`).Scan(&id); err != nil || id != 43 {
		t.Fatal(id, err)
	}
	if err = s.DB.QueryRow(ctx, `SELECT invited FROM users WHERE id=$1`, recipient).Scan(&invited); err != nil || !invited {
		t.Fatal("existing account did not receive access", err)
	}
	operator.Role = "member"
	if _, err = send(operator, input, "not-an-operator"); err == nil || calls != 1 {
		t.Fatal("nonoperator could invite or lookup")
	}
	operator.Role = "operator"
	if _, err = send(operator, `{"githubId":44,"githubUsername":"octocat","role":"member"}`, "mixed-identities"); err == nil || calls != 1 {
		t.Fatal("ambiguous identity was accepted")
	}
}
