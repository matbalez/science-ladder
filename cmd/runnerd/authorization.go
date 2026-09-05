package main

import (
	"context"
	"crypto"
	"net/http"
	"time"

	"github.com/matbalez/science-ladder/internal/runner"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

type hostAuthorization struct {
	config      runner.Config
	window      runner.AdmissionWindow
	keys        map[string]crypto.PublicKey
	nextAttempt time.Time
}

// Refresh the existing enrollment before its lease expires, without interrupting
// work or replacing on-disk evidence. Failed refreshes retain the prior lease;
// admission blocks new claims if it expires, while this loop keeps retrying.
func (a *hostAuthorization) refresh(ctx context.Context, client *http.Client, base string, now time.Time) (bool, error) {
	if !a.nextAttempt.IsZero() && (now.Before(a.nextAttempt) || !a.window.NeedsRenewal(now)) {
		return false, nil
	}
	a.nextAttempt = now.Add(time.Minute)
	binding, err := runner.ConfigBindingDigest(a.config)
	if err != nil {
		return false, err
	}
	var response struct {
		Attestation protocol.Envelope `json:"attestation"`
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := request(ctx, client, "POST", base+"/internal/v1/runner/authorization/renew", map[string]string{"configDigest": binding}, &response); err != nil {
		return false, err
	}
	config, window, err := runner.RenewAuthorization(a.config, a.keys, response.Attestation, now)
	if err != nil {
		return false, err
	}
	a.config, a.window = config, window
	return true, nil
}
