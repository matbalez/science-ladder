package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (s *Server) authStart(w http.ResponseWriter, r *http.Request, u *User) error {
	if s.Config.GitHubClientID == "" || s.Config.GitHubClientSecret == "" {
		return fail(503, "github_not_configured", "GitHub sign-in is awaiting application configuration")
	}
	state, verifier := secret(), secret()
	_, err := s.DB.Exec(r.Context(), `INSERT INTO oauth_states(token_hash,verifier,expires_at) VALUES($1,$2,now()+interval '10 minutes')`, hash(state), verifier)
	if err != nil {
		return err
	}
	challenge := sha256.Sum256([]byte(verifier))
	q := url.Values{"client_id": {s.Config.GitHubClientID}, "redirect_uri": {s.Config.PublicOrigin + "/v1/auth/github/callback"}, "state": {state}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"}}
	http.SetCookie(w, &http.Cookie{Name: "sl_oauth", Value: state, Path: "/v1/auth/github", HttpOnly: true, Secure: strings.HasPrefix(s.Config.PublicOrigin, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 600})
	http.Redirect(w, r, "https://github.com/login/oauth/authorize?"+q.Encode(), http.StatusFound)
	return nil
}
func (s *Server) authCallback(w http.ResponseWriter, r *http.Request, u *User) error {
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie("sl_oauth")
	if err != nil || state == "" || !hmac.Equal([]byte(state), []byte(cookie.Value)) {
		return fail(400, "oauth_state_invalid", "GitHub login expired or was not started in this browser")
	}
	var verifier string
	err = s.DB.QueryRow(r.Context(), `DELETE FROM oauth_states WHERE token_hash=$1 AND expires_at>now() RETURNING verifier`, hash(state)).Scan(&verifier)
	if err != nil {
		return fail(400, "oauth_state_invalid", "GitHub login expired; start again")
	}
	form := url.Values{"client_id": {s.Config.GitHubClientID}, "client_secret": {s.Config.GitHubClientSecret}, "code": {r.URL.Query().Get("code")}, "redirect_uri": {s.Config.PublicOrigin + "/v1/auth/github/callback"}, "code_verifier": {verifier}}
	req, err := http.NewRequestWithContext(r.Context(), "POST", "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var token struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&token); err != nil || token.AccessToken == "" {
		return fail(401, "oauth_failed", "GitHub did not authorize this login")
	}
	var identity struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	if err = s.github(r.Context(), "GET", "/user", token.AccessToken, nil, &identity); err != nil {
		return err
	}
	if identity.ID <= 0 {
		return fail(401, "oauth_identity_invalid", "GitHub identity is missing")
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	role, invited, quota := "member", false, 0
	if identity.ID == s.Config.OperatorGitHubID {
		role, invited, quota = "operator", true, 1000
	} else {
		err = tx.QueryRow(r.Context(), `SELECT role,validation_quota FROM invitations WHERE github_id=$1`, identity.ID).Scan(&role, &quota)
		if err == nil {
			invited = true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	var userID string
	err = tx.QueryRow(r.Context(), `INSERT INTO users(id,github_id,login,avatar_url,role,invited,validation_quota) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(github_id) DO UPDATE SET login=excluded.login,avatar_url=excluded.avatar_url,invited=users.invited OR excluded.invited RETURNING id`, ID(), identity.ID, identity.Login, identity.AvatarURL, role, invited, quota).Scan(&userID)
	if err != nil {
		return err
	}
	session := secret()
	_, err = tx.Exec(r.Context(), `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '12 hours')`, hash(session), userID)
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: "sl_session", Value: session, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(s.Config.PublicOrigin, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 43200})
	http.SetCookie(w, &http.Cookie{Name: "sl_oauth", Path: "/v1/auth/github", MaxAge: -1, HttpOnly: true, Secure: strings.HasPrefix(s.Config.PublicOrigin, "https://")})
	http.Redirect(w, r, "/account", 303)
	return nil
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request, u *User) error {
	if c, e := r.Cookie("sl_session"); e == nil {
		if _, e = s.DB.Exec(r.Context(), `DELETE FROM sessions WHERE token_hash=$1`, hash(c.Value)); e != nil {
			return e
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "sl_session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: strings.HasPrefix(s.Config.PublicOrigin, "https://"), SameSite: http.SameSiteLaxMode})
	respond(w, 200, map[string]bool{"signedOut": true})
	return nil
}
func (s *Server) me(w http.ResponseWriter, r *http.Request, u *User) error {
	remaining := 0
	write, review := false, false
	if u != nil {
		remaining = u.Quota
		write = u.Invited
		review = u.Role == "editor" || u.Role == "operator"
	}
	var groups int
	if err := s.DB.QueryRow(r.Context(), `SELECT count(DISTINCT host_group) FROM runner_hosts WHERE enabled`).Scan(&groups); err != nil {
		return err
	}
	respond(w, 200, map[string]any{"user": u, "quotas": map[string]any{"remaining": remaining, "activeLimit": s.Config.ActiveLimit}, "capabilities": map[string]bool{"creation": write, "submission": write && groups >= 2, "review": review}, "configuration": map[string]bool{"githubAuth": s.Config.GitHubClientID != "", "scientificReview": s.Config.OpenAIKey != "", "officialRunner": groups >= 2}})
	return nil
}
func (s *Server) webhook(w http.ResponseWriter, r *http.Request) error {
	if s.Config.GitHubWebhookSecret == "" {
		return fail(503, "webhook_unavailable", "GitHub webhook verification is not configured")
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, []byte(s.Config.GitHubWebhookSecret))
	mac.Write(b)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(r.Header.Get("X-Hub-Signature-256"))) {
		return fail(401, "webhook_signature_invalid", "Invalid webhook signature")
	}
	delivery := r.Header.Get("X-Github-Delivery")
	if delivery == "" {
		return fail(400, "delivery_missing", "GitHub delivery ID required")
	}
	_, err = s.DB.Exec(r.Context(), `INSERT INTO webhook_deliveries(id,event) VALUES($1,$2) ON CONFLICT DO NOTHING`, delivery, r.Header.Get("X-Github-Event"))
	if err != nil {
		return err
	}
	respond(w, 202, map[string]bool{"received": true})
	return nil
}
func (s *Server) cliStart(w http.ResponseWriter, r *http.Request, u *User) error {
	id, device, code := ID(), secret(), strings.ToUpper(secret()[:8])
	expires := time.Now().UTC().Add(10 * time.Minute)
	_, err := s.DB.Exec(r.Context(), `INSERT INTO cli_sessions(id,secret_hash,user_code,expires_at) VALUES($1,$2,$3,$4)`, id, hash(device), code, expires)
	if err != nil {
		return err
	}
	respond(w, 201, map[string]any{"id": id, "deviceSecret": device, "userCode": code, "verificationUrl": s.Config.PublicOrigin + "/authorize?session=" + id, "expiresAt": expires})
	return nil
}
func (s *Server) cliApprove(w http.ResponseWriter, r *http.Request, u *User) error {
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var in struct {
			UserCode string `json:"userCode"`
		}
		if err := readJSON(r, &in); err != nil {
			return 0, nil, err
		}
		tag, err := tx.Exec(r.Context(), `UPDATE cli_sessions SET user_id=$2 WHERE id=$1 AND user_code=$3 AND expires_at>now() AND user_id IS NULL`, r.PathValue("id"), u.ID, in.UserCode)
		if err != nil {
			return 0, nil, err
		}
		if tag.RowsAffected() != 1 {
			return 0, nil, fail(400, "device_code_invalid", "Device code is invalid or expired")
		}
		return 200, map[string]bool{"approved": true}, nil
	})
}
func (s *Server) cliToken(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct {
		DeviceSecret string `json:"deviceSecret"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var userID *string
	var expected string
	err = tx.QueryRow(r.Context(), `SELECT user_id,secret_hash FROM cli_sessions WHERE id=$1 AND expires_at>now() AND consumed_at IS NULL FOR UPDATE`, r.PathValue("id")).Scan(&userID, &expected)
	if err != nil || !hmac.Equal([]byte(hash(in.DeviceSecret)), []byte(expected)) {
		return fail(400, "device_session_invalid", "Device session is invalid or expired")
	}
	if userID == nil {
		return fail(428, "authorization_pending", "Approve the device in your browser")
	}
	token := secret()
	expires := time.Now().UTC().Add(24 * time.Hour)
	if _, err = tx.Exec(r.Context(), `INSERT INTO sessions(token_hash,user_id,scopes,expires_at) VALUES($1,$2,'{cli}',$3)`, hash(token), *userID, expires); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `UPDATE cli_sessions SET consumed_at=now() WHERE id=$1`, r.PathValue("id")); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	respond(w, 200, map[string]any{"token": token, "expiresAt": expires})
	return nil
}
func (s *Server) github(ctx context.Context, method, path, token string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(raw(body)))
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.github.com"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fail(422, "github_fetch_failed", "GitHub could not supply the requested repository, commit, or app permission")
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(res.Body, 32<<20)).Decode(out)
}
