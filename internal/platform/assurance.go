package platform

import (
	"crypto"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

// Fixture evidence proves separate signed isolated attempts, rather than
// trusting a summary counter. The enrolled host is still one trust domain.
func validateFixtureEvidence(job protocol.RunnerJob, parent protocol.RunReceipt, host runnerIdentity) error {
	key, err := parseRunnerKey(host.PublicKey)
	if err != nil {
		return err
	}
	parentDigest, err := protocol.Digest(job)
	if err != nil {
		return err
	}
	seenJobs, seenReceipts := map[string]bool{}, map[string]bool{}
	total := 0
	for _, fixture := range parent.BuildReport.Fixtures {
		if fixture.Stage == "artifact_parser" && fixture.Name != "baseline" && fixture.Name != "valid" && fixture.Outcome == "invalid_output" && fixture.FreshVMRuns == 0 && len(fixture.RunReceipts) == 0 {
			continue
		}
		if fixture.Stage != "isolated_execution" || fixture.FreshVMRuns != 2 || len(fixture.RunReceipts) != 2 {
			return fail(422, "fixture_repeat_evidence_required", "Every executable fixture requires two signed fresh-VM attempts")
		}
		artifact := ""
		for _, envelope := range fixture.RunReceipts {
			payload, err := protocol.Verify(envelope, map[string]crypto.PublicKey{host.ID: key})
			if err != nil {
				return fail(422, "fixture_signature_invalid", "Fixture attempt is not signed by the enrolled preflight host")
			}
			var run protocol.RunReceipt
			if err = protocol.DecodeStrict(payload, &run); err != nil {
				return err
			}
			if run.APIVersion != protocol.APIVersion || run.Kind != "ValidationRunReceipt" || !run.Official || !run.CleanupAttested || run.ID == "" || run.JobID == "" || run.ID == parent.ID || run.JobID == job.ID || seenJobs[run.JobID] || seenReceipts[run.ID] {
				return fail(422, "fixture_attempt_invalid", "Fixture attempts must be distinct isolated runs with teardown evidence")
			}
			if run.ParentJobDigest != parentDigest || run.HostID != host.ID || run.HostGroup != host.Group || run.VerificationPolicy != job.VerificationPolicy || run.DeploymentMode != job.DeploymentMode || run.OfficialAcceptance != job.OfficialAcceptance || run.ExecutionProfileDigest != job.ExecutionProfileDigest || run.RunnerEpoch != job.RunnerEpoch || run.FencingToken != job.FencingToken || run.ChallengeLockDigest != job.ChallengeLockDigest || run.SuiteDigest != parent.BuildReport.SuiteDigest || run.CreatedAt.Before(job.CreatedAt) || run.CreatedAt.After(parent.CreatedAt) || !protocol.ValidDigest(run.JobDigest) || !protocol.ValidDigest(run.ArtifactDigest) {
				return fail(422, "fixture_bindings_invalid", "Fixture attempt does not bind this preflight, host, suite, and execution policy")
			}
			if run.Outcome != fixture.Outcome || run.ScoreTicks != fixture.ScoreTicks || (artifact != "" && artifact != run.ArtifactDigest) {
				return fail(422, "fixture_repeat_disagreement", "Repeated fixture attempts must agree on artifact, outcome, and exact ticks")
			}
			if run.Outcome == "valid" {
				if len(run.Gates) != len(job.Manifest.HardGates) {
					return fail(422, "fixture_gates_invalid", "Every fixture hard gate must pass")
				}
				for _, gate := range job.Manifest.HardGates {
					if !run.Gates[gate] {
						return fail(422, "fixture_gates_invalid", "Every fixture hard gate must pass")
					}
				}
			}
			artifact = run.ArtifactDigest
			seenJobs[run.JobID], seenReceipts[run.ID] = true, true
			total++
		}
	}
	if total < 4 || parent.BuildReport.FreshVMRuns != total {
		return fail(422, "fixture_repeat_count_invalid", "Baseline and valid fixtures require distinct signed fresh-VM repeats")
	}
	return nil
}
