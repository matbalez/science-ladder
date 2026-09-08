package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"path"
	"sort"
	"strings"
)

// EvaluationVersion is an explicit extension boundary. A v1 checker never gains
// executable submissions or new score semantics merely by upgrading its host.
const EvaluationVersion = "science-ladder/evaluation/v1"

// EvaluationContract freezes the scientific interpretation, execution needs and
// public measurement vocabulary independently of the OS-specific executor.
type EvaluationContract struct {
	Version      string                  `json:"version"`
	Mode         string                  `json:"mode"` // artifact, proof, program, performance
	ComparisonID string                  `json:"comparisonId"`
	Executor     ExecutorRequirements    `json:"executor"`
	Measurements []MeasurementDefinition `json:"measurements"`
	Rationale    MetricRationale         `json:"rationale"`
	Program      *CandidateProgram       `json:"program,omitempty"`
	Measurement  *MeasurementPolicy      `json:"measurement,omitempty"`
	Assets       []EvaluationAsset       `json:"assets,omitempty"`
}

type MetricRationale struct {
	Objective           string   `json:"objective"`
	ImprovementMeaning  string   `json:"improvementMeaning"`
	EvidenceURLs        []string `json:"evidenceUrls"`
	PreservedConditions []string `json:"preservedConditions"`
	BaselineReason      string   `json:"baselineReason"`
	MeaningfulDelta     string   `json:"meaningfulDelta"`
	ProxyAttacks        []string `json:"proxyAttacks"`
	PermittedClaim      string   `json:"permittedClaim"`
	ExcludedClaims      []string `json:"excludedClaims"`
}

type MeasurementDefinition struct {
	Name           string `json:"name"`
	Type           string `json:"type"` // integer, decimal, rational
	Unit           string `json:"unit"`
	Role           string `json:"role"`           // primary, constraint, diagnostic
	Interpretation string `json:"interpretation"` // direct, bound, proxy, search-gradient
	Definition     string `json:"definition"`
	Minimum        string `json:"minimum,omitempty"`
	Maximum        string `json:"maximum,omitempty"`
}

// Predicates express achieved properties separately from a ranking gradient.
// They use the declared measurement's exact arithmetic, never display rounding.
type MeasurementPredicate struct {
	Measurement string `json:"measurement"`
	Operator    string `json:"operator"` // eq, ge, le
	Value       string `json:"value"`
}

type ExecutorRequirements struct {
	OS            string   `json:"os"`
	Architecture  string   `json:"architecture"`
	Accelerator   string   `json:"accelerator"` // none, metal, cuda
	HardwareClass string   `json:"hardwareClass,omitempty"`
	Features      []string `json:"features"`
}

// ExecutorCapabilities must come from authenticated platform enrollment, not an
// unauthenticated runner advertisement. The runtime digest pins toolchain bytes.
type ExecutorCapabilities struct {
	OS                 string   `json:"os"`
	Architecture       string   `json:"architecture"`
	Accelerator        string   `json:"accelerator"`
	HardwareClass      string   `json:"hardwareClass"`
	RuntimeImageDigest string   `json:"runtimeImageDigest"`
	Features           []string `json:"features"`
	MaxVCPU            int      `json:"maxVCpu"`
	MaxMemoryMB        int      `json:"maxMemoryMb"`
	MaxSessionSeconds  int      `json:"maxSessionSeconds"`
}

type StageBudget struct {
	TimeoutSeconds int   `json:"timeoutSeconds"`
	MemoryMB       int   `json:"memoryMb"`
	MaxOutputBytes int64 `json:"maxOutputBytes"`
	MaxProcesses   int   `json:"maxProcesses"`
}

// Commands are frozen by the creator; candidate-provided source, build hooks and
// generated executables all run in the candidate domain, including compilation.
// No command may run in the API, host broker or trusted checker process.
type CandidateProgram struct {
	Build       []string    `json:"build"`
	Run         []string    `json:"run"`
	BuildBudget StageBudget `json:"buildBudget"`
	RunBudget   StageBudget `json:"runBudget"`
	MaxRuns     int         `json:"maxRuns"`
	MinRuns     int         `json:"minRuns"`
	ScratchMB   int         `json:"scratchMb"`
}

