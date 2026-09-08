package protocol

import (
	"encoding/json"
	"errors"
	"math/big"
)

const FrontierClaimVersion = "science-ladder/frontier-claim/v1"

// FrontierClaim is untrusted local evidence, never a verification receipt.
type FrontierClaim struct {
	APIVersion     string              `json:"apiVersion"`
	VersionID      string              `json:"versionId"`
	LockDigest     string              `json:"lockDigest"`
	ArtifactDigest string              `json:"artifactDigest"`
	Mode           string              `json:"mode"`
	Result         *ValidatorResult    `json:"result,omitempty"`
	Qualification  *LocalQualification `json:"qualification,omitempty"`
}
type LocalQualification struct {
	ChecksPassed        bool   `json:"checksPassed"`
	ReportDigest        string `json:"reportDigest"`
	EstimatedScoreTicks string `json:"estimatedScoreTicks"`
	Reason              string `json:"reason"`
}

func AdmissionMode(m Manifest) string {
	if m.Suite.Visibility == "hidden" || m.Evaluation != nil && m.Evaluation.Mode == "performance" {
		return "qualification"
	}
	return "frontier"
}
func FrontierImprovement(score, prior string, m Metric) bool {
	x, e := ParseTicks(score)
	if e != nil {
		return false
	}
	y, e := ParseTicks(prior)
	if e != nil {
		return false
	}
	d, e := ParseTicks(m.MinimumDeltaTicks)
	if e != nil || d.Sign() <= 0 {
		return false
	}
	x.Sub(x, y)
	if m.Direction == "minimize" {
		x.Neg(x)
	} else if m.Direction != "maximize" {
		return false
	}
	return x.Cmp(d) >= 0
}
func (c FrontierClaim) Validate(m Manifest) (string, error) {
	if c.APIVersion != FrontierClaimVersion || c.VersionID == "" || !ValidDigest(c.LockDigest) || !ValidDigest(c.ArtifactDigest) || c.Mode != AdmissionMode(m) {
		return "", errors.New("claim version, digests or admission mode invalid")
	}
	if c.Mode == "qualification" {
		q := c.Qualification
		if c.Result != nil || q == nil || !q.ChecksPassed || !ValidDigest(q.ReportDigest) || len(q.Reason) < 20 || len(q.Reason) > 2000 {
			return "", errors.New("successful local qualification, report digest and measurement rationale required")
		}
		x, e := ParseTicks(q.EstimatedScoreTicks)
		if e != nil {
			return "", e
		}
		for _, b := range []struct {
			value string
			min   bool
		}{{m.Metric.DomainMinTicks, true}, {m.Metric.DomainMaxTicks, false}} {
			if b.value != "" {
				v, e := ParseTicks(b.value)
				if e != nil || b.min && x.Cmp(v) < 0 || !b.min && x.Cmp(v) > 0 {
					return "", errors.New("estimate outside metric domain")
				}
			}
		}
		return q.EstimatedScoreTicks, nil
	}
	if c.Result == nil || c.Qualification != nil {
		return "", errors.New("local validator result required")
	}
	data, e := json.Marshal(c.Result)
	if e != nil {
		return "", e
	}
	r, ticks, e := ValidateResult(data, m)
	if e != nil {
		return "", e
	}
	if ValidatorOutcome(r) != "valid" {
		return "", errors.New("all local correctness gates must pass")
	}
	return ticks, nil
}

// Ensure the public comparison never falls below the frozen reference.
func AdmissionFrontier(m Metric, public string) string {
	a, e := ParseTicks(public)
	if e != nil {
		return m.BaselineTicks
	}
	b, e := ParseTicks(m.BaselineTicks)
	if e != nil {
		return ""
	}
	cmp := new(big.Int).Sub(a, b).Sign()
	if m.Direction == "maximize" && cmp > 0 || m.Direction == "minimize" && cmp < 0 {
		return public
	}
	return m.BaselineTicks
}
