package platform

import "testing"

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