type EvaluationAsset struct {
	Name       string `json:"name"`
	Digest     string `json:"digest"`
	Size       int64  `json:"size"`
	Visibility string `json:"visibility"`
	Purpose    string `json:"purpose"` // toolchain, weights, dataset, proof
}

// Paired timings are produced by the trusted broker, not candidate stdout.
// The interval is the distribution-free order-statistic interval for the median
// paired speedup; its coverage is computed exactly from the binomial law.
type MeasurementPolicy struct {
	Estimator        string `json:"estimator"` // paired-median-ratio
	Warmups          int    `json:"warmups"`
	Repetitions      int    `json:"repetitions"`
	Order            string `json:"order"` // alternating
	ConfidencePPM    int    `json:"confidencePpm"`
	MaxRelativeWidth string `json:"maxRelativeWidth"`
	MinimumSpeedup   string `json:"minimumSpeedup"`
	BaselineDigest   string `json:"baselineDigest"`
	TimerBoundary    string `json:"timerBoundary"`
	Population       string `json:"population"`
}

func boundedText(s string) bool { return len(strings.TrimSpace(s)) >= 12 && len(s) <= 8192 }

func ValidateMetricRationale(r MetricRationale, sources []Source) error {
	for _, s := range []string{r.Objective, r.ImprovementMeaning, r.BaselineReason, r.MeaningfulDelta, r.PermittedClaim} {
		if !boundedText(s) {
			return errors.New("metric rationale requires substantive objective, interpretation, baseline, delta and bounded claim")
		}
	}
	for _, list := range [][]string{r.PreservedConditions, r.ProxyAttacks, r.ExcludedClaims} {
		if len(list) < 1 || len(list) > 32 {
			return errors.New("metric rationale requires preserved conditions, proxy attacks and excluded claims")
		}
		for _, s := range list {
			if !boundedText(s) {
				return errors.New("metric rationale contains an empty or oversized argument")
			}
		}
	}
	if len(r.EvidenceURLs) < 1 || len(r.EvidenceURLs) > 30 {
		return errors.New("metric rationale must cite supporting evidence")
	}
	seen := map[string]bool{}
	for _, u := range r.EvidenceURLs {
		found := false
		for _, s := range sources {
			found = found || s.URL == u
		}
		if !found || seen[u] {
			return errors.New("metric rationale references absent or duplicate evidence")
		}
		seen[u] = true
	}
	return nil
}

// MeasurementNumber rejects noncanonical integers/rationals, huge operands and
// nonfinite values before arithmetic. Rational values are reduced p/q strings.
func MeasurementNumber(value, kind string) (*big.Rat, error) {
	switch kind {
	case "integer":
		n, err := ParseTicks(value)
		if err != nil {
			return nil, err
		}
		return new(big.Rat).SetInt(n), nil
	case "decimal":
		return decimal(value)
	case "rational":
		parts := strings.Split(value, "/")
		if len(parts) != 2 {
			return nil, errors.New("rational measurement requires reduced numerator/denominator")
		}
		n, err := ParseTicks(parts[0])
		if err != nil {
			return nil, err
		}
		d, err := ParseTicks(parts[1])
		if err != nil || d.Sign() <= 0 {
			return nil, errors.New("rational denominator must be positive")
		}
		if new(big.Int).GCD(nil, nil, n, d).Cmp(big.NewInt(1)) != 0 {
			return nil, errors.New("rational must be reduced")
		}
		return new(big.Rat).SetFrac(n, d), nil
	}
	return nil, errors.New("unknown measurement type")
}

