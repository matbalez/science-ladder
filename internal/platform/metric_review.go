package platform

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

type MetricAssessment struct {
	Decision             string `json:"decision"`
	ScientificConnection string `json:"scientificConnection"`
	PreservedConditions  string `json:"preservedConditions"`
	ShortcutAnalysis     string `json:"shortcutAnalysis"`
	ClaimScope           string `json:"claimScope"`
}

func validateMetricAssessment(a *MetricAssessment) error {
	if a == nil {
		return errors.New("an explicit scientific metric assessment is required")
	}
	if a.Decision != "accepted" && a.Decision != "needs_work" && a.Decision != "unsupported" {
		return errors.New("unknown metric assessment decision")
	}
	for _, s := range []string{a.ScientificConnection, a.PreservedConditions, a.ShortcutAnalysis, a.ClaimScope} {
		if len(strings.TrimSpace(s)) < 20 || len(s) > 8192 {
			return errors.New("metric assessment must address scientific connection, preserved conditions, a concrete shortcut and claim scope")
		}
	}
	return nil
}

func applyMetricAssessment(review *ScienceReview, m protocol.Manifest) {
	if m.APIVersion != protocol.ManifestV2 {
		return
	}
	err := validateMetricAssessment(review.MetricAssessment)
	if err == nil && review.MetricAssessment.Decision == "accepted" {
		return
	}
	message := "The ranked metric does not yet have an accepted scientific rationale"
	if err != nil {
		message = err.Error()
	}
	review.Outcome = "changes_required"
	review.Findings = append(review.Findings, ReviewFinding{"error", "scientific_metric", message, "Explain the connection to scientific progress, preserved conditions, shortcut defenses and bounded claim; obtain an explicit metric decision"})
}

// A status flag alone cannot authorize v2 publication. The accepted assessment
// must bind this exact manifest and cannot precede a newer scientific review.
func metricReviewAccepted(ctx context.Context, tx pgx.Tx, version, manifestDigest string) (bool, error) {
	var accepted bool
	err := tx.QueryRow(ctx, `WITH latest AS (
	 SELECT report,created_at FROM review_runs WHERE version_id=$1 AND kind='scientific-legibility' ORDER BY created_at DESC,id DESC LIMIT 1
	) SELECT EXISTS(SELECT 1 FROM latest WHERE report->>'manifestDigest'=$2 AND report->'review'->'metricAssessment'->>'decision'='accepted')
	OR EXISTS(SELECT 1 FROM review_runs r WHERE r.version_id=$1 AND r.kind='scientific-metric' AND r.status='accepted' AND r.report->>'manifestDigest'=$2
	 AND r.created_at>=COALESCE((SELECT created_at FROM latest),'-infinity'::timestamptz))`, version, manifestDigest).Scan(&accepted)
	return accepted, err
}
