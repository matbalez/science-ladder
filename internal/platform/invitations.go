package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var githubUsernameRE = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)

type invitationIdentity struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Type  string `json:"type"`
}

// Look up the public username, but persist GitHub's stable ID so account renames
// do not transfer an invitation to whoever later acquires the old username.
func (s *Server) invitationIdentity(ctx context.Context, username string) (invitationIdentity, error) {
	var identity invitationIdentity
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if !githubUsernameRE.MatchString(username) {
		return identity, fail(422, "github_username_invalid", "Enter a GitHub username, such as octocat, rather than a profile URL")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/users/"+username, nil)
	if err != nil {
		return identity, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "ScienceLadder")
	client := http.Client{}
	if s.HTTP != nil {
		client = *s.HTTP
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(req)
	if err != nil {
		return identity, fail(503, "github_lookup_unavailable", "GitHub could not be reached. No invitation was created; please try again.")
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return identity, fail(422, "github_user_not_found", "No public GitHub account was found with that username. Check the spelling.")
	}
	if res.StatusCode != 200 {
		return identity, fail(503, "github_lookup_unavailable", "GitHub could not confirm this account. No invitation was created; please try again.")
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&identity); err != nil || identity.ID <= 0 || !strings.EqualFold(identity.Login, username) {
		return identity, fail(502, "github_identity_invalid", "GitHub returned an unexpected account identity. No invitation was created.")
	}
	if identity.Type != "User" {
		return identity, fail(422, "github_personal_account_required", "Invite a person’s GitHub username, rather than an organization or bot.")
	}
	return identity, nil
}