func ValidateEvaluation(e EvaluationContract, m Metric, sources []Source, resources Resources) error {
	if e.Version != EvaluationVersion || !identifierPattern.MatchString(e.ComparisonID) {
		return errors.New("unsupported evaluation version or comparison identity")
	}
	switch e.Mode {
	case "artifact", "proof", "program", "performance":
	default:
		return errors.New("unknown evaluation mode")
	}
	if err := ValidateMetricRationale(e.Rationale, sources); err != nil {
		return err
	}
	r := e.Executor
	if (r.OS != "linux" && r.OS != "darwin") || (r.Architecture != "amd64" && r.Architecture != "arm64") {
		return errors.New("unsupported executor architecture or OS")
	}
	if r.Accelerator != "none" && r.Accelerator != "metal" && r.Accelerator != "cuda" {
		return errors.New("unknown accelerator")
	}
	if r.Accelerator == "metal" && (r.OS != "darwin" || r.Architecture != "arm64") {
		return errors.New("Metal requires macOS arm64")
	}
	if r.Accelerator == "cuda" && r.OS != "linux" {
		return errors.New("CUDA requires a Linux executor")
	}
	if len(r.Features) == 0 || len(r.Features) > 32 {
		return errors.New("explicit executor capabilities required")
	}
	features := map[string]bool{}
	for _, f := range r.Features {
		if !identifierPattern.MatchString(f) || features[f] {
			return errors.New("invalid or repeated capability")
		}
		features[f] = true
	}
	if !features["isolated-checker"] {
		return errors.New("isolated checker capability required")
	}
	if len(e.Measurements) == 0 || len(e.Measurements) > 32 {
		return errors.New("one to thirty-two declared public measurements required")
	}
	primary := 0
	names := map[string]bool{}
	for _, d := range e.Measurements {
		if !identifierPattern.MatchString(d.Name) || names[d.Name] || d.Unit == "" || !boundedText(d.Definition) {
			return errors.New("invalid measurement definition")
		}
		names[d.Name] = true
		if d.Type != "integer" && d.Type != "decimal" && d.Type != "rational" {
			return errors.New("unknown measurement type")
		}
		switch d.Role {
		case "primary":
			primary++
			if d.Name != m.Name || d.Unit != m.Unit {
				return errors.New("primary measurement must match ranking metric")
			}
		case "constraint", "diagnostic":
		default:
			return errors.New("unknown measurement role")
		}
		switch d.Interpretation {
		case "direct", "bound", "proxy", "search-gradient":
		default:
			return errors.New("unknown scientific metric interpretation")
		}
		var min *big.Rat
		if d.Minimum != "" {
			var err error
			min, err = MeasurementNumber(d.Minimum, d.Type)
			if err != nil {
				return err
			}
		}
		if d.Maximum != "" {
			max, err := MeasurementNumber(d.Maximum, d.Type)
			if err != nil {
				return err
			}
			if min != nil && min.Cmp(max) > 0 {
				return errors.New("measurement bounds reversed")
			}
		}
	}
	if primary != 1 {
		return errors.New("exactly one primary measurement required; use separate tracks for incomparable objectives")
	}
	if e.Mode == "program" || e.Mode == "performance" {
		if e.Program == nil || !features["isolated-candidate"] {
			return errors.New("program evaluation requires an isolated candidate and frozen program contract")
		}
	} else if e.Program != nil {
		return errors.New("artifact and proof modes cannot silently execute submitted programs")
	}
	if e.Program != nil {
		p := e.Program
		if err := validateStageArgv(p.Build); err != nil {
			return err
		}
		if err := validateStageArgv(p.Run); err != nil {
			return err
		}
		if p.MaxRuns < 1 || p.MaxRuns > 10000 || p.MinRuns < 1 || p.MinRuns > p.MaxRuns || p.ScratchMB < 16 || p.ScratchMB > resources.MemoryMB {
			return errors.New("invalid candidate run count or scratch budget")
		}
		for _, b := range []StageBudget{p.BuildBudget, p.RunBudget} {
			if b.TimeoutSeconds < 1 || b.TimeoutSeconds > resources.TimeoutSeconds || b.MemoryMB < 32 || b.MemoryMB > resources.MemoryMB-256 || b.MaxOutputBytes < 1 || b.MaxOutputBytes > 1<<30 || b.MaxProcesses < 1 || b.MaxProcesses > 256 {
				return errors.New("stage budget exceeds session envelope")
			}
		}
	}
	if e.Mode == "performance" {
		if e.Measurement == nil || !features["trusted-timing"] || r.HardwareClass == "" {
			return errors.New("performance evaluation requires locked hardware and trusted measurement policy")
		}
		if err := ValidateMeasurementPolicy(*e.Measurement); err != nil {
			return err
		}
		if e.Program.MaxRuns < 2*(e.Measurement.Warmups+e.Measurement.Repetitions) {
			return errors.New("candidate run budget cannot complete paired measurement schedule")
		}
	} else if e.Measurement != nil {
		return errors.New("timing policy requires performance mode")
	}
	if e.Mode == "proof" && !features["native-proof-checker"] {
		return errors.New("proof evaluation requires a proof-checker capability")
	}
	if len(e.Assets) > 32 {
		return errors.New("too many immutable assets")
	}
	assetNames := map[string]bool{}
	for _, a := range e.Assets {
		if !identifierPattern.MatchString(a.Name) || assetNames[a.Name] || !ValidDigest(a.Digest) || a.Size < 1 || a.Size > 1<<40 {
			return errors.New("invalid immutable asset binding")
		}
		assetNames[a.Name] = true
		if a.Visibility != "public" && a.Visibility != "hidden" {
			return errors.New("asset disclosure policy required")
		}
		switch a.Purpose {
		case "toolchain", "weights", "dataset", "proof":
		default:
			return errors.New("unknown asset purpose")
		}
	}
	return nil
}

