package platform

import (
	"context"
	"encoding/json"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSourceEvidenceIgnoresProgramsAndHiddenMarkup(t *testing.T) {
	page := []byte(`<html><head><meta name="citation_publication_date" content="2025-06-01"><script>secret-script-quote</script></head><body><!--secret-comment-quote--><style>secret-style-quote</style><template>secret-template-quote</template><p hidden>secret-hidden-quote</p><p aria-hidden="true">secret-aria-quote</p><p style="display: none">secret-css-quote</p><p>Actual &amp; visible evidence</p></body></html>`)
	text := visibleSourceText(page, "text/html; charset=utf-8")
	if strings.Contains(text, "secret-") || !strings.Contains(text, "Actual & visible evidence") {
		t.Fatalf("invisible text counted as evidence: %q", text)
	}
	if date, ok := publicationDate(page); !ok || date.Format("2006-01-02") != "2025-06-01" {
		t.Fatal("real primary citation metadata missing")
	}
	if _, ok := publicationDate([]byte(`<!-- <meta name="citation_publication_date" content="2025-01-01"> -->`)); ok {
		t.Fatal("comment forged publication metadata")
	}
	if visibleSourceText([]byte("a quote"), "application/pdf") != "" {
		t.Fatal("opaque PDF bytes treated as verified text")
	}
}

type reviewTransport func(*http.Request) (*http.Response, error)

func (f reviewTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestScientificReviewRechecksFinalManifestSources(t *testing.T) {
	s := testDB(t)
	_, old := seed(t, s)
	ctx := context.Background()
	version := ID()
	manifest := reviewTestManifest()
	if _, err := s.DB.Exec(ctx, `INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,deadline) SELECT $1,challenge_id,repository,repository_id,source_commit,source_digest,$2,deadline FROM challenge_versions WHERE id=$3`, version, raw(manifest), old); err != nil {
		t.Fatal(err)
	}
	configureReviewStorage(t, s, manifest, version)
	s.Config.OpenAIKey = "synthetic-test-key"
	s.Config.OpenAIModel = "synthetic-review-model"
	calls := 0
	s.HTTP = &http.Client{Transport: reviewTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "api.openai.com" {
			t.Fatal("unexpected provider destination")
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		var evidence map[string]any
		if err := json.Unmarshal([]byte(request["input"].(string)), &evidence); err != nil {
			t.Fatal(err)
		}
		if evidence["sourceResolution"] != "failed" {
			t.Fatal("final manifest inherited the old candidate's ready status")
		}
		review := ScienceReview{Outcome: "automated_pass", Summary: "Synthetic attempted approval", Findings: []ReviewFinding{}}
		body := raw(map[string]any{"id": "synthetic-review", "status": "completed", "output": []any{map[string]any{"content": []any{map[string]any{"type": "output_text", "text": string(raw(review))}}}}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: http.Header{}}, nil
	})}
	if err := s.scientificReview(ctx, version); err != nil {
		t.Fatal(err)
	}
	var status, reviewedDigest string
	if err := s.DB.QueryRow(ctx, `SELECT status,report->>'manifestDigest' FROM review_runs WHERE version_id=$1`, version).Scan(&status, &reviewedDigest); err != nil {
		t.Fatal(err)
	}
	want, _ := protocol.Digest(manifest)
	if calls != 1 || status != "changes_required" || reviewedDigest != want {
		t.Fatalf("unresolved source bypassed review binding: %d %s %s", calls, status, reviewedDigest)
	}
}
