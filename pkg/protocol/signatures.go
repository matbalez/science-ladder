package protocol

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// DSSE pre-authentication encoding binds the declared payload type and bytes.
func pae(payloadType string, payload []byte) []byte {
	return []byte(fmt.Sprintf("DSSEv1 %d %s %d %s", len(payloadType), payloadType, len(payload), payload))
}

func Sign(keyID string, key crypto.Signer, payload any) (Envelope, error) {
	if key == nil || !identifierPattern.MatchString(keyID) {
		return Envelope{}, errors.New("invalid signing key or identifier")
	}
	public, ok := key.Public().(*ecdsa.PublicKey)
	if !ok || public.Curve != elliptic.P256() {
		return Envelope{}, errors.New("P-256 signer required")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	data, err = CanonicalJSON(data)
	if err != nil {
		return Envelope{}, err
	}
	hash := sha256.Sum256(pae(PayloadType, data))
	sig, err := key.Sign(rand.Reader, hash[:], crypto.SHA256)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{PayloadType: PayloadType, Payload: base64.StdEncoding.EncodeToString(data), Signatures: []Signature{{KeyID: keyID, Sig: base64.StdEncoding.EncodeToString(sig)}}}, nil
}

// Verify requires one trusted, valid signature and rejects duplicate key IDs.
// Callers additionally validate the typed payload, job binding, time and purpose.
func Verify(envelope Envelope, keys map[string]crypto.PublicKey) ([]byte, error) {
	if envelope.PayloadType != PayloadType || len(envelope.Payload) > 2*MaxDocumentBytes || len(envelope.Signatures) < 1 || len(envelope.Signatures) > 8 {
		return nil, errors.New("invalid signed envelope")
	}
	payload, err := base64.StdEncoding.Strict().DecodeString(envelope.Payload)
	if err != nil {
		return nil, errors.New("invalid envelope payload encoding")
	}
	canonical, err := CanonicalJSON(payload)
	if err != nil || !bytes.Equal(payload, canonical) {
		return nil, errors.New("receipt payload must be canonical JSON")
	}
	seen := map[string]bool{}
	valid := false
	for _, signature := range envelope.Signatures {
		if seen[signature.KeyID] {
			return nil, errors.New("duplicate signature key")
		}
		seen[signature.KeyID] = true
		key, ok := keys[signature.KeyID]
		if !ok {
			continue
		}
		sig, err := base64.StdEncoding.Strict().DecodeString(signature.Sig)
		public, ok := key.(*ecdsa.PublicKey)
		hash := sha256.Sum256(pae(envelope.PayloadType, payload))
		if err == nil && ok && public.Curve == elliptic.P256() && ecdsa.VerifyASN1(public, hash[:], sig) {
			valid = true
		}
	}
	if !valid {
		return nil, errors.New("no valid trusted receipt signature")
	}
	return payload, nil
}
