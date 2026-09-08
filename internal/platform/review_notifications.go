package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
)

func (c Config) reviewEmailReady() bool {
	if c.SendGridKey == "" {
		return false
	}
	for _, v := range []string{c.ReviewEmailFrom, c.ReviewEmailTo} {
		a, e := mail.ParseAddress(v)
		if e != nil || a.Address != v || strings.ContainsAny(v, "\r\n") {
			return false
		}
	}
	origin, e := url.Parse(c.PublicOrigin)
	return e == nil && origin.Scheme == "https" && origin.Host != "" && origin.User == nil && origin.RawQuery == "" && origin.Fragment == ""
}
func (s *Server) runReviewNotifications(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		if e := s.sendReviewNotification(ctx); e != nil && !errors.Is(e, pgx.ErrNoRows) && !errors.Is(e, context.Canceled) {
			slog.Error("review email worker", "error", e)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func emailText(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > max {
		r = r[:max]
	}
	return string(r)
}
func (s *Server) notificationContent(ctx context.Context, id string) (string, string, bool, error) {
	var candidate, version *string
	var test bool
	if e := s.DB.QueryRow(ctx, `SELECT candidate_id,version_id,is_test FROM review_notifications WHERE id=$1`, id).Scan(&candidate, &version, &test); e != nil {
		return "", "", false, e
	}
	if test {
		return "Science Ladder email test", "Science Ladder review notifications are connected.\n\nNew reviews will include the challenge title, why your attention is needed, and a link to the review queue.\n\n" + s.Config.PublicOrigin + "/review", true, nil
	}
	var title, reason string
	var active bool
	link := s.Config.PublicOrigin + "/review"
	if version != nil {
		e := s.DB.QueryRow(ctx, `SELECT COALESCE(manifest->>'title','Challenge'),review_status='human_review_required' AND status NOT IN('withdrawn','closed','superseded','rejected','compromised'),COALESCE((SELECT report->'review'->>'summary' FROM review_runs WHERE version_id=v.id AND kind='scientific-legibility' ORDER BY created_at DESC LIMIT 1),'Scientific review requires your decision.') FROM challenge_versions v WHERE id=$1`, *version).Scan(&title, &active, &reason)
		if e != nil {
			return "", "", false, e
		}
		link += "?version=" + url.QueryEscape(*version) + "#decision-form"
	} else {
		var findings []byte
		e := s.DB.QueryRow(ctx, `SELECT COALESCE(document->'manifest'->>'title',document->>'id','Challenge proposal'),status='human_review_required' AND NOT EXISTS(SELECT 1 FROM challenges c WHERE c.candidate_id=ca.id),findings FROM candidates ca WHERE id=$1`, *candidate).Scan(&title, &active, &findings)
		if e != nil {
			return "", "", false, e
		}
		var f []Finding
		_ = json.Unmarshal(findings, &f)
		for _, item := range f {
			if item.Severity == "review" {
				reason += item.Message + " "
			}
		}
		if reason == "" {
			reason = "The proposal's source evidence needs human review."
		}
		link += "#candidate-" + url.QueryEscape(*candidate)
	}
	title = emailText(title, 180)
	reason = emailText(reason, 1600)
	return "Review needed: " + title, "A Science Ladder challenge needs your review.\n\n" + title + "\n\n" + reason + "\n\nReview it here (editor sign-in required):\n" + link + "\n\nThis email requests a review; it does not approve or publish the challenge.", active, nil
}
func (s *Server) sendReviewNotification(ctx context.Context) error {
	// Do not consume attempts or discard queued reviews while configuration is absent.
	if !s.Config.reviewEmailReady() {
		return nil
	}
	if _, e := s.DB.Exec(ctx, `UPDATE review_notifications SET status='uncertain',last_error='Worker stopped during a send; check SendGrid activity before retrying.',lease_token=NULL,lease_expires_at=NULL WHERE status='sending' AND lease_expires_at<now()`); e != nil {
		return e
	}
	var id, lease string
	var attempts int
	e := s.DB.QueryRow(ctx, `WITH picked AS(SELECT id FROM review_notifications WHERE status='pending' AND available_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE review_notifications n SET status='sending',attempts=attempts+1,lease_token=gen_random_uuid(),lease_expires_at=now()+interval '2 minutes' FROM picked WHERE n.id=picked.id RETURNING n.id,n.lease_token,n.attempts`).Scan(&id, &lease, &attempts)
	if e != nil {
		return e
	}
	finish := func(state, message, provider string, delay time.Duration) error {
		// Persist delivery evidence even when shutdown cancels the outer worker context.
		save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, e := s.DB.Exec(save, `UPDATE review_notifications SET status=$3,last_error=NULLIF($4,''),provider_message_id=NULLIF($5,''),accepted_at=CASE WHEN $3='accepted' THEN now() ELSE accepted_at END,available_at=$6,lease_token=NULL,lease_expires_at=NULL WHERE id=$1 AND lease_token=$2`, id, lease, state, message, provider, time.Now().Add(delay))
		return e
	}
	subject, body, active, e := s.notificationContent(ctx, id)
	if e != nil {
		return finish("pending", "Could not load the review; retrying.", "", time.Minute)
	}
	if !active {
		return finish("cancelled", "Review was resolved or adopted before email was sent.", "", 0)
	}
	payload := map[string]any{
		"personalizations": []any{map[string]any{"to": []any{map[string]string{"email": s.Config.ReviewEmailTo}}, "custom_args": map[string]string{"science_ladder_notification_id": id}}},
		"from":             map[string]string{"email": s.Config.ReviewEmailFrom, "name": "Science Ladder"}, "subject": subject, "content": []any{map[string]string{"type": "text/plain", "value": body}},
		"tracking_settings": map[string]any{"click_tracking": map[string]bool{"enable": false, "enable_text": false}, "open_tracking": map[string]bool{"enable": false}},
	}
	data, _ := json.Marshal(payload)
	sendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(sendCtx, "POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewReader(data))
	if e != nil {
		return finish("failed", "Could not construct email request.", "", 0)
	}
	req.Header.Set("Authorization", "Bearer "+s.Config.SendGridKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if s.HTTP != nil {
		client.Transport = s.HTTP.Transport
	}
	response, e := client.Do(req)
	if e != nil {
		return finish("uncertain", "SendGrid response was not received; check its email activity before retrying.", "", 0)
	}
	// Never persist provider response bodies, which may echo addresses or credentials.
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	response.Body.Close()
	if response.StatusCode == http.StatusAccepted {
		return finish("accepted", "", emailText(response.Header.Get("X-Message-Id"), 200), 0)
	}
	message := fmt.Sprintf("SendGrid returned HTTP %d.", response.StatusCode)
	if (response.StatusCode == 429 || response.StatusCode >= 500) && attempts < 8 {
		return finish("pending", message, "", time.Duration(min(1<<attempts, 360))*time.Minute)
	}
	return finish("failed", message, "", 0)
}
func (s *Server) notificationStatus(ctx context.Context) (map[string]any, error) {
	counts, e := queryObjects(ctx, s.DB, `SELECT jsonb_build_object('status',status,'count',count(*)) FROM review_notifications GROUP BY status`)
	if e != nil {
		return nil, e
	}
	recent, e := queryObjects(ctx, s.DB, `SELECT jsonb_build_object('id',id,'versionId',version_id,'candidateId',candidate_id,'isTest',is_test,'status',status,'attempts',attempts,'createdAt',created_at,'acceptedAt',accepted_at,'lastError',last_error) FROM review_notifications ORDER BY created_at DESC LIMIT 20`)
	return map[string]any{"configured": s.Config.reviewEmailReady(), "counts": counts, "recent": recent}, e
}
func (s *Server) testReviewEmail(w http.ResponseWriter, r *http.Request, u *User) error {
	if !editor(u) {
		return fail(403, "editor_required", "Editor access required")
	}
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var in struct{}
		if e := readJSON(r, &in); e != nil {
			return 0, nil, e
		}
		if !s.Config.reviewEmailReady() {
			return 0, nil, fail(503, "review_email_not_configured", "Configure SendGrid and the verified sender and recipient first")
		}
		if _, e := tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(6842077296)`); e != nil {
			return 0, nil, e
		}
		var recent bool
		if e := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM review_notifications WHERE is_test AND created_at>now()-interval '1 hour')`).Scan(&recent); e != nil {
			return 0, nil, e
		}
		if recent {
			return 0, nil, fail(429, "test_email_limit", "A test email was already queued in the past hour")
		}
		id := ID()
		_, e := tx.Exec(r.Context(), `INSERT INTO review_notifications(id,is_test) VALUES($1,true)`, id)
		return 202, map[string]any{"id": id, "status": "pending"}, e
	})
}
func (s *Server) retryReviewEmail(w http.ResponseWriter, r *http.Request, u *User) error {
	if !editor(u) {
		return fail(403, "editor_required", "Editor access required")
	}
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var in struct {
			ConfirmPossibleDuplicate bool `json:"confirmPossibleDuplicate"`
		}
		if e := readJSON(r, &in); e != nil {
			return 0, nil, e
		}
		var state string
		if e := tx.QueryRow(r.Context(), `SELECT status FROM review_notifications WHERE id=$1 FOR UPDATE`, r.PathValue("id")).Scan(&state); e != nil {
			return 0, nil, e
		}
		if state != "failed" && state != "uncertain" {
			return 0, nil, fail(409, "email_not_retryable", "Only failed or uncertain messages can be retried")
		}
		if state == "uncertain" && !in.ConfirmPossibleDuplicate {
			return 0, nil, fail(409, "email_may_duplicate", "Check SendGrid activity and acknowledge a possible duplicate before retrying")
		}
		_, e := tx.Exec(r.Context(), `UPDATE review_notifications SET status='pending',attempts=0,available_at=now(),last_error=NULL WHERE id=$1`, r.PathValue("id"))
		return 200, map[string]string{"status": "pending"}, e
	})
}
