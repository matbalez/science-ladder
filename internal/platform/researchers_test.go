package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func exampleResearcher() Researcher {
	return Researcher{Name: "Example Researcher", ProfileURL: "https://university.example.org/people/researcher", Connection: "Author of the cited mathematical method.", WorkTitle: "A mathematical method", WorkURL: "https://arxiv.org/abs/1234.56789"}
}

func TestResearcherInputBoundsAndLinkPolicy(t *testing.T) {
	for _, bad := range []string{"http://example.org", "javascript:alert(1)", "mailto:person@example.org", "https://user:pass@example.org", "https://localhost/a", "https://127.0.0.1/a", "https://[::1]/a", "https://127.1/a", "https://2130706433/a", "https://0x7f.0x0.0x0.0x1/a", "https://0x7f.0x0.0x0.0x/a", "https://0177.0.0.01/a", "https://host.local/a", "https://host.internal/a", "https://example.org:8443/a", "https://example.org./a", "https://example.org/%0d%0aHeader:bad", "https://example.org\\@localhost/a", "https://-invalid.example.org/a", "https://exa mple.org/a"} {
		value := bad
		if researcherURL(&value) {
			t.Errorf("unsafe link accepted: %q", bad)
		}
	}
	for _, good := range []string{"https://arxiv.org/abs/2409.07222v1", "https://people.example.edu/researcher#work", "https://EXAMPLE.org:443/paper?q=math", "https://münchen.example.org/person"} {
		value := good
		if !researcherURL(&value) {
			t.Errorf("valid public HTTPS link rejected: %q", good)
		}
	}
	expanded := "https://example.org/" + strings.Repeat("é", 500)
	if researcherURL(&expanded) {
		t.Fatal("URL normalization escaped its persisted byte bound")
	}
	reason := "  Correct the published authorship context.  "
	people := []Researcher{exampleResearcher()}
	people[0].Name = " Example Researcher "
	if err := validateResearchers(people, &reason); err != nil || people[0].Name != "Example Researcher" || reason != "Correct the published authorship context." {
		t.Fatal("normalization failed", err)
	}
	for _, pair := range [][2]string{{"Alice", "alice"}, {"Straße", "STRASSE"}, {"Émile", "E\u0301mile"}} {
		people := []Researcher{exampleResearcher(), exampleResearcher()}
		people[0].Name, people[1].Name = pair[0], pair[1]
		if validateResearchers(people, &reason) == nil {
			t.Fatalf("duplicate normalized name accepted: %q", pair)
		}
	}
	for _, update := range []func(*Researcher){func(r *Researcher) { r.Name = strings.Repeat("x", 121) }, func(r *Researcher) { r.Connection = strings.Repeat("x", 1001) }, func(r *Researcher) { r.WorkTitle = strings.Repeat("x", 301) }, func(r *Researcher) { r.ProfileURL = "" }, func(r *Researcher) { r.Name = "Alice\u202eSmith" }} {
		r := exampleResearcher()
		update(&r)
		if validateResearchers([]Researcher{r}, &reason) == nil {
			t.Fatal("invalid or oversized researcher accepted")
		}
	}
	if validateResearchers(nil, &reason) == nil || validateResearchers(make([]Researcher, 7), &reason) == nil {
		t.Fatal("missing/null or oversized researcher list accepted")
	}
	if err := validateResearchers([]Researcher{}, &reason); err != nil {
		t.Fatal("explicit clearing list rejected", err)
	}
}

func researcherBrowser(t *testing.T, s *Server, role, scope string, githubID int64) string {
	t.Helper()
	id, token := ID(), secret()
	if _, err := s.DB.Exec(context.Background(), `INSERT INTO users(id,github_id,login,role,invited) VALUES($1,$2,$3,$4,true)`, id, githubID, role+"-original", role); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(context.Background(), `INSERT INTO sessions(token_hash,user_id,scopes,expires_at) VALUES($1,$2,$3,now()+interval '1 hour')`, hash(token), id, []string{scope}); err != nil {
		t.Fatal(err)
	}
	return token
}

