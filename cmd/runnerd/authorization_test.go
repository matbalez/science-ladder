package main

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/internal/runner"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

func TestAuthorizationRefreshRetainsLeaseAndRecoversAfterExpiry(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	keys := map[string]crypto.PublicKey{"test-platform": &key.PublicKey}
	dir := t.TempDir()
	pin := func(name string, value any) runner.PinnedFile {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return runner.PinnedFile{Path: path, Digest: protocol.DigestBytes(data)}
	}
	p := runner.PackageCoordinate{Ecosystem: "PyPI", Name: "test-only-dependency", Version: "1.0"}
	inv := runner.RuntimeInventory{APIVersion: "science-ladder-runtime-inventory/v1", RuntimeImageDigest: protocol.DigestBytes([]byte("synthetic image")), Packages: []runner.PackageCoordinate{p}}
	source := "https://api.osv.dev/v1/query"
	snapshot := runner.AdvisorySnapshot{APIVersion: "science-ladder-advisories/v1", Kind: "AdvisorySnapshot", ID: "synthetic-test-only", GeneratedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), Sources: []runner.AdvisorySource{{URL: source, FetchedAt: now.Add(-time.Hour), ContentDigest: protocol.DigestBytes([]byte("synthetic coverage"))}}, Coverage: []runner.AdvisoryCoverage{{Package: p, Status: "complete", SourceURL: source, Advisories: []runner.Advisory{}}}}
	signedScan, _ := protocol.Sign("test-platform", key, snapshot)
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	config := runner.Config{HostID: "synthetic-host", HostGroup: "synthetic-group", RunnerEpoch: "1", ExecutionProfileDigest: protocol.DigestBytes([]byte("synthetic-profile")), RuntimeImageDigest: inv.RuntimeImageDigest, RuntimeInventory: pin("inventory.json", inv), AdvisorySnapshot: pin("scan.json", signedScan), AdvisoryKeys: pin("keys.json", map[string]string{"test-platform": string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))})}
	binding, _ := runner.ConfigBindingDigest(config)
	host := runner.HostAttestation{HostID: config.HostID, PhysicalHostID: "synthetic-physical-host", HostGroup: config.HostGroup, ExpiresAt: now.Add(time.Hour), ExclusivePhysicalHost: true, EgressPolicyVerified: true, RunnerEpoch: "1", ExecutionProfileDigest: config.ExecutionProfileDigest, ConfigDigest: binding}
	config.Attestation, _ = protocol.Sign("test-platform", key, host)
	window, err := runner.LoadAdmissionWindow(config, keys)
	if err != nil {
		t.Fatal(err)
	}
	a := hostAuthorization{config: config, window: window, keys: keys}
	recoveryTime := now.Add(2 * time.Hour)
	host.ExpiresAt = recoveryTime.Add(24 * time.Hour)
	renewedEnvelope, _ := protocol.Sign("test-platform", key, host)
	var available atomic.Bool
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body map[string]string
		if r.Method != "POST" || r.URL.Path != "/internal/v1/runner/authorization/renew" || json.NewDecoder(r.Body).Decode(&body) != nil || len(body) != 1 || body["configDigest"] != binding {
			t.Error("renewal request changed or omitted exact configuration binding")
		}
		if !available.Load() {
			w.WriteHeader(503)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"attestation": renewedEnvelope})
	}))
	defer server.Close()
	if refreshed, err := a.refresh(context.Background(), server.Client(), server.URL, now); err == nil || refreshed {
		t.Fatal("unavailable authority appeared renewed")
	}
	if a.window.Check(now) != nil {
		t.Fatal("transient failure discarded valid lease")
	}
	if _, err := a.refresh(context.Background(), server.Client(), server.URL, now.Add(30*time.Second)); err != nil || calls.Load() != 1 {
		t.Fatal("renewal retry ignored backoff")
	}
	if a.window.Check(recoveryTime) == nil {
		t.Fatal("expired authorization allowed claims")
	}
	available.Store(true)
	if refreshed, err := a.refresh(context.Background(), server.Client(), server.URL, recoveryTime); err != nil || !refreshed {
		t.Fatalf("automatic recovery failed: %v", err)
	}
	purposes, err := a.window.Purposes(recoveryTime)
	if err != nil || !slices.Equal(purposes, []string{"artifact_prepare", "submission", "confirmation"}) {
		t.Fatalf("wrong recovered purposes: %v %v", purposes, err)
	}
	if _, err := a.refresh(context.Background(), server.Client(), server.URL, recoveryTime.Add(time.Hour)); err != nil || calls.Load() != 2 {
		t.Fatal("valid long lease renewed unnecessarily")
	}
	data, _ := os.ReadFile(config.AdvisorySnapshot.Path)
	if protocol.DigestBytes(data) != config.AdvisorySnapshot.Digest || a.config.AdvisorySnapshot != config.AdvisorySnapshot {
		t.Fatal("host renewal modified advisory evidence")
	}
}
