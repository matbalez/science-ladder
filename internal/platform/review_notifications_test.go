package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
)

func emailServer(t *testing.T) *Server {
	s := testDB(t)
	s.Config.PublicOrigin = "https://science.example"
	s.Config.SendGridKey = "test-secret"
	s.Config.ReviewEmailFrom = "sender@example.org"
	s.Config.ReviewEmailTo = "editor@example.org"
	return s
}
func emailState(t *testing.T, s *Server, id string) string {
	t.Helper()
	var state string
	if e := s.DB.QueryRow(context.Background(), `SELECT status FROM review_notifications WHERE id=$1`, id).Scan(&state); e != nil {
		t.Fatal(e)
	}
	return state
}
func queueEmail(t *testing.T, s *Server) string {
	t.Helper()
	id := ID()
	if _, e := s.DB.Exec(context.Background(), `INSERT INTO review_notifications(id,is_test) VALUES($1,true)`, id); e != nil {
		t.Fatal(e)
	}
	return id
}
func TestReviewEmailTransitions(t *testing.T) {
	s := emailServer(t)
	u, v := seed(t, s)
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := s.DB.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	count := func(want int) {
		t.Helper()
		var n int
		if e := s.DB.QueryRow(ctx, `SELECT count(*) FROM review_notifications`).Scan(&n); e != nil || n != want {
			t.Fatalf("count %d want %d: %v", n, want, e)
		}
	}
	exec(`UPDATE challenge_versions SET review_status='human_review_required' WHERE id=$1`, v)
	count(1)
	exec(`UPDATE challenge_versions SET review_status='human_review_required' WHERE id=$1`, v)
	count(1)
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	_, e = tx.Exec(ctx, `INSERT INTO candidates(id,owner_id,digest,document,status) VALUES($1,$2,'rollback','{}','human_review_required')`, ID(), u.ID)
	if e != nil {
		t.Fatal(e)
	}
	tx.Rollback(ctx)
	count(1)
	exec(`UPDATE challenge_versions SET review_status='automated_pass' WHERE id=$1`, v)
	exec(`UPDATE challenge_versions SET review_status='human_review_required' WHERE id=$1`, v)
	count(2)
	exec(`INSERT INTO candidates(id,owner_id,digest,document,status) VALUES($1,$2,'review','{}','human_review_required')`, ID(), u.ID)
	count(3)
}
func TestReviewEmailSendAndConcurrency(t *testing.T) {
	s := emailServer(t)
	id := queueEmail(t, s)
	var calls atomic.Int32
	s.HTTP = &http.Client{Transport: reviewTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.String() != "https://api.sendgrid.com/v3/mail/send" || r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("invalid endpoint/auth")
		}
		var p struct {
			Personalizations []struct{ To []struct{ Email string } }
			Content          []struct{ Type, Value string }
			Subject          string
		}
		if e := json.NewDecoder(r.Body).Decode(&p); e != nil {
			t.Error(e)
		}
		if len(p.Personalizations) != 1 || p.Personalizations[0].To[0].Email != "editor@example.org" || !strings.Contains(p.Content[0].Value, "https://science.example/review") || p.Subject != "Science Ladder email test" {
			t.Error("incorrect message")
		}
		return &http.Response{StatusCode: 202, Header: http.Header{"X-Message-Id": []string{"test-message"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := s.sendReviewNotification(context.Background()); e != nil && !errors.Is(e, pgx.ErrNoRows) {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 || emailState(t, s, id) != "accepted" {
		t.Fatal("duplicate or missing send")
	}
	var provider string
	if e := s.DB.QueryRow(context.Background(), `SELECT provider_message_id FROM review_notifications WHERE id=$1`, id).Scan(&provider); e != nil || provider != "test-message" {
		t.Fatal(provider, e)
	}
}
func TestReviewEmailFailureHandling(t *testing.T) {
	for _, tc := range []struct {
		name  string
		code  int
		state string
	}{{"throttled", 429, "pending"}, {"server", 503, "pending"}, {"forbidden", 403, "failed"}, {"redirect", 302, "failed"}, {"lost response", 0, "uncertain"}} {
		t.Run(tc.name, func(t *testing.T) {
			s := emailServer(t)
			id := queueEmail(t, s)
			s.HTTP = &http.Client{Transport: reviewTransport(func(*http.Request) (*http.Response, error) {
				if tc.code == 0 {
					return nil, errors.New("secret-sensitive-network-error")
				}
				return &http.Response{StatusCode: tc.code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("secret-provider-body"))}, nil
			})}
			if e := s.sendReviewNotification(context.Background()); e != nil {
				t.Fatal(e)
			}
			if emailState(t, s, id) != tc.state {
				t.Fatal("wrong failure state")
			}
			var last string
			var delayed bool
			if e := s.DB.QueryRow(context.Background(), `SELECT last_error,available_at>now() FROM review_notifications WHERE id=$1`, id).Scan(&last, &delayed); e != nil {
				t.Fatal(e)
			}
			if strings.Contains(last, "secret") {
				t.Fatal("secret echoed")
			}
			if tc.state == "pending" && !delayed {
				t.Fatal("missing backoff")
			}
		})
	}
}
func TestReviewEmailUnconfiguredAndResolved(t *testing.T) {
	s := emailServer(t)
	_, v := seed(t, s)
	ctx := context.Background()
	if _, e := s.DB.Exec(ctx, `UPDATE challenge_versions SET review_status='human_review_required' WHERE id=$1`, v); e != nil {
		t.Fatal(e)
	}
	var id string
	s.DB.QueryRow(ctx, `SELECT id FROM review_notifications`).Scan(&id)
	s.Config.SendGridKey = ""
	if e := s.sendReviewNotification(ctx); e != nil {
		t.Fatal(e)
	}
	var attempts int
	s.DB.QueryRow(ctx, `SELECT attempts FROM review_notifications WHERE id=$1`, id).Scan(&attempts)
	if attempts != 0 || emailState(t, s, id) != "pending" {
		t.Fatal("unconfigured consumed attempt")
	}
	s.Config.SendGridKey = "test-secret"
	s.DB.Exec(ctx, `UPDATE challenge_versions SET review_status='automated_pass' WHERE id=$1`, v)
	if e := s.sendReviewNotification(ctx); e != nil {
		t.Fatal(e)
	}
	if emailState(t, s, id) != "cancelled" {
		t.Fatal("resolved review sent")
	}
	id = queueEmail(t, s)
	s.DB.Exec(ctx, `UPDATE review_notifications SET status='sending',lease_expires_at=now()-interval '1 minute' WHERE id=$1`, id)
	if e := s.sendReviewNotification(ctx); !errors.Is(e, pgx.ErrNoRows) {
		t.Fatal(e)
	}
	if emailState(t, s, id) != "uncertain" {
		t.Fatal("expired send automatically resent")
	}
}
func TestReviewEmailConfiguration(t *testing.T) {
	good := Config{SendGridKey: "test", ReviewEmailFrom: "sender@example.org", ReviewEmailTo: "editor@example.org", PublicOrigin: "https://science.example"}
	if !good.reviewEmailReady() {
		t.Fatal("valid config refused")
	}
	for _, bad := range []string{"", "Name <sender@example.org>", "sender@example.org\nBcc: other@example.org"} {
		c := good
		c.ReviewEmailFrom = bad
		if c.reviewEmailReady() {
			t.Fatal("bad sender accepted")
		}
	}
	if emailText("Hello\r\nInjected", 100) != "Hello Injected" {
		t.Fatal("header sanitation")
	}
}

func TestReviewEmailEditorControls(t *testing.T) {
	s := emailServer(t)
	u, _ := seed(t, s)
	if _, e := admissionRequest(s, u, map[string]any{}, "forbidden-email", s.testReviewEmail); e == nil {
		t.Fatal("member sent test mail")
	}
	u.Role = "editor"
	out, e := admissionRequest(s, u, map[string]any{}, "test-email-once", s.testReviewEmail)
	if e != nil {
		t.Fatal(e)
	}
	again, e := admissionRequest(s, u, map[string]any{}, "test-email-once", s.testReviewEmail)
	if e != nil || out["id"] != again["id"] {
		t.Fatal("idempotency", e)
	}
	if _, e := admissionRequest(s, u, map[string]any{}, "test-email-twice", s.testReviewEmail); e == nil {
		t.Fatal("hourly limit bypass")
	}
	id := out["id"].(string)
	if _, e := s.DB.Exec(context.Background(), `UPDATE review_notifications SET status='uncertain' WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	handler := func(w http.ResponseWriter, r *http.Request, u *User) error {
		r.SetPathValue("id", id)
		return s.retryReviewEmail(w, r, u)
	}
	if _, e := admissionRequest(s, u, map[string]any{}, "retry-no-confirm", handler); e == nil {
		t.Fatal("uncertain retried without acknowledgement")
	}
	if _, e := admissionRequest(s, u, map[string]any{"confirmPossibleDuplicate": true}, "retry-confirmed", handler); e != nil {
		t.Fatal(e)
	}
	if emailState(t, s, id) != "pending" {
		t.Fatal("not queued")
	}
}
