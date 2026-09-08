package main

import (
	"context"
	"net/http"
	"net/url"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

type runnerClaim struct {
	Job         *protocol.Envelope `json:"job"`
	ResultToken string             `json:"resultToken"`
}

// Keep the time-dependent guard and claim together, so retry paths cannot send a
// lease request before checking the cached, authenticated admission window.
func claimIfAdmitted(ctx context.Context, client *http.Client, base string, check func() ([]string, error), profiles ...string) (runnerClaim, error) {
	var claim runnerClaim
	purposes, err := check()
	if err != nil {
		return claim, err
	}
	if len(purposes) == 0 {
		return claim, nil
	}
	endpoint := base + "/internal/v1/runner/jobs/claim"
	if len(profiles) > 0 {
		endpoint += "?profile=" + url.QueryEscape(profiles[0])
	}
	err = request(ctx, client, "POST", endpoint, map[string]any{"purposes": purposes}, &claim)
	return claim, err
}
