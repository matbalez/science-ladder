package protocol

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestMeasurementNumbersRejectForgedProofParameters(t *testing.T) {
	for _, value := range []string{"1/0", "2/4", "01/2", "1/-2", "-0/1", "NaN/1", "1e2/1", strings.Repeat("9", 161) + "/1"} {
		if _, err := MeasurementNumber(value, "rational"); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	for _, value := range []string{"0/1", "-1/2", "999999999999999999999999999999/1000000000000000000000000000000"} {
		if _, err := MeasurementNumber(value, "rational"); err != nil {
			t.Errorf("rejected %q: %v", value, err)
		}
	}
}

func TestGradientCannotClaimCompleteCorona(t *testing.T) {
	e := EvaluationContract{Measurements: []MeasurementDefinition{{Name: "gradient", Type: "decimal"}, {Name: "coronas", Type: "integer"}, {Name: "nonTiling", Type: "integer", Minimum: "0", Maximum: "1"}}}
	values := map[string]string{"gradient": "4.999999999999999999", "coronas": "4", "nonTiling": "1"}
	p := []MeasurementPredicate{{Measurement: "coronas", Operator: "ge", Value: "5"}, {Measurement: "nonTiling", Operator: "eq", Value: "1"}}
	if ok, err := MeetsPredicates(values, p, e); err != nil || ok {
		t.Fatalf("partial corona earned full milestone: %v %v", ok, err)
	}
	values["coronas"] = "5"
	if ok, err := MeetsPredicates(values, p, e); err != nil || !ok {
		t.Fatalf("valid achievement rejected: %v %v", ok, err)
	}
	values["nonTiling"] = "0"
	if ok, _ := MeetsPredicates(values, p, e); ok {
		t.Fatal("missing non-tiling proof accepted")
	}
	values["hidden-answer"] = "123"
	if _, err := MeetsPredicates(values, p, e); err == nil {
		t.Fatal("undeclared diagnostic channel accepted")
	}
}

func TestAppleExecutionIsAnExplicitCapabilityMatch(t *testing.T) {
	digest := DigestBytes([]byte("pinned runtime"))
	e := EvaluationContract{Executor: ExecutorRequirements{OS: "darwin", Architecture: "arm64", Accelerator: "metal", HardwareClass: "m3-max-36gb", Features: []string{"isolated-checker", "isolated-candidate", "trusted-timing"}}}
	r := Resources{VCPU: 8, MemoryMB: 32768, TimeoutSeconds: 3600}
	mac := ExecutorCapabilities{OS: "darwin", Architecture: "arm64", Accelerator: "metal", HardwareClass: "m3-max-36gb", RuntimeImageDigest: digest, Features: e.Executor.Features, MaxVCPU: 16, MaxMemoryMB: 36864, MaxSessionSeconds: 7200}
	if err := MatchExecutor(e, r, digest, mac); err != nil {
		t.Fatal(err)
	}
	linux := mac
	linux.OS = "linux"
	linux.Architecture = "amd64"
	linux.Accelerator = "none"
	if MatchExecutor(e, r, digest, linux) == nil {
		t.Fatal("Linux host offered for a Metal workload")
	}
	wrong := mac
	wrong.HardwareClass = "m4-pro-48gb"
	if MatchExecutor(e, r, digest, wrong) == nil {
		t.Fatal("different hardware admitted to same timing series")
	}
	wrong = mac
	wrong.Features = []string{"isolated-checker", "isolated-candidate"}
	if MatchExecutor(e, r, digest, wrong) == nil {
		t.Fatal("untrusted timing accepted")
	}
	wrong = mac
	wrong.RuntimeImageDigest = DigestBytes([]byte("changed compiler"))
	if MatchExecutor(e, r, digest, wrong) == nil {
		t.Fatal("toolchain drift accepted")
	}
}

func timingPolicy() MeasurementPolicy {
	return MeasurementPolicy{BaselinePath: "baseline/source", BaselineBuild: []string{"/usr/bin/gcc", "main.c", "-o", "main"}, BaselineRun: []string{"/work/main"}, Estimator: "paired-median-ratio", Warmups: 2, Repetitions: nineTrials, Order: "alternating", ConfidencePPM: 950000, MaxRelativeWidth: "0.1", MinimumSpeedup: "1.02", BaselineDigest: DigestBytes([]byte("baseline")), TimerBoundary: "Process start through complete output and synchronization.", Population: "The locked workload cases on this exact enrolled hardware."}
}

const nineTrials = 9

func TestPairedMeasurementDoesNotPromoteNoiseOrDropFailures(t *testing.T) {
	p := timingPolicy()
	trials := make([]TimingTrial, 9)
	for i := range trials {
		trials[i] = TimingTrial{BaselineNanos: 200, CandidateNanos: 100, BaselineFirst: i%2 == 0, QualityPassed: true}
	}
	r, err := SummarizeTimings(trials, p)
	if err != nil || r.LowerRatio != "2/1" || r.UpperRatio != "2/1" || !r.PracticalGain || r.Outcome != "valid" {
		t.Fatalf("stable gain: %+v %v", r, err)
	}
	trials[0].QualityPassed = false
	if _, err = SummarizeTimings(trials, p); err == nil {
		t.Fatal("dropped quality failure")
	}
	trials[0].QualityPassed = true
	if _, err = SummarizeTimings(trials[:8], p); err == nil {
		t.Fatal("accepted early stopping")
	}
	trials[0].CandidateNanos = 0
	if _, err = SummarizeTimings(trials, p); err == nil {
		t.Fatal("accepted zero timer")
	}
	trials[0].CandidateNanos = 100
	for i := range trials {
		trials[i].BaselineNanos = int64(100 + i*40)
	}
	r, err = SummarizeTimings(trials, p)
	if err != nil || r.Outcome != "inconclusive" || r.PracticalGain {
		t.Fatalf("unstable gain promoted: %+v %v", r, err)
	}
	p.ConfidencePPM = 999999
	if ValidateMeasurementPolicy(p) == nil {
		t.Fatal("impossible confidence accepted with nine samples")
	}
}

func TestMedianIntervalUsesPairedRatiosAndExactBinomialCoverage(t *testing.T) {
	if k := medianIntervalIndex(9, 950000); k != 2 {
		t.Fatalf("nine-trial 95%% interval uses order statistic %d, want 2", k)
	}
	p := timingPolicy()
	trials := make([]TimingTrial, 9)
	// Median(baseline)/median(candidate) would give 1 here, while the
	// median of paired ratios is 2. Pairing cannot be discarded.
	for i := range trials {
		b, c := int64(2), int64(1)
		if i < 4 {
			b, c = 10000, 100
		}
		if i > 4 {
			b, c = 100, 10000
		}
		trials[i] = TimingTrial{b, c, i%2 == 0, true}
	}
	r, err := SummarizeTimings(trials, p)
	if err != nil || r.MedianRatio != "2/1" {
		t.Fatalf("pairing lost: %+v %v", r, err)
	}
}

func TestV2ResultBindsSeriesPrimaryValueAndRunEvidence(t *testing.T) {
	e := &EvaluationContract{ComparisonID: "proof-radius-v2", Measurements: []MeasurementDefinition{{Name: "radius", Type: "rational"}}}
	m := Manifest{APIVersion: ManifestV2, Evaluation: e, Metric: Metric{Name: "radius", Quantum: "0.1", Direction: "maximize"}, HardGates: []string{"proof"}}
	r := ValidatorResult{APIVersion: ManifestV2, Kind: "ValidatorResult", ComparisonID: e.ComparisonID, Score: "2/3", Measurements: map[string]string{"radius": "2/3"}, Gates: map[string]bool{"proof": true}}
	data, _ := json.Marshal(r)
	_, ticks, err := ValidateResult(data, m)
	if err != nil || ticks != "6" {
		t.Fatalf("rational ranking incorrectly rounded: %s %v", ticks, err)
	}
	run := RunReceipt{Outcome: "valid", ScoreTicks: ticks, Gates: r.Gates, ValidatorResult: &r}
	if err := ValidateRunMeasurementEvidence(run, m); err != nil {
		t.Fatal(err)
	}
	run.ScoreTicks = "7"
	if ValidateRunMeasurementEvidence(run, m) == nil {
		t.Fatal("outer receipt inflated the exact measurement")
	}
	run.ScoreTicks = ticks
	r.ComparisonID = "other-series"
	if ValidateRunMeasurementEvidence(run, m) == nil {
		t.Fatal("cross-series result accepted")
	}
	r.ComparisonID = e.ComparisonID
	r.Score = "3/4"
	if ValidateRunMeasurementEvidence(run, m) == nil {
		t.Fatal("primary value and score differ")
	}
	r.Score = "2/3"
	m.APIVersion = APIVersion
	if ValidateRunMeasurementEvidence(run, m) == nil {
		t.Fatal("v2 evidence silently downgraded into a v1 lock")
	}
}

func TestLegacySchemaAndManifestStayFrozen(t *testing.T) {
	for name := range SchemaTypes {
		actual, err := Schema(name)
		if err != nil {
			t.Fatal(err)
		}
		frozen, err := os.ReadFile("../../protocol/schemas/" + name + "-v1.schema.json")
		if err != nil {
			t.Fatal(err)
		}
		var expected map[string]any
		if err = json.Unmarshal(frozen, &expected); err != nil {
			t.Fatal(err)
		}
		a, _ := Digest(actual)
		b, _ := Digest(expected)
		if a != b {
			t.Errorf("legacy schema drift: %s", name)
		}
	}
	data, err := os.ReadFile("../../web/public/examples/quiet-echoes-manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if m.Evaluation != nil {
		t.Fatal("legacy manifest acquired evaluation semantics")
	}
	before, err := Digest(m)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(m)
	parsed, err := ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := Digest(parsed)
	if before != after {
		t.Fatal("legacy round trip changed its lock input")
	}
	m.Evaluation = &EvaluationContract{}
	if ValidateManifest(m) == nil {
		t.Fatal("v1 manifest accepted v2 fields")
	}
}

func timingRun(t *testing.T, offset int64) (Manifest, RunReceipt) {
	t.Helper()
	p := timingPolicy()
	e := &EvaluationContract{Mode: "performance", ComparisonID: "paired-test-v2", Measurement: &p, Measurements: []MeasurementDefinition{{Name: "speedup", Type: "rational"}}}
	m := Manifest{APIVersion: ManifestV2, Evaluation: e, Metric: Metric{Name: "speedup", Quantum: "0.001", Direction: "maximize"}, HardGates: []string{"quality"}}
	evidence := &TimingEvidence{BaselineDigest: p.BaselineDigest}
	for i := 0; i < p.Warmups; i++ {
		evidence.Warmups = append(evidence.Warmups, TimingTrial{200, 100, i%2 == 0, true})
	}
	for i := 0; i < p.Repetitions; i++ {
		evidence.Trials = append(evidence.Trials, TimingTrial{200 + offset + int64(i), 100, i%2 == 0, true})
	}
	var err error
	evidence.Summary, err = SummarizeTimings(evidence.Trials, p)
	if err != nil {
		t.Fatal(err)
	}
	result := &ValidatorResult{APIVersion: ManifestV2, Kind: "ValidatorResult", ComparisonID: e.ComparisonID, Score: evidence.Summary.LowerRatio, Measurements: map[string]string{"speedup": evidence.Summary.LowerRatio}, Gates: map[string]bool{"quality": true}, Timing: evidence}
	data, _ := json.Marshal(result)
	_, ticks, err := ValidateResult(data, m)
	if err != nil {
		t.Fatal(err)
	}
	return m, RunReceipt{Outcome: "valid", ScoreTicks: ticks, Gates: result.Gates, ValidatorResult: result}
}

func TestMeasuredConfirmationRecomputesEvidenceAndDoesNotUseScalarTolerance(t *testing.T) {
	m, a := timingRun(t, 0)
	_, b := timingRun(t, 1)
	if _, err := ConfirmScores(a.ScoreTicks, b.ScoreTicks, Metric{Direction: "maximize", ToleranceTicks: "0"}); err == nil {
		t.Fatal("test requires distinct scalars")
	}
	if ticks, err := ConfirmMeasuredRuns(a, b, m); err != nil || ticks != a.ScoreTicks {
		t.Fatalf("overlapping fresh intervals rejected: %s %v", ticks, err)
	}
	_, b = timingRun(t, 100)
	if _, err := ConfirmMeasuredRuns(a, b, m); err == nil {
		t.Fatal("non-overlapping measurement series accepted")
	}
	for _, mutate := range []func(*RunReceipt){
		func(r *RunReceipt) { r.ScoreTicks = "999999" },
		func(r *RunReceipt) { r.ValidatorResult.Timing.Trials[0].QualityPassed = false },
		func(r *RunReceipt) { r.ValidatorResult.Timing.Trials = r.ValidatorResult.Timing.Trials[:8] },
		func(r *RunReceipt) { r.ValidatorResult.Timing.Summary.LowerRatio = "99/1" },
		func(r *RunReceipt) { r.ValidatorResult.Timing.BaselineDigest = DigestBytes([]byte("other")) },
		func(r *RunReceipt) { r.ValidatorResult.Timing.Warmups[0].BaselineNanos = 86401e9 },
	} {
		_, b = timingRun(t, 0)
		mutate(&b)
		if ValidateRunMeasurementEvidence(b, m) == nil {
			t.Fatal("forged timing evidence accepted")
		}
	}
	m.APIVersion = APIVersion
	if _, err := ConfirmMeasuredRuns(a, a, m); err == nil {
		t.Fatal("legacy lock accepted measured confirmation")
	}
}
