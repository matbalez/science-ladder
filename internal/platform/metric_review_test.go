package platform

import (
	"context"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func acceptedMetricAssessment() *MetricAssessment {
	return &MetricAssessment{Decision: "accepted", ScientificConnection: "A lower verified operation count is a direct computational cost improvement.", PreservedConditions: "Every input and exact mathematical output remain fixed by the correctness checker.", ShortcutAnalysis: "Returning an empty result reduces work but fails the independently checked output constraint.", ClaimScope: "This establishes a lower cost on the declared problem instances, not universal superiority."}
}

func TestScientificMetricCannotPassOnReproducibilityAlone(t *testing.T) {
	m := protocol.Manifest{APIVersion: protocol.ManifestV2}
	for _, assessment := range []*MetricAssessment{nil, {Decision: "accepted", ScientificConnection: "Reproducible"}, {Decision: "unsupported", ScientificConnection: stringsForAssessment(), PreservedConditions: stringsForAssessment(), ShortcutAnalysis: stringsForAssessment(), ClaimScope: stringsForAssessment()}} {
		r := ScienceReview{Outcome: "automated_pass", MetricValidity: "The arithmetic is correct.", MetricAssessment: assessment}
		applyMetricAssessment(&r, m)
		if r.Outcome != "changes_required" {
			t.Fatal("scientific metric omission passed")
		}
	}
	r := ScienceReview{Outcome: "automated_pass", MetricAssessment: acceptedMetricAssessment()}
	applyMetricAssessment(&r, m)
	if r.Outcome != "automated_pass" {
		t.Fatal("complete accepted assessment rejected")
	}
	legacy := ScienceReview{Outcome: "automated_pass"}
	applyMetricAssessment(&legacy, protocol.Manifest{APIVersion: protocol.APIVersion})
	if legacy.Outcome != "automated_pass" {
		t.Fatal("retroactively changed legacy review")
	}
}

func stringsForAssessment() string {
	return "This is a substantive synthetic test assessment, not scientific evidence."
}

func TestMetricReviewBindsManifestAndLatestDecision(t *testing.T) {
	s := testDB(t)
	_, version := seed(t, s)
	ctx := context.Background()
	insert := func(kind, status, digest, decision string, at time.Time) {
		t.Helper()
		report := map[string]any{"manifestDigest": digest, "review": map[string]any{"metricAssessment": map[string]string{"decision": decision}}}
		if _, err := s.DB.Exec(ctx, `INSERT INTO review_runs(id,version_id,kind,status,report,digest,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, ID(), version, kind, status, raw(report), protocol.DigestBytes([]byte(ID())), at); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want bool) {
		t.Helper()
		tx, err := s.DB.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		got, err := metricReviewAccepted(ctx, tx, version, "manifest-a")
		if err != nil || got != want {
			t.Fatalf("metric publication gate got %v want %v: %v", got, want, err)
		}
	}
	base := time.Now().UTC().Add(-time.Hour)
	check(false)
	insert("scientific-legibility", "automated_pass", "manifest-a", "accepted", base)
	check(true)
	insert("scientific-legibility", "changes_required", "manifest-a", "needs_work", base.Add(time.Second))
	check(false)
	insert("scientific-metric", "accepted", "manifest-b", "accepted", base.Add(2*time.Second))
	check(false)
	insert("scientific-metric", "accepted", "manifest-a", "accepted", base.Add(3*time.Second))
	check(true)
	insert("scientific-legibility", "changes_required", "manifest-a", "unsupported", base.Add(4*time.Second))
	check(false)
}
