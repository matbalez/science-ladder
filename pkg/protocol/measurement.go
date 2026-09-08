package protocol

import (
	"errors"
	"math/big"
	"sort"
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
