package protocol

import (
	"strings"
	"testing"
)

func TestFrontierClaimGate(t *testing.T) {
	m := Manifest{APIVersion: APIVersion, Metric: Metric{Quantum: "1", Direction: "minimize", BaselineTicks: "17996", MinimumDeltaTicks: "4", DomainMinTicks: "0"}, HardGates: []string{"correct"}}
	c := FrontierClaim{APIVersion: FrontierClaimVersion, VersionID: "v", LockDigest: DigestBytes([]byte("lock")), ArtifactDigest: DigestBytes([]byte("artifact")), Mode: "frontier", Result: &ValidatorResult{APIVersion: APIVersion, Kind: "ValidatorResult", Score: "17992", Gates: map[string]bool{"correct": true}}}
	score, e := c.Validate(m)
	if e != nil || !FrontierImprovement(score, m.Metric.BaselineTicks, m.Metric) {
		t.Fatal(score, e)
	}
	for _, score := range []string{"17996", "17995", "17993", "NaN", "-0", strings.Repeat("9", 161)} {
		if FrontierImprovement(score, m.Metric.BaselineTicks, m.Metric) {
			t.Fatalf("admitted %s", score)
		}
	}
	c.Result.Gates["correct"] = false
	if _, e = c.Validate(m); e == nil {
		t.Fatal("failed gate admitted")
	}
	c.Result.Gates["correct"] = true
	c.Mode = "qualification"
	if _, e = c.Validate(m); e == nil {
		t.Fatal("client bypassed public checker")
	}
	m.Suite.Visibility = "hidden"
	c.Result = nil
	c.Qualification = &LocalQualification{ChecksPassed: true, ReportDigest: DigestBytes([]byte("report")), EstimatedScoreTicks: "17992", Reason: "Public suite passed; hidden suite needs the host."}
	if _, e = c.Validate(m); e != nil {
		t.Fatal(e)
	}
	c.Qualification.EstimatedScoreTicks = "-1"
	if _, e = c.Validate(m); e == nil {
		t.Fatal("out of domain estimate admitted")
	}
	m.Suite.Visibility = "public"
	m.Evaluation = &EvaluationContract{Mode: "performance"}
	if AdmissionMode(m) != "qualification" {
		t.Fatal("hardware measurement path missing")
	}
	if AdmissionFrontier(m.Metric, "18000") != "17996" || AdmissionFrontier(m.Metric, "17990") != "17990" {
		t.Fatal("reference/public comparison wrong")
	}
}
