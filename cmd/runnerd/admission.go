package main

import (
	"context"
	"net/http"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

type runnerClaim struct {
	Job         *protocol.Envelope `json:"job"`
	ResultToken string             `json:"resultToken"`
}

// Keep the time-dependent guard and claim together, so retry paths cannot send a
// lease request before checking the cached, authenticated admission window.
func claimIfAdmitted(ctx context.Context, client *http.Client, base string, check func() error) (runnerClaim, error) {
	var claim runnerClaim
	if err := check(); err != nil {
		return claim, err
	}
	err := request(ctx, client, "POST", base+"/internal/v1/runner/jobs/claim", map[string]any{}, &claim)
	return claim, err
}
