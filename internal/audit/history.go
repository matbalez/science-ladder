package audit

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

type Delegation struct {
	KeyID        string     `json:"keyId"`
	PublicKeyPEM string     `json:"publicKeyPem"`
	Roles        []string   `json:"roles"`
	NotBefore    time.Time  `json:"notBefore"`
	NotAfter     time.Time  `json:"notAfter"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
	HostID       string     `json:"hostId,omitempty"`
	Custody      string     `json:"custody"`
}
type WitnessIdentity struct {
	KeyID    string `json:"keyId"`
	Operator string `json:"operator"`
}
type History struct {
	APIVersion              string            `json:"apiVersion"`
	Kind                    string            `json:"kind"`
	RootFingerprint         string            `json:"rootFingerprint"`
	PreviousDigest          string            `json:"previousDigest"`
	GenesisCheckpointDigest string            `json:"genesisCheckpointDigest"`
	IssuedAt                time.Time         `json:"issuedAt"`
	Delegations             []Delegation      `json:"delegations"`
	Witnesses               []WitnessIdentity `json:"witnesses"`
	WitnessQuorum           int               `json:"witnessQuorum"`
	OutageGraceSeconds      int               `json:"outageGraceSeconds"`
}

func ParsePublicKey(text string) (crypto.PublicKey, error) {
	b, rest := pem.Decode([]byte(text))
	if b == nil || b.Type != "PUBLIC KEY" || strings.TrimSpace(string(rest)) != "" {
		return nil, errors.New("exactly one public-key PEM is required")
	}
	key, e := x509.ParsePKIXPublicKey(b.Bytes)
	p, ok := key.(*ecdsa.PublicKey)
	if e != nil || !ok || p.Curve != elliptic.P256() {
		return nil, errors.New("P-256 public key required")
	}
	return p, nil
}
func Fingerprint(key crypto.PublicKey) (string, error) {
	p, ok := key.(*ecdsa.PublicKey)
	if !ok || p == nil || p.Curve != elliptic.P256() {
		return "", errors.New("P-256 public key required")
	}
	der, e := x509.MarshalPKIXPublicKey(key)
	if e != nil {
		return "", e
	}
	return Hash(der), nil
}

// VerifyHistory starts from an externally pinned root; fetching a new root from
// the same service being audited never creates trust. Rotations retain old keys.
func VerifyHistory(envelope protocol.Envelope, rootID string, root crypto.PublicKey, previous *History, now time.Time) (History, error) {
	b, e := protocol.Verify(envelope, map[string]crypto.PublicKey{rootID: root})
	if e != nil {
		return History{}, e
	}
	var h History
	if e = protocol.DecodeStrict(b, &h); e != nil {
		return h, e
	}
	fingerprint, e := Fingerprint(root)
	if e != nil {
		return h, e
	}
	if h.APIVersion != protocol.APIVersion || h.Kind != "KeyHistory" || h.RootFingerprint != fingerprint || h.IssuedAt.IsZero() || h.IssuedAt.After(now.Add(5*time.Minute)) || !validDigest(h.GenesisCheckpointDigest, false) || h.WitnessQuorum != 2 || h.OutageGraceSeconds != 3600 || len(h.Witnesses) != 3 {
		return h, errors.New("key history violates pinned MVP bootstrap policy")
	}
	if previous == nil {
		if h.PreviousDigest != Genesis {
			return h, errors.New("key-history bootstrap gap")
		}
	} else {
		d, _ := CanonicalDigest(previous)
		if h.PreviousDigest != d || h.IssuedAt.Before(previous.IssuedAt) || h.GenesisCheckpointDigest != previous.GenesisCheckpointDigest || h.RootFingerprint != previous.RootFingerprint {
			return h, errors.New("broken key-history chain")
		}
	}
	keys := map[string]crypto.PublicKey{}
	fingerprints := map[string]bool{fingerprint: true}
	roles := map[string]map[string]bool{}
	for _, d := range h.Delegations {
		key, err := ParsePublicKey(d.PublicKeyPEM)
		if err != nil {
			return h, err
		}
		f, _ := Fingerprint(key)
		if d.KeyID == "" || keys[d.KeyID] != nil || fingerprints[f] || !d.NotBefore.Before(d.NotAfter) || d.Custody == "" || len(d.Roles) == 0 {
			return h, errors.New("invalid or duplicate key delegation")
		}
		if d.RevokedAt != nil && (d.RevokedAt.Before(d.NotBefore) || d.RevokedAt.After(h.IssuedAt)) {
			return h, errors.New("invalid revocation effective time")
		}
		roles[d.KeyID] = map[string]bool{}
		for _, role := range d.Roles {
			switch role {
			case "control-plane-receipt", "audit-checkpoint", "validation-run", "audit-witness":
			default:
				return h, errors.New("unsupported delegated role")
			}
			if roles[d.KeyID][role] {
				return h, errors.New("duplicate key role")
			}
			roles[d.KeyID][role] = true
		}
		if roles[d.KeyID]["validation-run"] && (d.HostID == "" || len(d.Roles) != 1) {
			return h, errors.New("runner key must be bound to one host and validation-run role")
		}
		if roles[d.KeyID]["audit-witness"] && len(d.Roles) != 1 {
			return h, errors.New("witness keys must have only the witness role")
		}
		keys[d.KeyID] = key
		fingerprints[f] = true
	}
	operators := map[string]bool{}
	ids := map[string]bool{}
	for _, w := range h.Witnesses {
		if w.Operator == "" || operators[w.Operator] || ids[w.KeyID] || !roles[w.KeyID]["audit-witness"] {
			return h, errors.New("three distinct independently operated witness keys required")
		}
		operators[w.Operator] = true
		ids[w.KeyID] = true
	}
	if previous != nil {
		current := map[string]Delegation{}
		for _, d := range h.Delegations {
			current[d.KeyID] = d
		}
		for _, old := range previous.Delegations {
			d, ok := current[old.KeyID]
			if !ok {
				return h, errors.New("key history cannot discard historical delegations")
			}
			oldRevoked, dRevoked := old.RevokedAt, d.RevokedAt
			old.RevokedAt = nil
			d.RevokedAt = nil
			a, _ := CanonicalDigest(old)
			z, _ := CanonicalDigest(d)
			if a != z {
				return h, errors.New("existing delegation cannot be rewritten")
			}
			if oldRevoked != nil && (dRevoked == nil || !oldRevoked.Equal(*dRevoked)) {
				return h, errors.New("a revocation cannot be withdrawn or rewritten")
			}
		}
	}
	return h, nil
}
func (h History) KeysAt(role string, at time.Time) map[string]crypto.PublicKey {
	keys := map[string]crypto.PublicKey{}
	for _, d := range h.Delegations {
		if at.Before(d.NotBefore) || !at.Before(d.NotAfter) || (d.RevokedAt != nil && !at.Before(*d.RevokedAt)) {
			continue
		}
		for _, r := range d.Roles {
			if r == role {
				if key, e := ParsePublicKey(d.PublicKeyPEM); e == nil {
					keys[d.KeyID] = key
				}
			}
		}
	}
	return keys
}
func (h History) WitnessOperators() map[string]string {
	m := map[string]string{}
	for _, w := range h.Witnesses {
		m[w.KeyID] = w.Operator
	}
	return m
}

// IntakeAllowed applies the one-hour grace only after a verified quorum has
// existed. An unbootstrapped service never acquires a free one-hour intake window.
func IntakeAllowed(lastQuorum, now time.Time) bool {
	return !lastQuorum.IsZero() && !lastQuorum.After(now.Add(5*time.Minute)) && now.Sub(lastQuorum) <= time.Hour
}
