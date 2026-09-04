package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/matbalez/science-ladder/internal/runner"
)

func TestClaimNeverPrecedesAdmission(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "POST" || r.URL.Path != "/internal/v1/runner/jobs/claim" {
			t.Error("unexpected runner request")
		}
		fmt.Fprint(w, `{"job":null}`)
	}))
	defer server.Close()
	for _, reason := range []string{"expired host", "expired advisory", "near expiry", "unverified cache"} {
		err := fmt.Errorf("%w: %s", runner.ErrAdmissionMaintenance, reason)
		_, got := claimIfAdmitted(context.Background(), server.Client(), server.URL, func() error { return err })
		if !errors.Is(got, runner.ErrAdmissionMaintenance) || calls.Load() != 0 {
			t.Fatalf("%s requested a lease before rejecting: %v, requests=%d", reason, got, calls.Load())
		}
	}
	if _, err := claimIfAdmitted(context.Background(), server.Client(), server.URL, func() error { return nil }); err != nil || calls.Load() != 1 {
		t.Fatalf("current admitted worker did not claim once: %v, requests=%d", err, calls.Load())
	}
	// Crossing the safety boundary after one completed job must not claim another.
	if _, err := claimIfAdmitted(context.Background(), server.Client(), server.URL, func() error { return runner.ErrAdmissionMaintenance }); !errors.Is(err, runner.ErrAdmissionMaintenance) || calls.Load() != 1 {
		t.Fatal("retry bypassed maintenance admission")
	}
}
