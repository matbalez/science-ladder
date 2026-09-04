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
	}{{"current", time.Hour, time.Hour, true}, {"expired host", -time.Second, time.Hour, false}, {"expired advisory", time.Hour, -time.Second, false}, {"near host", 19 * time.Minute, time.Hour, false}, {"near advisory", time.Hour, 19 * time.Minute, false}, {"exact safety boundary", time.Hour, AdmissionSafetyWindow, false}} {
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
