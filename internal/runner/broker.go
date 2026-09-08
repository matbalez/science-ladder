package runner

import (
	"errors"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

const maxBrokerInput = 16 << 20

// Broker requests carry only data and a stage selector. The immutable manifest
// supplies every command and limit; submitted code cannot select a host command.
type BrokerRequest struct {
	Action string `json:"action"` // build, run
	Input  []byte `json:"input,omitempty"`
}

type BrokerResponse struct {
	Outcome       string `json:"outcome"`
	Stdout        []byte `json:"stdout,omitempty"`
	Stderr        []byte `json:"stderr,omitempty"`
	DurationNanos int64  `json:"durationNanos"`
	ExitCode      int    `json:"exitCode"`
}

func validateBrokerRequest(request BrokerRequest, built bool, runs int, program protocol.CandidateProgram) error {
	if len(request.Input) > maxBrokerInput {
		return errors.New("case input exceeds broker limit")
	}
	switch request.Action {
	case "build":
		if built || runs != 0 || len(request.Input) != 0 {
			return errors.New("build must occur exactly once before execution, without case inputs")
		}
	case "run":
		if !built || runs >= program.MaxRuns {
			return errors.New("candidate is unbuilt or its case budget is exhausted")
		}
	default:
		return errors.New("broker supports only the frozen build and run stages")
	}
	return nil
}
