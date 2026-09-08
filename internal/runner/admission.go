package runner

import (
	"crypto"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

// The API grants 15-minute leases. Reserve another five minutes for claim
// transport, clock skew and result delivery before any trusted input expires.
const AdmissionSafetyWindow = 20 * time.Minute

var ErrAdmissionMaintenance = errors.New("runner trust renewal required before new claims")

// AdmissionWindow is an immutable cache of verified signed trust deadlines.
// It authorizes no execution; Run/Prepare still check the full host controls.
type AdmissionWindow struct {
	verified        bool
	safetyWindow    time.Duration
	validFrom       time.Time
	hostExpires     time.Time
	advisoryExpires time.Time
}

// LoadAdmissionWindow verifies signatures, exact config/file bindings and base
// runtime advisory coverage once. It deliberately preserves expired deadlines so
// serve can replay existing signed results and automatically renew authorization.
func LoadAdmissionWindow(config Config, keys map[string]crypto.PublicKey) (AdmissionWindow, error) {
	var window AdmissionWindow
	payload, err := protocol.Verify(config.Attestation, keys)
	if err != nil {
		return window, err
	}
	var host HostAttestation
	if err := protocol.DecodeStrict(payload, &host); err != nil {
		return window, err
	}
	binding, err := ConfigBindingDigest(config)
	if err != nil || binding != host.ConfigDigest || host.HostID != config.HostID || host.HostGroup != config.HostGroup || host.PhysicalHostID == "" || !host.ExclusivePhysicalHost || !host.EgressPolicyVerified || host.ExecutionProfileDigest != config.ExecutionProfileDigest || host.RunnerEpoch != config.RunnerEpoch {
		return window, errors.New("admission trust does not bind the configured host")
	}
	if err := validateCapabilitiesBinding(config, host); err != nil {
		return window, err
	}
	for _, file := range []PinnedFile{config.AdvisoryKeys, config.AdvisorySnapshot} {
		if err := verifyPinned(file); err != nil {
			return window, err
		}
	}
	advisoryKeys, err := ReadPublicKeys(config.AdvisoryKeys.Path)
	if err != nil {
		return window, err
	}
	data, err := os.ReadFile(config.AdvisorySnapshot.Path)
	if err != nil {
		return window, err
	}
	var envelope protocol.Envelope
	if err := protocol.DecodeStrict(data, &envelope); err != nil {
		return window, err
	}
	payload, err = protocol.Verify(envelope, advisoryKeys)
	if err != nil {
		return window, errors.New("admission advisory lacks a trusted signature")
	}
	var advisory AdvisorySnapshot
	if err := protocol.DecodeStrict(payload, &advisory); err != nil {
		return window, err
	}
	inventory, err := ReadRuntimeInventory(config.RuntimeInventory)
	if err != nil {
		return window, err
	}
	if inventory.APIVersion != "science-ladder-runtime-inventory/v1" || len(inventory.Packages) == 0 || inventory.RuntimeImageDigest != config.RuntimeImageDigest {
		return window, errors.New("admission inventory does not bind the configured runtime")
	}
	// Check signed structure, provenance and exact coverage at generation time;
	// Check/Purposes below apply the distinct authorization and preflight deadlines.
	if _, status := ScanAdvisoriesForDomains(inventory.Packages, advisory, advisory.GeneratedAt, separatedToolchain(config, inventory)); status != "pass" {
		return window, errors.New("admission advisory coverage is not approved")
	}
	safety := AdmissionSafetyWindow
	if config.Capabilities != nil && config.Capabilities.MaxJobSeconds > 0 {
		if config.Capabilities.MaxJobSeconds < 900 || config.Capabilities.MaxJobSeconds > int(protocol.MaximumJobLease/time.Second) {
			return window, errors.New("invalid enrolled maximum job lease")
		}
		safety = time.Duration(config.Capabilities.MaxJobSeconds)*time.Second + 5*time.Minute
	}
	window = AdmissionWindow{verified: true, safetyWindow: safety, validFrom: advisory.GeneratedAt.Add(-time.Minute), hostExpires: host.ExpiresAt, advisoryExpires: advisory.ExpiresAt}
	return window, nil
}

func (w AdmissionWindow) margin() time.Duration {
	if w.safetyWindow > AdmissionSafetyWindow {
		return w.safetyWindow
	}
	return AdmissionSafetyWindow
}

func (w AdmissionWindow) Check(now time.Time) error {
	if !w.verified {
		return fmt.Errorf("%w: unverified trust window", ErrAdmissionMaintenance)
	}
	if now.Before(w.validFrom) {
		return fmt.Errorf("%w: signed advisory is not yet valid", ErrAdmissionMaintenance)
	}
	if !w.hostExpires.After(now.Add(w.margin())) {
		return fmt.Errorf("%w: host authorization expires at %s; requires more than %s remaining", ErrAdmissionMaintenance, w.hostExpires.UTC().Format(time.RFC3339), w.margin())
	}
	return nil
}

// Purpose filtering keeps a stale admission scan from shutting down already
// locked challenges. Preflight still requires current evidence, and its scanner
// independently checks freshness. The API intersects this list with enrollment.
func (w AdmissionWindow) Purposes(now time.Time) ([]string, error) {
	if err := w.Check(now); err != nil {
		return nil, err
	}
	purposes := []string{"artifact_prepare", "submission", "confirmation"}
	if w.advisoryExpires.After(now.Add(w.margin())) {
		purposes = append(purposes, "preflight")
	}
	return purposes, nil
}

func (w AdmissionWindow) NeedsRenewal(now time.Time) bool {
	renewBefore := 6 * time.Hour
	if w.margin()+time.Hour > renewBefore {
		renewBefore = w.margin() + time.Hour
	}
	return !w.verified || !w.hostExpires.After(now.Add(renewBefore))
}

func (w AdmissionWindow) HostExpiresAt() time.Time { return w.hostExpires }

// RenewAuthorization accepts only a bounded lease for the exact existing
// operator-approved enrollment. A renewed lease is not fresh vulnerability or
// hardware-measurement evidence. All existing pins and Run/Prepare checks remain.
func RenewAuthorization(config Config, keys map[string]crypto.PublicKey, envelope protocol.Envelope, now time.Time) (Config, AdmissionWindow, error) {
	decode := func(e protocol.Envelope) (HostAttestation, error) {
		var a HostAttestation
		payload, err := protocol.Verify(e, keys)
		if err == nil {
			err = protocol.DecodeStrict(payload, &a)
		}
		return a, err
	}
	old, err := decode(config.Attestation)
	if err != nil {
		return config, AdmissionWindow{}, err
	}
	fresh, err := decode(envelope)
	if err != nil {
		return config, AdmissionWindow{}, err
	}
	if !fresh.ExpiresAt.After(now.Add(AdmissionSafetyWindow)) || fresh.ExpiresAt.After(now.Add(25*time.Hour)) {
		return config, AdmissionWindow{}, errors.New("renewed host authorization has an invalid lease duration")
	}
	old.ExpiresAt = fresh.ExpiresAt
	oldDigest, oldErr := protocol.Digest(old)
	freshDigest, freshErr := protocol.Digest(fresh)
	if oldErr != nil || freshErr != nil || oldDigest != freshDigest {
		return config, AdmissionWindow{}, errors.New("renewal changes the approved host enrollment")
	}
	proposed := config
	proposed.Attestation = envelope
	window, err := LoadAdmissionWindow(proposed, keys)
	if err != nil {
		return config, AdmissionWindow{}, err
	}
	return proposed, window, nil
}
