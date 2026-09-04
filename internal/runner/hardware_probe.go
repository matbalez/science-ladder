package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

// HardwareProbe executes only the embedded first-party corpus. Its typed receipt
// proves neither cross-host determinism, advisory clearance nor scientific merit.
// Unlike Run/Prepare, it cannot accept source, candidate artifacts or secret data.
func (r *Runtime) HardwareProbe(ctx context.Context, diagnostics io.Writer) (protocol.Envelope, error) {
	if err := r.Config.CheckHost(r.Keys); err != nil {
		return protocol.Envelope{}, err
	}
	if r.Signer == nil {
		return protocol.Envelope{}, errors.New("host signer required")
	}
	workspace, err := os.MkdirTemp(r.Config.WorkRoot, "hardware-probe-")
	if err != nil {
		return protocol.Envelope{}, err
	}
	defer os.RemoveAll(workspace)
	runtime := *r
	runtime.probeDiagnostics = diagnostics
	builder := Builder{Runtime: &runtime, MakeSquashFS: r.Config.MakeSquashFS}
	now := time.Now().UTC()
	id := fmt.Sprintf("hardware-probe-%d", now.UnixNano())
	manifest := hostProbeManifest(r.Config.RuntimeImageDigest)
	parent := protocol.RunnerJob{APIVersion: protocol.APIVersion, Kind: "ValidationJob", ID: id, CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute), Producer: r.Config.HostID, Purpose: "preflight", DeploymentMode: "controlled-demo", OfficialAcceptance: false, Manifest: manifest, RunnerEpoch: r.Config.RunnerEpoch, ExecutionProfileDigest: r.Config.ExecutionProfileDigest, FencingToken: 1}
	parent.VerificationPolicy = protocol.VerificationPlatform
	passed, probeErr := builder.isolationCorpus(ctx, parent, workspace)
	if err := os.RemoveAll(workspace); err != nil {
		return protocol.Envelope{}, errors.New("hardware probe cleanup failed")
	}
	data := map[string]any{"hostId": r.Config.HostID, "hostGroup": r.Config.HostGroup, "passed": passed, "crossHostVerified": false, "advisoryGateSatisfied": false, "cleanupAttested": true, "durationMillis": time.Since(now).Milliseconds(), "scope": "fixed first-party single-host KVM isolation and resource corpus"}
	if probeErr != nil {
		data["failure"] = probeErr.Error()
	}
	receipt := protocol.Receipt{APIVersion: protocol.APIVersion, Kind: "HostConformanceReceipt", ID: id, CreatedAt: time.Now().UTC(), Producer: r.Config.HostID, SubjectDigest: r.Config.ExecutionProfileDigest, EconomicMode: "none", DeploymentMode: "controlled-demo", OfficialAcceptance: false, Data: data}
	receipt.VerificationPolicy = protocol.VerificationPlatform
	envelope, err := protocol.Sign(r.KeyID, r.Signer, receipt)
	if err != nil {
		return protocol.Envelope{}, err
	}
	return envelope, probeErr
}

func hostProbeManifest(digest string) protocol.Manifest {
	return protocol.Manifest{APIVersion: protocol.APIVersion, Kind: "ChallengeManifest", ID: "hardware-protocol-fixture", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Producer: "science-ladder-platform", Slug: "hardware-protocol-fixture", Title: "Internal hardware protocol fixture", Summary: "Fixed platform isolation test; not a scientific challenge.", ScientificQuestion: "No scientific claim; verify hardware boundaries.", Evidence: []protocol.Source{{URL: "https://github.com/matbalez/science-ladder", Title: "First-party protocol fixture", Evidence: "Source of the platform probe, not scientific evidence.", Location: "internal/runner/conformance.go"}}, Impact: "No scientific impact claimed.", Limitations: []string{"Single-host fixed corpus; does not establish security certification or cross-host determinism."}, SafetyClassification: "low-risk-computational", EconomicMode: "none", Metric: protocol.Metric{Name: "fixture", Direction: "maximize", Unit: "checks", Quantum: "1", BaselineTicks: "0", MinimumDeltaTicks: "1", ToleranceTicks: "0"}, HardGates: []string{"isolation"}, Milestones: []protocol.Milestone{{ID: "fixture", Title: "Protocol test", ThresholdTicks: "1", Rationale: "Internal conformance only"}}, Deadline: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Submission: protocol.SubmissionContract{AllowedPaths: []string{"data/"}, AllowedExtensions: []string{".txt"}, MaxBytes: 1 << 20, MaxFiles: 4, License: "MIT"}, Validator: protocol.Validator{Profile: "artifact-checker-v1", Entrypoint: []string{"/usr/local/bin/python3", "/sl/challenge/probe.py"}, DependencyLock: "requirements.lock", RuntimeImageDigest: digest}, Suite: protocol.Suite{Visibility: "public", Path: "suite"}, Resources: protocol.Resources{Class: "cpu-small", VCPU: 1, MemoryMB: 256, TimeoutSeconds: 3, MaxOutputBytes: 4096}, Fixtures: []protocol.Fixture{{Name: "baseline", Path: "fixtures/baseline", ExpectedOutcome: "valid", ExpectedTicks: "0"}, {Name: "valid", Path: "fixtures/valid", ExpectedOutcome: "valid", ExpectedTicks: "1"}, {Name: "invalid", Path: "fixtures/invalid", ExpectedOutcome: "hard_gate_failed"}, {Name: "malformed", Path: "fixtures/malformed", ExpectedOutcome: "invalid_output"}}}
}
