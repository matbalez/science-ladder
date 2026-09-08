package platform

import (
	"encoding/json"
	"github.com/matbalez/science-ladder/prompts"
	"net/http/httptest"
	"testing"
)

func TestScientificReviewRequiresBothEducationalSections(t *testing.T) {
	for _, doc := range []string{`{}`, `{"education":{"frontier":"A record"}}`, `{"education":{"frontier":" ","significance":"A score"}}`} {
		if validateCandidateEducation([]byte(doc)) == nil {
			t.Fatal("missing educational context passed review gate")
		}
	}
	if err := validateCandidateEducation([]byte(`{"education":{"frontier":"A cited record on a fixed instance.","significance":"An improvement in exact energy, not peak sidelobe or receiver performance."}}`)); err != nil {
		t.Fatal(err)
	}
}

func TestScoutCurrentAndArchivedPromptVersions(t *testing.T) {
	for _, tc := range []struct{ request, version, prompt string }{{"v1", "1.7.0", prompts.Scout}, {"1.7.0", "1.7.0", prompts.Scout}, {"1.6.0", "1.6.0", prompts.ScoutV16}, {"1.5.0", "1.5.0", prompts.ScoutV15}} {
		r := httptest.NewRequest("GET", "/v1/prompts/challenge-scout/"+tc.request, nil)
		r.SetPathValue("version", tc.request)
		w := httptest.NewRecorder()
		if err := (&Server{}).scout(w, r, nil); err != nil {
			t.Fatal(err)
		}
		var got struct {
			Version string `json:"version"`
			Prompt  string `json:"prompt"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Version != tc.version || got.Prompt != tc.prompt {
			t.Fatalf("prompt version was silently changed: %s", tc.request)
		}
	}
}
