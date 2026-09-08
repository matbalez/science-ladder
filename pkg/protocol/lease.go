package protocol

import "time"

// The authorization lease covers the entire declared job, including both fresh
// fixture runs during preflight. Session time remains a separate VM budget.
const MaximumJobLease = 18 * time.Hour

func JobLeaseDuration(m Manifest, purpose string) time.Duration {
	if m.APIVersion != ManifestV2 {
		return 15 * time.Minute
	}
	seconds := int64(m.Resources.TimeoutSeconds) + 60
	if purpose == "preflight" {
		seconds *= 2 * int64(len(m.Fixtures))
	}
	if purpose == "artifact_prepare" {
		seconds = 300
	}
	duration := time.Duration(seconds+600) * time.Second
	if duration < 15*time.Minute {
		duration = 15 * time.Minute
	}
	if duration > MaximumJobLease || duration < 0 {
		return MaximumJobLease
	}
	return duration
}

func MatchJobLease(m Manifest, purpose string, c ExecutorCapabilities) bool {
	limit := c.MaxJobSeconds
	if limit == 0 {
		limit = 900
	}
	return JobLeaseDuration(m, purpose) <= time.Duration(limit)*time.Second
}
