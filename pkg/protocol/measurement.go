package protocol

import (
	"errors"
	"math/big"
	"sort"
	"strings"
)

type TimingTrial struct {
	BaselineNanos  int64 `json:"baselineNanos"`
	CandidateNanos int64 `json:"candidateNanos"`
	BaselineFirst  bool  `json:"baselineFirst"`
	QualityPassed  bool  `json:"qualityPassed"`
}

type TimingSummary struct {
	MedianRatio    string `json:"medianRatio"`
	LowerRatio     string `json:"lowerRatio"`
	UpperRatio     string `json:"upperRatio"`
	ConfidencePPM  int    `json:"confidencePpm"`
	AcceptedTrials int    `json:"acceptedTrials"`
	Outcome        string `json:"outcome"` // valid, inconclusive
	PracticalGain  bool   `json:"practicalGain"`
}

func ValidateMeasurementPolicy(p MeasurementPolicy) error {
	if p.Estimator != "paired-median-ratio" || p.Order != "alternating" || p.Warmups < 1 || p.Warmups > 100 || p.Repetitions < 9 || p.Repetitions > 255 || p.ConfidencePPM < 900000 || p.ConfidencePPM > 999999 {
		return errors.New("unsupported or insufficient paired measurement schedule")
	}
	if ValidatePath(strings.TrimSuffix(p.BaselinePath, "/")) != nil || !strings.HasPrefix(p.BaselinePath, "baseline/") || validateStageArgv(p.BaselineBuild) != nil || validateStageArgv(p.BaselineRun) != nil {
		return errors.New("frozen baseline source path and commands required")
	}
	if !ValidDigest(p.BaselineDigest) || !boundedText(p.TimerBoundary) || !boundedText(p.Population) {
		return errors.New("baseline bytes, timer boundary and target population must be frozen")
	}
	width, err := decimal(p.MaxRelativeWidth)
	if err != nil || width.Sign() <= 0 || width.Cmp(big.NewRat(1, 1)) > 0 {
		return errors.New("relative interval width must be in (0,1]")
	}
	gain, err := decimal(p.MinimumSpeedup)
	if err != nil || gain.Cmp(big.NewRat(1, 1)) <= 0 {
		return errors.New("practical speedup must exceed one")
	}
	if medianIntervalIndex(p.Repetitions, p.ConfidencePPM) < 1 {
		return errors.New("too few trials for the requested median confidence")
	}
	return nil
}

// medianIntervalIndex finds the largest k for which [X_(k), X_(n-k+1)]
// covers the population median with at least the requested probability under
// the independent identically distributed trial assumption. Contract review
// must justify that assumption; serial drift is not repaired by this formula.
// Ties make this interval conservative. All binomial comparisons are exact.
func medianIntervalIndex(n, confidencePPM int) int {
	denom := new(big.Int).Lsh(big.NewInt(1), uint(n))
	target := new(big.Int).Mul(denom, big.NewInt(int64(1000000-confidencePPM)))
	sum := new(big.Int)
	answer := 0
	for k := 1; k <= (n+1)/2; k++ {
		sum.Add(sum, new(big.Int).Binomial(int64(n), int64(k-1)))
		if new(big.Int).Mul(sum, big.NewInt(2000000)).Cmp(target) <= 0 {
			answer = k
		} else {
			break
		}
	}
	return answer
}

func rationalString(r *big.Rat) string { return r.Num().String() + "/" + r.Denom().String() }

// SummarizeTimings never drops failed trials or selects the best subset. Every
// scheduled measured pair must pass the locked correctness/quality checks.
// Timing provenance and warmup execution are verified by the executor separately.
func SummarizeTimings(trials []TimingTrial, p MeasurementPolicy) (TimingSummary, error) {
	var result TimingSummary
	if err := ValidateMeasurementPolicy(p); err != nil {
		return result, err
	}
	if len(trials) != p.Repetitions {
		return result, errors.New("missing or extra measured trials")
	}
	ratios := make([]*big.Rat, len(trials))
	for i, t := range trials {
		if !t.QualityPassed {
			return result, errors.New("quality failure cannot be excluded from measured trials")
		}
		if t.BaselineNanos <= 0 || t.CandidateNanos <= 0 || t.BaselineNanos > 86400e9 || t.CandidateNanos > 86400e9 {
			return result, errors.New("invalid trusted duration")
		}
		if t.BaselineFirst != (i%2 == 0) {
			return result, errors.New("measured order differs from locked alternation")
		}
		ratios[i] = big.NewRat(t.BaselineNanos, t.CandidateNanos)
	}
	sort.Slice(ratios, func(i, j int) bool { return ratios[i].Cmp(ratios[j]) < 0 })
	n := len(ratios)
	k := medianIntervalIndex(n, p.ConfidencePPM)
	median := new(big.Rat).Set(ratios[n/2])
	if n%2 == 0 {
		median.Add(median, ratios[n/2-1])
		median.Quo(median, big.NewRat(2, 1))
	}
	lower, upper := ratios[k-1], ratios[n-k]
	width := new(big.Rat).Quo(new(big.Rat).Sub(upper, lower), median)
	maxWidth, _ := decimal(p.MaxRelativeWidth)
	minimum, _ := decimal(p.MinimumSpeedup)
	result = TimingSummary{MedianRatio: rationalString(median), LowerRatio: rationalString(lower), UpperRatio: rationalString(upper), ConfidencePPM: p.ConfidencePPM, AcceptedTrials: n, Outcome: "valid", PracticalGain: lower.Cmp(minimum) >= 0}
	if width.Cmp(maxWidth) > 0 {
		result.Outcome = "inconclusive"
		result.PracticalGain = false
	}
	return result, nil
}