func validateStageArgv(argv []string) error {
	if len(argv) < 1 || len(argv) > 64 || !path.IsAbs(argv[0]) || path.Clean(argv[0]) != argv[0] {
		return errors.New("stage requires an absolute direct command")
	}
	for _, s := range argv {
		if len(s) > 4096 || strings.ContainsRune(s, 0) {
			return errors.New("invalid stage argument")
		}
	}
	return nil
}

func MatchExecutor(e EvaluationContract, resources Resources, runtimeDigest string, c ExecutorCapabilities) error {
	r := e.Executor
	if r.OS != c.OS || r.Architecture != c.Architecture || r.Accelerator != c.Accelerator || r.HardwareClass != "" && r.HardwareClass != c.HardwareClass {
		return errors.New("no matching operating system, architecture, accelerator or hardware class")
	}
	if !ValidDigest(runtimeDigest) || runtimeDigest != c.RuntimeImageDigest {
		return errors.New("executor runtime differs from locked toolchain")
	}
	if resources.VCPU > c.MaxVCPU || resources.MemoryMB > c.MaxMemoryMB || resources.TimeoutSeconds > c.MaxSessionSeconds {
		return errors.New("executor has insufficient resources")
	}
	for _, required := range r.Features {
		found := false
		for _, available := range c.Features {
			found = found || required == available
		}
		if !found {
			return fmt.Errorf("executor lacks capability %s", required)
		}
	}
	return nil
}

func ValidateMeasurements(values map[string]string, e EvaluationContract) error {
	if len(values) != len(e.Measurements) {
		return errors.New("missing or undeclared measurement; arbitrary diagnostics are forbidden")
	}
	for _, d := range e.Measurements {
		v, ok := values[d.Name]
		if !ok {
			return fmt.Errorf("missing measurement %s", d.Name)
		}
		x, err := MeasurementNumber(v, d.Type)
		if err != nil {
			return fmt.Errorf("measurement %s: %w", d.Name, err)
		}
		if d.Minimum != "" {
			bound, err := MeasurementNumber(d.Minimum, d.Type)
			if err != nil || x.Cmp(bound) < 0 {
				return errors.New("measurement below declared domain")
			}
		}
		if d.Maximum != "" {
			bound, err := MeasurementNumber(d.Maximum, d.Type)
			if err != nil || x.Cmp(bound) > 0 {
				return errors.New("measurement above declared domain")
			}
		}
	}
	return nil
}

