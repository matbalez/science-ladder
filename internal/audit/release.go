package audit

import (
	"crypto"
	"errors"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/url"
	"time"
)

type ReleaseEvidence struct {
	Kind        string    `json:"kind"`
	Digest      string    `json:"digest"`
	URL         string    `json:"url"`
	Assessor    string    `json:"assessor"`
	CompletedAt time.Time `json:"completedAt"`
}
type ReleaseAttestation struct {
	APIVersion       string            `json:"apiVersion"`
	Kind             string            `json:"kind"`
	SourceCommit     string            `json:"sourceCommit"`
	IssuedAt         time.Time         `json:"issuedAt"`
	ExpiresAt        time.Time         `json:"expiresAt"`
	KeyHistoryDigest string            `json:"keyHistoryDigest"`
	Evidence         []ReleaseEvidence `json:"evidence"`
}

// VerifyRelease checks the operator's explicit signed release evidence, not the
// scientific quality of the external assessment. Automated tests cannot invent it.
func VerifyRelease(envelope protocol.Envelope, rootID string, root crypto.PublicKey, historyDigest, sourceCommit string, now time.Time) (ReleaseAttestation, error) {
	b, e := protocol.Verify(envelope, map[string]crypto.PublicKey{rootID: root})
	if e != nil {
		return ReleaseAttestation{}, e
	}
	var r ReleaseAttestation
	if e = protocol.DecodeStrict(b, &r); e != nil {
		return r, e
	}
	if r.APIVersion != protocol.APIVersion || r.Kind != "OfficialReleaseAttestation" || r.SourceCommit != sourceCommit || len(sourceCommit) != 40 || r.KeyHistoryDigest != historyDigest || r.IssuedAt.IsZero() || r.IssuedAt.After(now) || !now.Before(r.ExpiresAt) {
		return r, errors.New("release attestation does not authorize this source, key history, or time")
	}
	required := map[string]bool{"independent-security-review": false, "database-restore-drill": false, "key-rotation-drill": false, "runner-isolation-drill": false, "witness-outage-and-fork-drill": false, "external-invitation-pilot": false}
	for _, ev := range r.Evidence {
		if _, ok := required[ev.Kind]; !ok || required[ev.Kind] {
			return r, errors.New("unknown or duplicate release evidence")
		}
		u, err := url.Parse(ev.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || !validDigest(ev.Digest, false) || ev.Assessor == "" || ev.CompletedAt.IsZero() || ev.CompletedAt.After(r.IssuedAt) {
			return r, errors.New("incomplete release evidence")
		}
		required[ev.Kind] = true
	}
	for _, present := range required {
		if !present {
			return r, errors.New("required independent release evidence missing")
		}
	}
	return r, nil
}
