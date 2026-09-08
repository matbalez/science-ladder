//go:build linux

package runner

import (
	"context"
	"errors"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"path/filepath"
)

func (b *candidateBroker) prepareBaseline(m protocol.Manifest) error {
	if m.Evaluation == nil || m.Evaluation.Measurement == nil {
		return nil
	}
	policy := *m.Evaluation.Measurement
	source := filepath.Join("/sl/challenge", policy.BaselinePath)
	_, digest, err := protocol.CanonicalArtifact(source, m.Submission)
	if err != nil || digest != policy.BaselineDigest {
		return errors.New("frozen baseline artifact does not match measurement policy")
	}
	base := &candidateBroker{assets: b.assets, program: b.program, root: "/sl/baseline"}
	base.program.Build = policy.BaselineBuild
	base.program.Run = policy.BaselineRun
	if err := base.prepareRootFrom(m.Submission, source); err != nil {
		return err
	}
	b.baseline = base
	b.measurement = &policy
	return nil
}

func (b *candidateBroker) timingRequest(ctx context.Context, request BrokerRequest) BrokerResponse {
	invalid := BrokerResponse{Outcome: "invalid_request", ExitCode: -1}
	if b.measurement == nil || b.baseline == nil {
		return invalid
	}
	p := b.measurement
	if request.Action == "assess" {
		if !b.pending || request.QualityPassed == nil || len(request.Input) != 0 {
			return invalid
		}
		t := &b.pairs[len(b.pairs)-1]
		t.QualityPassed = *request.QualityPassed && b.pairOutputsValid
		b.pending = false
		return BrokerResponse{Outcome: "valid", ExitCode: 0}
	}
	if request.Action != "pair" || request.QualityPassed != nil || len(request.Input) > maxBrokerInput || !b.ready || !b.baseline.ready || b.pending || len(b.pairs) >= p.Warmups+p.Repetitions || b.runs+2 > b.program.MaxRuns {
		return invalid
	}
	index := len(b.pairs)
	warmup := index < p.Warmups
	relative := index
	if !warmup {
		relative -= p.Warmups
	}
	baselineFirst := relative%2 == 0
	pair := BrokerPair{Index: index, Warmup: warmup}
	var err error
	execute := func(base bool) {
		if err != nil {
			return
		}
		target := b
		if base {
			target = b.baseline
		}
		result, e := target.execute(ctx, target.program.Run, target.program.RunBudget, request.Input)
		err = e
		if base {
			pair.Baseline = result
		} else {
			pair.Candidate = result
		}
	}
	execute(baselineFirst)
	execute(!baselineFirst)
	if err != nil {
		b.fault = err
		return BrokerResponse{Outcome: "infrastructure_fault", ExitCode: -1}
	}
	b.runs += 2
	b.pending = true
	b.pairOutputsValid = pair.Baseline.Outcome == "valid" && pair.Candidate.Outcome == "valid"
	b.pairs = append(b.pairs, protocol.TimingTrial{BaselineNanos: pair.Baseline.DurationNanos, CandidateNanos: pair.Candidate.DurationNanos, BaselineFirst: baselineFirst})
	return BrokerResponse{Outcome: "valid", ExitCode: 0, Pair: &pair}
}

func (b *candidateBroker) timingEvidence() (*protocol.TimingEvidence, error) {
	if b.measurement == nil {
		return nil, nil
	}
	p := *b.measurement
	if b.pending || len(b.pairs) != p.Warmups+p.Repetitions {
		return nil, errors.New("paired measurement schedule is incomplete")
	}
	e := &protocol.TimingEvidence{BaselineDigest: p.BaselineDigest, Warmups: b.pairs[:p.Warmups], Trials: b.pairs[p.Warmups:]}
	summary, err := protocol.SummarizeTimings(e.Trials, p)
	if err != nil {
		return nil, err
	}
	e.Summary = summary
	if _, err := protocol.ValidateTimingEvidence(*e, p); err != nil {
		return nil, err
	}
	return e, nil
}