func ValidatePredicates(predicates []MeasurementPredicate, e EvaluationContract) error {
	if len(predicates) > 32 {
		return errors.New("too many achievement predicates")
	}
	for _, p := range predicates {
		if p.Operator != "eq" && p.Operator != "ge" && p.Operator != "le" {
			return errors.New("invalid achievement comparison")
		}
		found := false
		for _, d := range e.Measurements {
			if d.Name == p.Measurement {
				found = true
				if _, err := MeasurementNumber(p.Value, d.Type); err != nil {
					return err
				}
			}
		}
		if !found {
			return errors.New("achievement references undeclared measurement")
		}
	}
	return nil
}

func MeetsPredicates(values map[string]string, predicates []MeasurementPredicate, e EvaluationContract) (bool, error) {
	if err := ValidateMeasurements(values, e); err != nil {
		return false, err
	}
	if err := ValidatePredicates(predicates, e); err != nil {
		return false, err
	}
	for _, p := range predicates {
		for _, d := range e.Measurements {
			if d.Name == p.Measurement {
				x, _ := MeasurementNumber(values[d.Name], d.Type)
				y, _ := MeasurementNumber(p.Value, d.Type)
				c := x.Cmp(y)
				if p.Operator == "eq" && c != 0 || p.Operator == "ge" && c < 0 || p.Operator == "le" && c > 0 {
					return false, nil
				}
			}
		}
	}
	return true, nil
}

// SortedMeasurementNames is useful for deterministic presentation and exports.
func SortedMeasurementNames(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for k := range values {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ValidateRunMeasurementEvidence recomputes ranking from the signed typed result
// at ingestion and export verification. An outer tick assertion is insufficient.
func ValidateRunMeasurementEvidence(run RunReceipt, m Manifest) error {
	if m.APIVersion != ManifestV2 {
		if run.ValidatorResult != nil {
			return errors.New("legacy run contains unbound measurement evidence")
		}
		return nil
	}
	if run.Outcome != "valid" && run.Outcome != "hard_gate_failed" {
		return nil
	}
	if run.ValidatorResult == nil {
		return errors.New("v2 run is missing its typed measurement evidence")
	}
	data, err := json.Marshal(run.ValidatorResult)
	if err != nil {
		return err
	}
	result, ticks, err := ValidateResult(data, m)
	if err != nil {
		return err
	}
	if ticks != run.ScoreTicks || len(result.Gates) != len(run.Gates) {
		return errors.New("run score or gates differ from typed evidence")
	}
	failed := false
	for name, value := range result.Gates {
		outer, exists := run.Gates[name]
		if !exists || outer != value {
			return errors.New("run gate differs from typed evidence")
		}
		failed = failed || !value
	}
	if failed != (run.Outcome == "hard_gate_failed") {
		return errors.New("run outcome differs from measurement gates")
	}
	return nil
}

// ConfirmedAchievement requires each fresh accepted run to demonstrate every
// property. The conservative scalar score alone cannot prove a milestone.
func ConfirmedAchievement(m Manifest, milestone Milestone, runs []RunReceipt) (bool, error) {
	if len(milestone.Requires) == 0 {
		return true, nil
	}
	if m.Evaluation == nil || len(runs) != 2 {
		return false, errors.New("achievement lacks two complete measurement records")
	}
	if runs[0].JobID == "" || runs[0].JobID == runs[1].JobID {
		return false, errors.New("achievement confirmation must be a different run")
	}
	for _, run := range runs {
		if run.Outcome != "valid" {
			return false, nil
		}
		if err := ValidateRunMeasurementEvidence(run, m); err != nil {
			return false, err
		}
		ok, err := MeetsPredicates(run.ValidatorResult.Measurements, milestone.Requires, *m.Evaluation)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}
