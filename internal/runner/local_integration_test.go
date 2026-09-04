package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

// This is an explicitly synthetic protocol fixture, never a scientific claim or
// a challenge to publish. Runtime integration is opt-in because it starts Docker.
func localFixtureManifest(digest string) protocol.Manifest {
	return protocol.Manifest{APIVersion: protocol.APIVersion, Kind: "ChallengeManifest", ID: "protocol-fixture", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Producer: "science-ladder-protocol-tests", Slug: "internal-protocol-fixture", Title: "Internal protocol fixture", Summary: "Synthetic contract test, not a scientific challenge.", ScientificQuestion: "No scientific claim: this checks serialization and isolation only.", Evidence: []protocol.Source{{URL: "https://example.invalid/protocol-fixture", Title: "Explicit synthetic test fixture", Evidence: "Not scientific evidence. Never submit or publish as a challenge.", Location: "Test-only fixture"}}, Impact: "No scientific impact claimed.", Limitations: []string{"Synthetic internal fixture; not eligible for publication."}, SafetyClassification: "low-risk-computational", EconomicMode: "none", Metric: protocol.Metric{Name: "fixture", Direction: "maximize", Unit: "test ticks", Quantum: "1", BaselineTicks: "0", MinimumDeltaTicks: "1", ToleranceTicks: "0"}, HardGates: []string{"valid"}, Milestones: []protocol.Milestone{{ID: "fixture", Title: "Test threshold", ThresholdTicks: "1", Rationale: "Protocol fixture only"}}, Deadline: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Submission: protocol.SubmissionContract{AllowedPaths: []string{"data/"}, AllowedExtensions: []string{".json"}, MaxBytes: 4096, MaxFiles: 4, License: "MIT"}, Validator: protocol.Validator{Profile: "artifact-checker-v1", Entrypoint: []string{"/usr/local/bin/python3", "/sl/challenge/checker.py"}, DependencyLock: "requirements.lock", RuntimeImageDigest: digest}, Suite: protocol.Suite{Visibility: "public", Path: "suite"}, Resources: protocol.Resources{Class: "cpu-small", VCPU: 1, MemoryMB: 256, TimeoutSeconds: 10, MaxOutputBytes: 4096}, Fixtures: []protocol.Fixture{{Name: "baseline", Path: "fixtures/baseline", ExpectedOutcome: "valid", ExpectedTicks: "0"}, {Name: "valid", Path: "fixtures/valid", ExpectedOutcome: "valid", ExpectedTicks: "1"}, {Name: "invalid", Path: "fixtures/invalid", ExpectedOutcome: "hard_gate_failed"}, {Name: "malformed", Path: "fixtures/malformed", ExpectedOutcome: "invalid_output"}}}
}

func TestLocalRuntimeIntegration(t *testing.T) {
	digest := os.Getenv("SL_TEST_RUNTIME_DIGEST")
	if digest == "" {
		t.Skip("set SL_TEST_RUNTIME_DIGEST to opt into isolated local Docker execution")
	}
	if !protocol.ValidDigest(digest) {
		t.Fatal("invalid opt-in runtime digest")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := localFixtureManifest(digest)
	checker := `import json
from pathlib import Path
candidate=json.loads(Path('/sl/submission/data/input.json').read_text())
suite=json.loads(Path('/sl/suite/test.json').read_text())
Path('/sl/output/result.json').write_text(json.dumps({'apiVersion':'science-ladder/v1','kind':'ValidatorResult','score':str(candidate['value']),'gates':{'valid':candidate['value'] <= suite['maximum']}}))
`
	files := map[string][]byte{"checker.py": []byte(checker), "requirements.lock": []byte("# stdlib only"), "suite/test.json": []byte(`{"maximum":10}`), "fixtures/baseline/data/input.json": []byte(`{"value":0}`), "fixtures/valid/data/input.json": []byte(`{"value":1}`), "fixtures/invalid/data/input.json": []byte(`{"value":20}`), "fixtures/malformed/data/input.json": []byte(`malformed`)}
	if err := writeTree(root, files); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range manifest.Fixtures[:3] {
		t.Run(fixture.Name, func(t *testing.T) {
			report, err := LocalValidate(context.Background(), manifest, root, filepath.Join(root, fixture.Path), true)
			if err != nil || report.Official || report.Outcome != fixture.ExpectedOutcome || fixture.ExpectedTicks != "" && report.ScoreTicks != fixture.ExpectedTicks {
				t.Fatalf("%+v %v", report, err)
			}
		})
	}
	manifest.Suite.Visibility = "hidden"
	manifest.Suite.Commitment = protocol.DigestBytes([]byte("test-only commitment"))
	preview, err := LocalValidateWithSuite(context.Background(), manifest, root, filepath.Join(root, "fixtures/valid"), filepath.Join(root, "suite"), true)
	if err != nil || preview.Outcome != "valid" || preview.Official {
		t.Fatalf("private preview: %+v %v", preview, err)
	}
	if _, err := LocalValidate(context.Background(), manifest, root, filepath.Join(root, "fixtures/valid"), true); err == nil {
		t.Fatal("hosted hidden suite silently materialized")
	}
	manifest.Suite.Visibility = "public"
	manifest.Suite.Commitment = ""
	if err := os.WriteFile(filepath.Join(root, "checker.py"), []byte(checker+"\nPath('/sl/output/unexpected.txt').write_text('extra output')\n"), 0644); err != nil {
		t.Fatal(err)
	}
	report, err := LocalValidate(context.Background(), manifest, root, filepath.Join(root, "fixtures/valid"), true)
	if err == nil || report.Outcome != "invalid_output" {
		t.Fatalf("extra output accepted: %+v %v", report, err)
	}
	encoded, _ := json.Marshal(manifest)
	if !json.Valid(encoded) {
		t.Fatal("fixture serialization")
	}
}