// TimingEvidence is populated by the root guest broker after the reviewed
// checker acknowledges quality. Candidate output never supplies these times.
type TimingEvidence struct {
	BaselineDigest string        `json:"baselineDigest"`
	Warmups        []TimingTrial `json:"warmups"`
	Trials         []TimingTrial `json:"trials"`
	Summary        TimingSummary `json:"summary"`
}

func ValidateTimingEvidence(e TimingEvidence, p MeasurementPolicy) (TimingSummary, error) {
	if e.BaselineDigest != p.BaselineDigest || len(e.Warmups) != p.Warmups {
		return TimingSummary{}, errors.New("baseline or warmup schedule mismatch")
	}
	for i, t := range e.Warmups {
		if !t.QualityPassed || t.BaselineNanos <= 0 || t.CandidateNanos <= 0 || t.BaselineNanos > 86400e9 || t.CandidateNanos > 86400e9 || t.BaselineFirst != (i%2 == 0) {
			return TimingSummary{}, errors.New("warmup failed or order changed")
		}
	}
	summary, err := SummarizeTimings(e.Trials, p)
	if err != nil {
		return summary, err
	}
	if summary != e.Summary {
		return summary, errors.New("reported timing summary differs from raw paired evidence")
	}
	return summary, nil
}

// ConfirmMeasuredRuns uses independent timing intervals, not scalar equality or
// a widened deterministic tolerance. A disagreement is inconclusive and does
// not pause unrelated challenge submissions.
func ConfirmMeasuredRuns(a, b RunReceipt, m Manifest) (string, error) {
	if m.APIVersion != ManifestV2 || m.Evaluation == nil || m.Evaluation.Measurement == nil || m.Evaluation.Mode != "performance" || a.Outcome != "valid" || b.Outcome != "valid" {
		return "", errors.New("valid measured repeats required")
	}
	for _, r := range []RunReceipt{a, b} {
		if err := ValidateRunMeasurementEvidence(r, m); err != nil {
			return "", err
		}
	}
	x, y := a.ValidatorResult.Timing.Summary, b.ValidatorResult.Timing.Summary
	xl, _ := MeasurementNumber(x.LowerRatio, "rational")
	xu, _ := MeasurementNumber(x.UpperRatio, "rational")
	yl, _ := MeasurementNumber(y.LowerRatio, "rational")
	yu, _ := MeasurementNumber(y.UpperRatio, "rational")
	if xl.Cmp(yu) > 0 || yl.Cmp(xu) > 0 {
		return "", errors.New("fresh timing intervals do not overlap")
	}
	if c, _ := CompareTicks(a.ScoreTicks, b.ScoreTicks); c < 0 {
		return a.ScoreTicks, nil
	}
	return b.ScoreTicks, nil
}

func ValidateMeasuredFixture(a, b RunReceipt, m Manifest, f Fixture) (string, error) {
	if a.Outcome != f.ExpectedOutcome || b.Outcome != a.Outcome || a.ArtifactDigest != b.ArtifactDigest {
		return "", errors.New("fixture outcome or artifact changed")
	}
	if a.Outcome != "valid" {
		return "", nil
	}
	score, err := ConfirmMeasuredRuns(a, b, m)
	if err != nil {
		return "", err
	}
	if f.Name == "baseline" {
		if a.ArtifactDigest != m.Evaluation.Measurement.BaselineDigest {
			return "", errors.New("baseline fixture is not the frozen reference program")
		}
		for _, r := range []RunReceipt{a, b} {
			s := r.ValidatorResult.Timing.Summary
			lo, _ := MeasurementNumber(s.LowerRatio, "rational")
			hi, _ := MeasurementNumber(s.UpperRatio, "rational")
			if lo.Cmp(big.NewRat(1, 1)) > 0 || hi.Cmp(big.NewRat(1, 1)) < 0 {
				return "", errors.New("baseline self-comparison shows measurement bias")
			}
		}
	}
	return score, nil
}
