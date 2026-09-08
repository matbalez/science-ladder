package protocol

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestEducationCompatibility(t *testing.T) {
	for _, e := range []*Education{nil, {}, {Frontier: "context"}, {Frontier: "context", Significance: "  "}, {Frontier: strings.Repeat("x", 12001), Significance: "meaning"}} {
		if ValidateEducation(e) == nil {
			t.Fatal("accepted missing or oversized education")
		}
	}
	e := Education{Frontier: "The published reference is not known to be optimal.", Significance: "A smaller exact energy improves this finite mathematical object."}
	if err := ValidateEducation(&e); err != nil {
		t.Fatal(err)
	}
	// Adding educational prose must not change the serialized execution contract.
	var m Manifest
	before, _ := json.Marshal(m)
	var c Candidate
	c.Manifest = &m
	c.Education = &e
	after, _ := json.Marshal(c.Manifest)
	if string(before) != string(after) {
		t.Fatal("education changed manifest bytes")
	}
}

func TestNewScoutRequiresEducationButLegacyCandidateRemainsReadable(t *testing.T) {
	b, err := os.ReadFile("../../web/public/examples/quiet-echoes-candidate.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseCandidate(b)
	if err != nil {
		t.Fatal(err)
	}
	c.PromptVersion = "1.2.0"
	if err = ValidateCandidate(c); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"1.3.0", "1.4.0", "1.5.0", "1.6.0", ScoutVersion} {
		c.PromptVersion = version
		if err = ValidateCandidate(c); err == nil || !strings.Contains(err.Error(), "education") {
			t.Fatalf("missing education for %s: %v", version, err)
		}
	}
	c.Education = &Education{Frontier: "Published length-512 reference; optimum unknown.", Significance: "Lower exact energy improves the finite objective, not all signal properties."}
	b, err = json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseCandidate(b)
	if err != nil || parsed.Education == nil || *parsed.Education != *c.Education {
		t.Fatalf("education lost in candidate parsing: %v", err)
	}
}