func researcherRequest(s *Server, token, method, path, key string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(string(body)))
	if token != "" {
		r.AddCookie(&http.Cookie{Name: "sl_session", Value: token})
		r.Header.Set("Origin", s.Config.PublicOrigin)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestResearcherEditorialAuthIdempotencyImmutabilityAndExport(t *testing.T) {
	s := testDB(t)
	u, version := seed(t, s, "platform")
	ctx := context.Background()
	intent := readyIntent(t, s, u, version, 90)
	if _, _, err := accept(t, s, u, intent, "researcher-seed-accept"); err != nil {
		t.Fatal(err)
	}
	const snapshotSQL = `SELECT jsonb_build_object('version',to_jsonb(v),'lock',(SELECT document FROM locks WHERE digest=v.lock_digest),'receipts',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.digest) FROM receipts r),'submissions',(SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id) FROM submissions s),'milestones',(SELECT jsonb_agg(to_jsonb(m) ORDER BY m.id) FROM milestone_tiers m),'claims',(SELECT jsonb_agg(to_jsonb(mc) ORDER BY mc.id) FROM milestone_claims mc),'capacity',(SELECT to_jsonb(c) FROM capacity c WHERE id=1)) FROM challenge_versions v WHERE id=$1`
	var before, after []byte
	if err := s.DB.QueryRow(ctx, snapshotSQL, version).Scan(&before); err != nil {
		t.Fatal(err)
	}
	initial := researcherRequest(s, "", "GET", "/v1/challenges/test-challenge", "", nil)
	var initialChallenge map[string]json.RawMessage
	if initial.Code != 200 || json.Unmarshal(initial.Body.Bytes(), &initialChallenge) != nil || string(initialChallenge["researcherContext"]) != "null" {
		t.Fatal("new challenge context must be null", initial.Code, initial.Body.String())
	}
	editorToken := researcherBrowser(t, s, "editor", "browser", 101)
	memberToken := researcherBrowser(t, s, "member", "browser", 102)
	cliToken := researcherBrowser(t, s, "operator", "cli", 103)
	operatorToken := researcherBrowser(t, s, "operator", "browser", 104)
	path := "/v1/editor/challenge-versions/" + version + "/researchers"
	body := raw(map[string]any{"researchers": []Researcher{exampleResearcher()}, "reason": "Add the relevant author and their cited primary work."})
	for _, token := range []string{"", memberToken, cliToken} {
		w := researcherRequest(s, token, "POST", path, "researcher-auth-test", body)
		if w.Code != 403 {
			t.Fatalf("unauthorized edit accepted: %d %s", w.Code, w.Body.String())
		}
	}
	if w := researcherRequest(s, editorToken, "POST", path, "", body); w.Code != 400 {
		t.Fatal("missing idempotency key accepted", w.Code)
	}
	r := httptest.NewRequest("POST", path, strings.NewReader(string(body)))
	r.AddCookie(&http.Cookie{Name: "sl_session", Value: editorToken})
	r.Header.Set("Origin", "https://untrusted.example.org")
	r.Header.Set("Idempotency-Key", "researcher-origin-test")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin browser edit accepted")
	}
	for _, invalid := range []string{`{"researchers":[],"reason":"Valid public editorial explanation.","contacted":true}`, `{"researchers":[{"name":"Author","profileUrl":"https://example.org","connection":"Author","workTitle":"Paper","workUrl":"https://example.org/paper","sponsor":true}],"reason":"Valid public editorial explanation."}`} {
		if w := researcherRequest(s, editorToken, "POST", path, "researcher-unknown-fields", []byte(invalid)); w.Code != 400 {
			t.Fatal("unsupported contact/endorsement fields accepted", w.Code)
		}
	}
	// Concurrent replays must append one edition and one audit event.
	var wg sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			responses <- researcherRequest(s, editorToken, "POST", path, "researcher-first-edition", body)
		}()
	}
	wg.Wait()
	close(responses)
	firstDigest := ""
	var first json.RawMessage
	for response := range responses {
		if response.Code != 201 {
			t.Fatal("editor mutation or replay failed", response.Code, response.Body.String())
		}
		digest, err := protocol.Digest(json.RawMessage(response.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if firstDigest == "" {
			firstDigest, first = digest, append([]byte(nil), response.Body.Bytes()...)
		} else if digest != firstDigest {
			t.Fatal("idempotent replay changed edition")
		}
	}
	clearBody := raw(map[string]any{"researchers": []Researcher{}, "reason": "Clear this editorial list while preserving its history."})
	if w := researcherRequest(s, editorToken, "POST", path, "researcher-first-edition", clearBody); w.Code != 409 {
		t.Fatal("same-key different content accepted", w.Code)
	}
	if _, err := s.DB.Exec(ctx, `UPDATE users SET login='editor-renamed' WHERE github_id=101`); err != nil {
		t.Fatal(err)
	}
	if w := researcherRequest(s, operatorToken, "POST", path, "researcher-clear-edition", clearBody); w.Code != 201 {
		t.Fatal("operator clearing edition failed", w.Code, w.Body.String())
	}
	for _, query := range []string{`UPDATE challenge_researcher_editions SET reason='Rewrite previous editorial history.' WHERE version_id=$1`, `DELETE FROM challenge_researcher_editions WHERE version_id=$1`} {
		if _, err := s.DB.Exec(ctx, query, version); err == nil {
			t.Fatal("editorial history is mutable")
		}
	}
	var editions, events int
	if err := s.DB.QueryRow(ctx, `SELECT (SELECT count(*) FROM challenge_researcher_editions WHERE version_id=$1),(SELECT count(*) FROM audit_events WHERE version_id=$1 AND kind='editorial.researchers')`, version).Scan(&editions, &events); err != nil || editions != 2 || events != 2 {
		t.Fatal("retries duplicated editions/audit", editions, events, err)
	}
	if err := s.DB.QueryRow(ctx, snapshotSQL, version).Scan(&after); err != nil || string(before) != string(after) {
		t.Fatal("editorial context changed scoring/lock/receipts/admission state", err)
	}
	exported := researcherRequest(s, "", "GET", "/v1/exports/challenge-versions/"+version, "", nil)
	var export struct {
		Challenge struct {
			ResearcherContext struct {
				Researchers []Researcher `json:"researchers"`
			} `json:"researcherContext"`
		} `json:"challenge"`
		History []json.RawMessage `json:"researcherHistory"`
		Audit   []struct {
			Kind    string `json:"kind"`
			Payload struct {
				Digest string `json:"editionDigest"`
			} `json:"payload"`
		} `json:"audit"`
	}
	if exported.Code != 200 || json.Unmarshal(exported.Body.Bytes(), &export) != nil || len(export.History) != 2 || export.Challenge.ResearcherContext.Researchers == nil || len(export.Challenge.ResearcherContext.Researchers) != 0 {
		t.Fatal("public export lost clearing edition/history", exported.Code, exported.Body.String())
	}
	digest, err := protocol.Digest(export.History[0])
	if err != nil || digest != firstDigest || !strings.Contains(string(export.History[0]), "editor-original") || strings.Contains(string(export.History[0]), "editor-renamed") {
		t.Fatal("append-only edition or original editor identity changed", err, string(first))
	}
	editionIndex := 0
	for _, event := range export.Audit {
		if event.Kind != "editorial.researchers" {
			continue
		}
		digest, err := protocol.Digest(export.History[editionIndex])
		if err != nil || event.Payload.Digest != digest {
			t.Fatal("audit does not bind exported edition", err)
		}
		editionIndex++
	}
	if editionIndex != 2 {
		t.Fatal("missing editorial audit bindings")
	}
}

func TestResearcherDraftVisibilityAndAuditPrivacy(t *testing.T) {
	s := testDB(t)
	owner, published := seed(t, s)
	ctx := context.Background()
	draft := ID()
	if _, err := s.DB.Exec(ctx, `INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,deadline) SELECT $1,challenge_id,repository,repository_id,source_commit,source_digest,manifest,deadline FROM challenge_versions WHERE id=$2`, draft, published); err != nil {
		t.Fatal(err)
	}
	editor := researcherBrowser(t, s, "editor", "browser", 201)
	r := exampleResearcher()
	r.Name = "Private Draft Researcher"
	body := raw(map[string]any{"researchers": []Researcher{r}, "reason": "Private draft editorial explanation withheld until publication."})
	if w := researcherRequest(s, editor, "POST", "/v1/editor/challenge-versions/"+draft+"/researchers", "private-researcher-edition", body); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := researcherRequest(s, "", "GET", "/v1/exports/challenge-versions/"+draft, "", nil); w.Code != 404 {
		t.Fatal("unpublished context publicly exported", w.Code)
	}
	public := researcherRequest(s, "", "GET", "/v1/challenges/test-challenge", "", nil)
	if public.Code != 200 || strings.Contains(public.Body.String(), r.Name) {
		t.Fatal("unpublished context leaked through challenge", public.Code)
	}
	var events []byte
	if err := s.DB.QueryRow(ctx, `SELECT jsonb_agg(payload) FROM audit_events WHERE version_id=$1`, draft).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(events), r.Name) || strings.Contains(string(events), "Private draft editorial") {
		t.Fatal("draft editorial content leaked into global audit")
	}
	ownerToken := secret()
	if _, err := s.DB.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,scopes,expires_at) VALUES($1,$2,'{browser}',now()+interval '1 hour')`, hash(ownerToken), owner.ID); err != nil {
		t.Fatal(err)
	}
	private := researcherRequest(s, ownerToken, "GET", "/v1/challenges/test-challenge", "", nil)
	if private.Code != 200 || !strings.Contains(private.Body.String(), r.Name) {
		t.Fatal("owner cannot inspect draft researcher context", private.Code, private.Body.String())
	}
	missing := researcherRequest(s, editor, "POST", "/v1/editor/challenge-versions/not-a-version/researchers", "missing-researcher-version", body)
	if missing.Code != 404 {
		t.Fatal("unknown version must be 404", missing.Code)
	}
}
