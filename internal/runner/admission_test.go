package runner

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func admissionFixture(t *testing.T, now time.Time, hostRemaining, advisoryRemaining time.Duration) (Config, map[string]crypto.PublicKey) {
	t.Helper()
	inventory, advisory := scannerFixture(now)
	advisory.ExpiresAt = now.Add(advisoryRemaining)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pin := func(name string, value any) PinnedFile {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		filename := filepath.Join(dir, name)
		if err := os.WriteFile(filename, data, 0600); err != nil {
			t.Fatal(err)
		}
		return PinnedFile{Path: filename, Digest: protocol.DigestBytes(data)}
	}
	envelope, err := protocol.Sign("reviewed-policy", key, advisory)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	config := Config{HostID: "fixture-host", HostGroup: "fixture-group", RunnerEpoch: "1", ExecutionProfileDigest: protocol.DigestBytes([]byte("profile")), RuntimeImageDigest: inventory.RuntimeImageDigest, RuntimeInventory: pin("inventory.json", inventory), AdvisorySnapshot: pin("advisory.json", envelope), AdvisoryKeys: pin("advisory-keys.json", map[string]string{"reviewed-policy": string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))})}
	binding, _ := ConfigBindingDigest(config)
	config.Attestation, err = protocol.Sign("platform", key, HostAttestation{HostID: config.HostID, HostGroup: config.HostGroup, PhysicalHostID: "fixture-physical", ExclusivePhysicalHost: true, EgressPolicyVerified: true, RunnerEpoch: config.RunnerEpoch, ExecutionProfileDigest: config.ExecutionProfileDigest, ConfigDigest: binding, ExpiresAt: now.Add(hostRemaining)})
	if err != nil {
		t.Fatal(err)
	}
	return config, map[string]crypto.PublicKey{"platform": &key.PublicKey}
}

func TestAdmissionSignedDeadlineWindows(t *testing.T) {
	now := time.Date(2026, 9, 4, 23, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name           string
		host, advisory time.Duration
		allowed        bool
	}{{"current", time.Hour, time.Hour, true}, {"expired host", -time.Second, time.Hour, false}, {"expired advisory", time.Hour, -time.Second, true}, {"near host", 19 * time.Minute, time.Hour, false}, {"near advisory", time.Hour, 19 * time.Minute, true}, {"exact advisory boundary", time.Hour, AdmissionSafetyWindow, true}} {
		t.Run(test.name, func(t *testing.T) {
			config, keys := admissionFixture(t, now, test.host, test.advisory)
			window, err := LoadAdmissionWindow(config, keys)
			if err != nil {
				t.Fatal(err)
			}
			err = window.Check(now)
			if (err == nil) != test.allowed || err != nil && !errors.Is(err, ErrAdmissionMaintenance) {
				t.Fatalf("allowed=%v, error=%v", test.allowed, err)
			}
			purposes, purposeErr := window.Purposes(now)
			if test.allowed {
				if purposeErr != nil || len(purposes) < 3 || slices.Contains(purposes, "preflight") != (test.advisory > AdmissionSafetyWindow) {
					t.Fatalf("wrong work eligibility: %v %v", purposes, purposeErr)
				}
			} else if purposeErr == nil || len(purposes) != 0 {
				t.Fatal("expired host retained claim purposes")
			}
		})
	}
	config, keys := admissionFixture(t, now, time.Hour, time.Hour)
	window, err := LoadAdmissionWindow(config, keys)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(window.Check(now.Add(41*time.Minute)), ErrAdmissionMaintenance) {
		t.Fatal("cached admission did not become maintenance as time advanced")
	}
	if !errors.Is(window.Check(now.Add(-2*time.Hour)), ErrAdmissionMaintenance) {
		t.Fatal("clock rollback admitted a future policy")
	}
	if !errors.Is((AdmissionWindow{}).Check(now), ErrAdmissionMaintenance) {
		t.Fatal("zero trust cache admitted work")
	}
}

func TestRenewedAuthorizationSurvivesOldDeadlineWithoutRefreshingScan(t *testing.T) {
	now := time.Now().UTC()
	config, keys := admissionFixture(t, now, time.Hour, time.Hour)
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	keys["renewal-platform"] = &key.PublicKey
	payload, _ := protocol.Verify(config.Attestation, keys)
	var original HostAttestation
	if err := protocol.DecodeStrict(payload, &original); err != nil {
		t.Fatal(err)
	}
	baseBytes, _ := os.ReadFile(config.AdvisorySnapshot.Path)
	renewAt := now.Add(2 * time.Hour)
	fresh := original
	fresh.ExpiresAt = renewAt.Add(24 * time.Hour)
	envelope, _ := protocol.Sign("renewal-platform", key, fresh)
	updated, window, err := RenewAuthorization(config, keys, envelope, renewAt)
	if err != nil {
		t.Fatal(err)
	}
	purposes, err := window.Purposes(renewAt)
	if err != nil || !slices.Equal(purposes, []string{"artifact_prepare", "submission", "confirmation"}) {
		t.Fatalf("renewed old challenge cannot continue: %v %v", purposes, err)
	}
	before, _ := ConfigBindingDigest(config)
	after, _ := ConfigBindingDigest(updated)
	bytesAfter, _ := os.ReadFile(config.AdvisorySnapshot.Path)
	if before != after || string(baseBytes) != string(bytesAfter) {
		t.Fatal("renewal altered original evidence or configuration")
	}
	if window.NeedsRenewal(renewAt.Add(17*time.Hour)) || !window.NeedsRenewal(renewAt.Add(18*time.Hour)) {
		t.Fatal("renewal margin is not six hours")
	}
	if window.Check(renewAt.Add(24*time.Hour)) == nil {
		t.Fatal("fresh lease did not expire")
	}
	for name, alter := range map[string]func(*HostAttestation){
		"other host":          func(a *HostAttestation) { a.HostID = "other" },
		"other physical host": func(a *HostAttestation) { a.PhysicalHostID = "other" },
		"changed egress":      func(a *HostAttestation) { a.EgressPolicyVerified = false },
		"different config":    func(a *HostAttestation) { a.ConfigDigest = protocol.DigestBytes([]byte("changed")) },
		"overlong lease":      func(a *HostAttestation) { a.ExpiresAt = renewAt.Add(26 * time.Hour) },
		"expired lease":       func(a *HostAttestation) { a.ExpiresAt = renewAt },
	} {
		t.Run(name, func(t *testing.T) {
			a := fresh
			alter(&a)
			forged, _ := protocol.Sign("renewal-platform", key, a)
			if _, _, err := RenewAuthorization(config, keys, forged, renewAt); err == nil {
				t.Fatal("accepted changed/invalid enrollment")
			}
		})
	}
	envelope.Signatures = nil
	if _, _, err := RenewAuthorization(config, keys, envelope, renewAt); err == nil {
		t.Fatal("accepted invalid signature")
	}
}

func TestAdmissionRejectsTamperedTrust(t *testing.T) {
	now := time.Now().UTC()
	config, keys := admissionFixture(t, now, time.Hour, time.Hour)
	config.HostID = "other-host"
	if _, err := LoadAdmissionWindow(config, keys); err == nil {
		t.Fatal("configuration not bound by attestation was admitted")
	}
	config, keys = admissionFixture(t, now, time.Hour, time.Hour)
	if err := os.WriteFile(config.AdvisorySnapshot.Path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAdmissionWindow(config, keys); err == nil {
		t.Fatal("mutated signed policy was admitted")
	}
}
