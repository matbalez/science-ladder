package protocol

import (
	"testing"
	"time"
)

func TestDeclaredPreflightAndLargerHardwareHaveCoherentLeases(t *testing.T) {
	m := Manifest{APIVersion: ManifestV2, Resources: Resources{VCPU: 8, MemoryMB: 32768, TimeoutSeconds: 3600}, Fixtures: make([]Fixture, 4)}
	if duration := JobLeaseDuration(m, "preflight"); duration != time.Duration(2*4*3660+600)*time.Second {
		t.Fatal(duration)
	}
	c := ExecutorCapabilities{MaxJobSeconds: 3600}
	if MatchJobLease(m, "preflight", c) {
		t.Fatal("hour authorization admitted multi-hour fixture schedule")
	}
	c.MaxJobSeconds = 10 * 3600
	if !MatchJobLease(m, "preflight", c) {
		t.Fatal("adequate declared authorization rejected")
	}
	m.APIVersion = APIVersion
	if JobLeaseDuration(m, "preflight") != 15*time.Minute {
		t.Fatal("legacy lease semantics changed")
	}
}

func TestImmutableAssetsAreExactCapabilities(t *testing.T) {
	d := DigestBytes([]byte("runtime"))
	asset := EvaluationAsset{Name: "weights", Digest: DigestBytes([]byte("data")), Size: 20 << 30, Visibility: "public", Purpose: "weights"}
	e := EvaluationContract{Executor: ExecutorRequirements{OS: "darwin", Architecture: "arm64", Accelerator: "metal", Features: []string{"isolated-checker"}}, Assets: []EvaluationAsset{asset}}
	c := ExecutorCapabilities{OS: "darwin", Architecture: "arm64", Accelerator: "metal", Features: e.Executor.Features, RuntimeImageDigest: d, MaxVCPU: 16, MaxMemoryMB: 65536, MaxSessionSeconds: 7200, Assets: []EvaluationAsset{asset}}
	r := Resources{VCPU: 8, MemoryMB: 32768, TimeoutSeconds: 3600}
	if err := MatchExecutor(e, r, d, c); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*EvaluationAsset){func(a *EvaluationAsset) { a.Digest = DigestBytes([]byte("drift")) }, func(a *EvaluationAsset) { a.Size++ }, func(a *EvaluationAsset) { a.Visibility = "hidden" }} {
		other := c
		other.Assets = append([]EvaluationAsset(nil), c.Assets...)
		mutate(&other.Assets[0])
		if MatchExecutor(e, r, d, other) == nil {
			t.Fatal("asset identity or disclosure drift accepted")
		}
	}
}
