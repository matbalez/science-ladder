package protocol

import (
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func ValidateCandidate(c Candidate) error {
	if c.APIVersion != APIVersion || c.Kind != "ChallengeCandidate" || !identifierPattern.MatchString(c.ID) || c.Producer == "" || c.CreatedAt.IsZero() || c.PromptVersion != ScoutVersion && c.PromptVersion != "1.5.0" && c.PromptVersion != "1.4.0" && c.PromptVersion != "1.3.0" && c.PromptVersion != "1.2.0" && c.PromptVersion != "1.1.0" && c.PromptVersion != "1.0.0" {
		return errors.New("invalid candidate identity/version")
	}
	if c.Disposition != "viable" && c.Disposition != "needs_work" && c.Disposition != "rejected" {
		return errors.New("invalid candidate disposition")
	}
	if len(c.Sources) > 0 || c.Disposition == "viable" {
		if err := validateSources(c.Sources); err != nil {
			return err
		}
	}
	if c.Disposition == "viable" && c.Manifest == nil {
		return errors.New("viable candidate requires manifest")
	}
	if c.Disposition == "rejected" && len(c.Uncertainties) == 0 {
		return errors.New("rejected candidate requires explicit reasons")
	}
	if c.Education != nil || (c.PromptVersion == ScoutVersion || c.PromptVersion == "1.5.0" || c.PromptVersion == "1.4.0" || c.PromptVersion == "1.3.0") && c.Disposition == "viable" {
		if err := ValidateEducation(c.Education); err != nil {
			return err
		}
	}
	if c.Manifest != nil {
		return ValidateManifest(*c.Manifest)
	}
	return nil
}

func validateSources(sources []Source) error {
	if len(sources) == 0 || len(sources) > 30 {
		return errors.New("one to thirty evidence sources required")
	}
	for _, source := range sources {
		u, err := url.Parse(source.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return errors.New("source requires an HTTPS URL")
		}
		if strings.TrimSpace(source.Title) == "" || strings.TrimSpace(source.Evidence) == "" || strings.TrimSpace(source.Location) == "" {
			return errors.New("source title, evidence and exact location required")
		}
	}
	return nil
}

func ValidateManifest(m Manifest) error {
	if !ValidVerificationPolicy(ManifestVerificationPolicy(m)) {
		return errors.New("verification policy must be platform or independent")
	}
	if (m.APIVersion != APIVersion && m.APIVersion != ManifestV2) || m.Kind != "ChallengeManifest" || !identifierPattern.MatchString(m.ID) || m.Producer == "" || m.CreatedAt.IsZero() {
		return errors.New("invalid manifest identity/version")
	}
	if m.APIVersion == APIVersion {
		if m.Submission.Format != "" {
			return errors.New("versioned source submissions require a v2 manifest")
		}
		if m.Evaluation != nil {
			return errors.New("evaluation extensions require an explicit v2 manifest")
		}
		for _, milestone := range m.Milestones {
			if len(milestone.Requires) != 0 {
				return errors.New("achievement predicates require an explicit v2 manifest")
			}
		}
	} else {
		if m.Evaluation == nil {
			return errors.New("v2 requires a frozen evaluation contract and scientific metric rationale")
		}
		if err := ValidateEvaluation(*m.Evaluation, m.Metric, m.Evidence, m.Resources); err != nil {
			return err
		}
		for _, milestone := range m.Milestones {
			if err := ValidatePredicates(milestone.Requires, *m.Evaluation); err != nil {
				return err
			}
		}
	}
	if len(m.Slug) > 100 || !slugPattern.MatchString(m.Slug) || len(m.Title) < 5 || len(m.Title) > 160 || m.Summary == "" || m.ScientificQuestion == "" || m.Impact == "" {
		return errors.New("scientific question, impact, title, summary and slug required")
	}
	if err := validateSources(m.Evidence); err != nil {
		return err
	}
	if len(m.Limitations) == 0 {
		return errors.New("explicit scientific limitations required")
	}
	if m.EconomicMode != "none" {
		return errors.New("only economicMode none is supported")
	}
	if m.SafetyClassification != "low-risk-computational" && m.SafetyClassification != "review-required" {
		return errors.New("unsupported safety classification")
	}
	if !m.Deadline.After(m.CreatedAt) {
		return errors.New("deadline must follow creation")
	}
	if m.Metric.Name == "" || m.Metric.Unit == "" {
		return errors.New("metric name and unit required")
	}
	q, err := decimal(m.Metric.Quantum)
	if err != nil || q.Sign() <= 0 {
		return errors.New("invalid quantum")
	}
	if m.Metric.Direction != "maximize" && m.Metric.Direction != "minimize" {
		return errors.New("metric direction must be maximize or minimize")
	}
	baseline, err := ParseTicks(m.Metric.BaselineTicks)
	if err != nil {
		return fmt.Errorf("baseline: %w", err)
	}
	minimum, err := ParseTicks(m.Metric.MinimumDeltaTicks)
	if err != nil || minimum.Sign() <= 0 {
		return errors.New("minimum delta must be positive integer ticks")
	}
	tolerance, err := ParseTicks(m.Metric.ToleranceTicks)
	if err != nil || tolerance.Sign() < 0 {
		return errors.New("tolerance must be nonnegative ticks")
	}
	for _, bound := range []string{m.Metric.DomainMinTicks, m.Metric.DomainMaxTicks} {
		if bound != "" {
			if _, err := ParseTicks(bound); err != nil {
				return errors.New("invalid metric domain")
			}
		}
	}
	if m.Metric.DomainMinTicks != "" && m.Metric.DomainMaxTicks != "" {
		cmp, _ := CompareTicks(m.Metric.DomainMinTicks, m.Metric.DomainMaxTicks)
		if cmp >= 0 {
			return errors.New("metric domain must increase")
		}
	}
	if len(m.Milestones) == 0 || len(m.Milestones) > 50 {
		return errors.New("one to fifty milestones required")
	}
	previous := baseline
	ids := map[string]bool{}
	for _, milestone := range m.Milestones {
		if !identifierPattern.MatchString(milestone.ID) || ids[milestone.ID] || milestone.Title == "" || milestone.Rationale == "" {
			return errors.New("invalid or duplicate milestone")
		}
		ids[milestone.ID] = true
		ticks, err := ParseTicks(milestone.ThresholdTicks)
		if err != nil {
			return err
		}
		delta := new(big.Int).Sub(ticks, previous)
		if m.Metric.Direction == "minimize" {
			delta.Neg(delta)
		}
		if delta.Sign() <= 0 {
			return errors.New("milestone thresholds must strictly improve")
		}
		fromBaseline := new(big.Int).Sub(ticks, baseline)
		if m.Metric.Direction == "minimize" {
			fromBaseline.Neg(fromBaseline)
		}
		if fromBaseline.Cmp(minimum) < 0 {
			return errors.New("milestone below meaningful delta")
		}
		previous = ticks
	}
	if len(m.HardGates) == 0 || len(m.HardGates) > 32 {
		return errors.New("one to thirty-two hard gates required")
	}
	gates := map[string]bool{}
	for _, gate := range m.HardGates {
		if !identifierPattern.MatchString(gate) || gates[gate] {
			return errors.New("invalid or duplicate hard gate")
		}
		gates[gate] = true
	}
	if err := ValidateSubmissionContract(m.Submission); err != nil {
		return err
	}
	native := m.APIVersion == ManifestV2 && m.Validator.Profile == "native-evaluator-v2"
	if (!native && m.Validator.Profile != "artifact-checker-v1") || m.Validator.DependencyLock == "" || !ValidDigest(m.Validator.RuntimeImageDigest) {
		return errors.New("locked, supported runtime profile and dependency lock required")
	}
	if m.Submission.Format == "source-v2" && !native {
		return errors.New("executable source requires the native evaluation profile")
	}

	if native && (m.Evaluation.Mode == "program" || m.Evaluation.Mode == "performance") && m.Submission.Format != "source-v2" {
		return errors.New("submitted programs require source-v2 artifact semantics")
	}
	if m.Evaluation != nil && m.Evaluation.Mode != "artifact" && m.Validator.Profile == "artifact-checker-v1" {
		return errors.New("proof, program and performance evaluation require a separately enrolled native execution profile")
	}
	if err := ValidatePath(m.Validator.DependencyLock); err != nil {
		return errors.New("dependency lock must be a safe relative source path")
	}
	if len(m.Validator.Entrypoint) < 2 || len(m.Validator.Entrypoint) > 16 || m.Validator.Entrypoint[0] != "/usr/local/bin/python3" {
		return errors.New("validator entrypoint must be a direct Python argv")
	}
	if !strings.HasPrefix(m.Validator.Entrypoint[1], "/sl/challenge/") || path.Clean(m.Validator.Entrypoint[1]) != m.Validator.Entrypoint[1] || !strings.HasSuffix(m.Validator.Entrypoint[1], ".py") {
		return errors.New("validator entrypoint must name a locked Python checker")
	}
	for _, arg := range m.Validator.Entrypoint {
		if len(arg) > 1024 || strings.ContainsRune(arg, 0) {
			return errors.New("invalid argv")
		}
	}
	if m.Suite.Visibility != "public" && m.Suite.Visibility != "hidden" {
		return errors.New("suite visibility required")
	}
	if err := ValidatePath(strings.TrimSuffix(m.Suite.Path, "/")); err != nil {
		return errors.New("invalid suite path")
	}
	if m.Suite.Visibility == "hidden" && !ValidDigest(m.Suite.Commitment) {
		return errors.New("hidden suite requires precommitment")
	}
	r := m.Resources
	if r.Class != "cpu-small" && r.Class != "cpu-medium" && !(m.APIVersion == ManifestV2 && (r.Class == "cpu-large" || r.Class == "accelerator")) {
		return errors.New("unsupported resource class")
	}
	maxCPU, maxMemory, maxSeconds := 4, 8192, 600
	if m.APIVersion == ManifestV2 {
		maxCPU, maxMemory, maxSeconds = 64, 524288, 7200
	}
	if r.VCPU < 1 || r.VCPU > maxCPU || r.MemoryMB < 128 || r.MemoryMB > maxMemory || r.TimeoutSeconds < 1 || r.TimeoutSeconds > maxSeconds || r.MaxOutputBytes < 1024 || r.MaxOutputBytes > 65536 {
		return errors.New("resource class bounds exceeded")
	}
	if len(m.Fixtures) < 4 || len(m.Fixtures) > 100 {
		return errors.New("at least baseline, valid, invalid and malformed fixtures required")
	}
	if m.APIVersion == ManifestV2 && int64(2*len(m.Fixtures))*int64(r.TimeoutSeconds+60)+600 > 18*3600 {
		return errors.New("declared preflight schedule exceeds the 18-hour authorization envelope")
	}
	names := map[string]bool{}
	for _, fixture := range m.Fixtures {
		if !identifierPattern.MatchString(fixture.Name) || names[fixture.Name] {
			return errors.New("invalid or duplicate fixture")
		}
		names[fixture.Name] = true
		if err := ValidatePath(fixture.Path); err != nil {
			return err
		}
		switch fixture.ExpectedOutcome {
		case "valid", "hard_gate_failed", "invalid_output", "resource_limit":
		default:
			return errors.New("unsupported fixture outcome")
		}
		if m.Evaluation != nil && m.Evaluation.Mode == "performance" && fixture.ExpectedTicks != "" {
			return errors.New("measured fixtures use timing evidence, not predeclared exact ticks")
		}
		if fixture.ExpectedTicks != "" {
			if _, err := ParseTicks(fixture.ExpectedTicks); err != nil {
				return err
			}
		}
		switch fixture.Name {
		case "baseline":
			if fixture.ExpectedOutcome != "valid" || (m.Evaluation == nil || m.Evaluation.Mode != "performance") && fixture.ExpectedTicks != m.Metric.BaselineTicks {
				return errors.New("baseline fixture must declare the valid baseline ticks")
			}
		case "valid":
			if fixture.ExpectedOutcome != "valid" {
				return errors.New("valid fixture must pass")
			}
		case "invalid":
			if fixture.ExpectedOutcome != "hard_gate_failed" && fixture.ExpectedOutcome != "invalid_output" {
				return errors.New("invalid fixture must fail a hard gate or parsing")
			}
		case "malformed":
			if fixture.ExpectedOutcome != "invalid_output" {
				return errors.New("malformed fixture must fail parsing")
			}
		}
	}
	for _, required := range []string{"baseline", "valid", "invalid", "malformed"} {
		if !names[required] {
			return fmt.Errorf("required fixture %s missing", required)
		}
	}
	return nil
}

func ValidateSubmissionContract(c SubmissionContract) error {
	if c.Format != "" && c.Format != "source-v2" {
		return errors.New("unknown artifact content format")
	}
	if c.MaxBytes < 1 || c.MaxBytes > 64<<20 || c.MaxFiles < 1 || c.MaxFiles > 4096 || c.License == "" || len(c.AllowedPaths) == 0 || len(c.AllowedPaths) > 32 || len(c.AllowedExtensions) == 0 {
		return errors.New("invalid submission limits/license/paths")
	}
	for _, p := range c.AllowedPaths {
		if err := ValidatePath(strings.TrimSuffix(p, "/")); err != nil {
			return err
		}
	}
	for _, extension := range c.AllowedExtensions {
		if c.Format == "source-v2" {
			if len(extension) > 17 || extension != "" && !regexp.MustCompile(`^\.[a-z0-9]{1,16}$`).MatchString(extension) {
				return errors.New("invalid source extension")
			}
			continue
		}
		switch extension {
		case ".json", ".csv", ".tsv", ".txt", ".dat", ".npy", ".bin":
		default:
			return fmt.Errorf("unsupported data artifact extension %s", extension)
		}
	}
	return nil
}

// Legacy locks omit education. New scientific reviews require it; replay does not.
func ValidateEducation(e *Education) error {
	if e == nil || strings.TrimSpace(e.Frontier) == "" || strings.TrimSpace(e.Significance) == "" {
		return errors.New("education.frontier and education.significance are required: explain the research frontier and why metric progress matters")
	}
	if len(e.Frontier) > 12000 || len(e.Significance) > 12000 {
		return errors.New("each education section must be at most 12,000 bytes")
	}
	return nil
}
