package protocol

import (
	"errors"
	"math/big"
	"regexp"
	"strings"
)

var decimalPattern = regexp.MustCompile(`^[+-]?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
var ticksPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

func decimal(value string) (*big.Rat, error) {
	if !decimalPattern.MatchString(value) {
		return nil, errors.New("score must be a canonical finite decimal string")
	}
	abs := strings.TrimLeft(value, "+-")
	digits := strings.ReplaceAll(abs, ".", "")
	significant := strings.TrimLeft(digits, "0")
	if len(significant) > 100 {
		return nil, errors.New("decimal exceeds 100 significant digits")
	}
	if point := strings.IndexByte(abs, '.'); point >= 0 && len(abs)-point-1 > 50 {
		return nil, errors.New("decimal exceeds 50 fractional digits")
	}
	r, ok := new(big.Rat).SetString(value)
	if !ok || r.Sign() == 0 && strings.HasPrefix(value, "-") {
		return nil, errors.New("invalid decimal or negative zero")
	}
	return r, nil
}

func ParseTicks(value string) (*big.Int, error) {
	if len(value) > 160 || !ticksPattern.MatchString(value) || value == "-0" {
		return nil, errors.New("invalid integer ticks")
	}
	v, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return nil, errors.New("invalid ticks")
	}
	return v, nil
}
func CompareTicks(a, b string) (int, error) {
	x, err := ParseTicks(a)
	if err != nil {
		return 0, err
	}
	y, err := ParseTicks(b)
	if err != nil {
		return 0, err
	}
	return x.Cmp(y), nil
}

func NormalizeScore(score string, metric Metric) (string, error) {
	v, err := decimal(score)
	if err != nil {
		return "", err
	}
	q, err := decimal(metric.Quantum)
	if err != nil || q.Sign() <= 0 {
		return "", errors.New("quantum must be a positive decimal")
	}
	if metric.Direction != "maximize" && metric.Direction != "minimize" {
		return "", errors.New("unknown metric direction")
	}
	ratio := new(big.Rat).Quo(v, q)
	ticks, remainder := new(big.Int), new(big.Int)
	ticks.QuoRem(ratio.Num(), ratio.Denom(), remainder)
	if remainder.Sign() != 0 {
		if metric.Direction == "maximize" && ratio.Sign() < 0 {
			ticks.Sub(ticks, big.NewInt(1))
		}
		if metric.Direction == "minimize" && ratio.Sign() > 0 {
			ticks.Add(ticks, big.NewInt(1))
		}
	}
	result := ticks.String()
	if _, err := ParseTicks(result); err != nil {
		return "", err
	}
	if metric.DomainMinTicks != "" {
		min, err := ParseTicks(metric.DomainMinTicks)
		if err != nil || ticks.Cmp(min) < 0 {
			return "", errors.New("score below declared domain")
		}
	}
	if metric.DomainMaxTicks != "" {
		max, err := ParseTicks(metric.DomainMaxTicks)
		if err != nil || ticks.Cmp(max) > 0 {
			return "", errors.New("score above declared domain")
		}
	}
	return result, nil
}

func Qualifies(score, threshold, direction string) (bool, error) {
	cmp, err := CompareTicks(score, threshold)
	if err != nil {
		return false, err
	}
	switch direction {
	case "maximize":
		return cmp >= 0, nil
	case "minimize":
		return cmp <= 0, nil
	}
	return false, errors.New("unknown direction")
}

func ConfirmScores(a, b string, metric Metric) (string, error) {
	x, err := ParseTicks(a)
	if err != nil {
		return "", err
	}
	y, err := ParseTicks(b)
	if err != nil {
		return "", err
	}
	t, err := ParseTicks(metric.ToleranceTicks)
	if err != nil || t.Sign() < 0 {
		return "", errors.New("invalid tolerance")
	}
	delta := new(big.Int).Abs(new(big.Int).Sub(x, y))
	if delta.Cmp(t) > 0 {
		return "", errors.New("nondeterministic scores")
	}
	if metric.Direction == "maximize" {
		if x.Cmp(y) < 0 {
			return a, nil
		}
		return b, nil
	}
	if metric.Direction == "minimize" {
		if x.Cmp(y) > 0 {
			return a, nil
		}
		return b, nil
	}
	return "", errors.New("unknown direction")
}

func ValidateResult(data []byte, manifest Manifest) (ValidatorResult, string, error) {
	var result ValidatorResult
	if len(data) > 65536 {
		return result, "", errors.New("result exceeds 64 KiB")
	}
	if err := DecodeStrict(data, &result); err != nil {
		return result, "", err
	}
	if result.APIVersion != APIVersion || result.Kind != "ValidatorResult" {
		return result, "", errors.New("unsupported result version/kind")
	}
	if len(result.Gates) != len(manifest.HardGates) {
		return result, "", errors.New("missing or unknown hard gate")
	}
	for _, gate := range manifest.HardGates {
		if _, ok := result.Gates[gate]; !ok {
			return result, "", errors.New("missing hard gate")
		}
	}
	ticks, err := NormalizeScore(result.Score, manifest.Metric)
	return result, ticks, err
}
