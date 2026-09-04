// Package audit verifies the append-only public checkpoint and delegation chain.
package audit

import (
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

const Genesis = "genesis"
const MaxEvents = 10000

type Event struct {
	Sequence       string          `json:"sequence"`
	Kind           string          `json:"kind"`
	Payload        json.RawMessage `json:"payload"`
	PreviousDigest string          `json:"previousDigest"`
	Digest         string          `json:"digest"`
}

type Checkpoint struct {
	APIVersion               string    `json:"apiVersion"`
	Kind                     string    `json:"kind"`
	LogID                    string    `json:"logId"`
	FromSequence             string    `json:"fromSequence"`
	ToSequence               string    `json:"toSequence"`
	PreviousCheckpointDigest string    `json:"previousCheckpointDigest"`
	PreviousEventDigest      string    `json:"previousEventDigest"`
	LastEventDigest          string    `json:"lastEventDigest"`
	MerkleRoot               string    `json:"merkleRoot"`
	IssuedAt                 time.Time `json:"issuedAt"`
}

// A Bundle carries one immutable checkpoint payload, its platform signature,
// independent signatures over exactly that payload, and its full event interval.
type Bundle struct {
	Checkpoint protocol.Envelope   `json:"checkpoint"`
	Witnesses  []protocol.Envelope `json:"witnesses"`
	Events     []Event             `json:"events"`
}

func Hash(data []byte) string { h := sha256.Sum256(data); return "sha256:" + hex.EncodeToString(h[:]) }
func CanonicalDigest(value any) (string, error) {
	b, e := json.Marshal(value)
	if e != nil {
		return "", e
	}
	b, e = protocol.CanonicalJSON(b)
	if e != nil {
		return "", e
	}
	return Hash(b), nil
}
func EventDigest(previous, kind string, payload []byte) (string, error) {
	if !validDigest(previous, true) || kind == "" || len(kind) > 128 || strings.ContainsAny(kind, "\r\n") {
		return "", errors.New("invalid audit chain fields")
	}
	b, err := protocol.CanonicalJSON(payload)
	if err != nil {
		return "", err
	}
	return Hash(append([]byte(previous+"\n"+kind+"\n"), b...)), nil
}
func validDigest(s string, genesis bool) bool {
	if genesis && s == Genesis {
		return true
	}
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	b, e := hex.DecodeString(s[7:])
	return e == nil && len(b) == 32 && strings.ToLower(s) == s
}
func sequence(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 || strconv.FormatInt(n, 10) != s {
		return 0, errors.New("sequence must be a canonical nonnegative integer string")
	}
	return n, nil
}

// MerkleRoot uses domain-separated leaf (0x00) and node (0x01) hashes with
// largest-power-of-two splitting. There is no duplicate-last-leaf ambiguity.
func MerkleRoot(digests []string) (string, error) {
	if len(digests) > MaxEvents {
		return "", errors.New("checkpoint event bound exceeded")
	}
	leaves := make([][]byte, len(digests))
	for i, d := range digests {
		if !validDigest(d, false) {
			return "", errors.New("invalid Merkle leaf digest")
		}
		b, _ := hex.DecodeString(d[7:])
		h := sha256.Sum256(append([]byte{0}, b...))
		leaves[i] = h[:]
	}
	var tree func([][]byte) []byte
	tree = func(nodes [][]byte) []byte {
		if len(nodes) == 0 {
			h := sha256.Sum256(nil)
			return h[:]
		}
		if len(nodes) == 1 {
			return nodes[0]
		}
		k := 1
		for k*2 < len(nodes) {
			k *= 2
		}
		h := sha256.Sum256(append(append([]byte{1}, tree(nodes[:k])...), tree(nodes[k:])...))
		return h[:]
	}
	return "sha256:" + hex.EncodeToString(tree(leaves)), nil
}

func Build(logID string, events []Event, previous *Checkpoint, at time.Time) (Checkpoint, error) {
	if logID == "" || len(logID) > 128 || at.IsZero() {
		return Checkpoint{}, errors.New("log identity and timestamp required")
	}
	from, last := int64(1), int64(0)
	priorCheckpoint, priorEvent := Genesis, Genesis
	if previous != nil {
		var err error
		last, err = sequence(previous.ToSequence)
		if err != nil {
			return Checkpoint{}, err
		}
		if last == int64(^uint64(0)>>1) {
			return Checkpoint{}, errors.New("sequence exhausted")
		}
		from = last + 1
		priorEvent = previous.LastEventDigest
		priorCheckpoint, err = CanonicalDigest(previous)
		if err != nil {
			return Checkpoint{}, err
		}
		if previous.LogID != logID || at.Before(previous.IssuedAt) {
			return Checkpoint{}, errors.New("checkpoint changed log identity or regressed time")
		}
	}
	cp := Checkpoint{APIVersion: protocol.APIVersion, Kind: "AuditCheckpoint", LogID: logID, FromSequence: strconv.FormatInt(from, 10), ToSequence: strconv.FormatInt(last, 10), PreviousCheckpointDigest: priorCheckpoint, PreviousEventDigest: priorEvent, LastEventDigest: priorEvent, IssuedAt: at.UTC()}
	digests := make([]string, 0, len(events))
	for i, e := range events {
		n, err := sequence(e.Sequence)
		if err != nil || n != from+int64(i) || n < from {
			return Checkpoint{}, errors.New("audit sequence gap or overflow")
		}
		d, err := EventDigest(cp.LastEventDigest, e.Kind, e.Payload)
		if err != nil || e.PreviousDigest != cp.LastEventDigest || d != e.Digest {
			return Checkpoint{}, errors.New("broken audit event hash chain")
		}
		cp.LastEventDigest = e.Digest
		cp.ToSequence = e.Sequence
		digests = append(digests, e.Digest)
	}
	var err error
	cp.MerkleRoot, err = MerkleRoot(digests)
	return cp, err
}

// VerifyCheckpoint refuses gaps, forks, changed interval content, and future
// checkpoints. The caller pins previous from its durable cache, not this bundle.
func VerifyCheckpoint(envelope protocol.Envelope, keys map[string]crypto.PublicKey, events []Event, previous *Checkpoint, now time.Time) (Checkpoint, error) {
	b, err := protocol.Verify(envelope, keys)
	if err != nil {
		return Checkpoint{}, err
	}
	var cp Checkpoint
	if err = protocol.DecodeStrict(b, &cp); err != nil {
		return cp, err
	}
	if cp.APIVersion != protocol.APIVersion || cp.Kind != "AuditCheckpoint" || cp.IssuedAt.After(now.Add(5*time.Minute)) {
		return cp, errors.New("unsupported or future checkpoint")
	}
	expected, err := Build(cp.LogID, events, previous, cp.IssuedAt)
	if err != nil {
		return cp, err
	}
	a, _ := CanonicalDigest(cp)
	z, _ := CanonicalDigest(expected)
	if a != z {
		return cp, errors.New("checkpoint interval, chain link, or Merkle root differs from observed events")
	}
	return cp, nil
}

// VerifyQuorum counts distinct independently administered witness identities,
// not multiple signatures or keys controlled by one operator.
func VerifyQuorum(platform protocol.Envelope, witnesses []protocol.Envelope, keys map[string]crypto.PublicKey, operators map[string]string, quorum int) error {
	if quorum != 2 || len(operators) != 3 {
		return errors.New("MVP requires three witnesses and a 2-of-3 quorum")
	}
	allOperators, allKeys := map[string]bool{}, map[string]bool{}
	for id, admin := range operators {
		fingerprint, err := Fingerprint(keys[id])
		if admin == "" || err != nil || allOperators[admin] || allKeys[fingerprint] {
			return errors.New("witnesses must have distinct configured operators and keys")
		}
		allOperators[admin] = true
		allKeys[fingerprint] = true
	}
	seen := map[string]bool{}
	for _, envelope := range witnesses {
		if envelope.PayloadType != platform.PayloadType || envelope.Payload != platform.Payload {
			continue
		}
		for _, sig := range envelope.Signatures {
			key, ok := keys[sig.KeyID]
			if !ok {
				continue
			}
			single := protocol.Envelope{PayloadType: envelope.PayloadType, Payload: envelope.Payload, Signatures: []protocol.Signature{sig}}
			if _, err := protocol.Verify(single, map[string]crypto.PublicKey{sig.KeyID: key}); err == nil {
				if op := operators[sig.KeyID]; op != "" {
					seen[op] = true
				}
			}
		}
	}
	if len(seen) < quorum {
		return fmt.Errorf("witness quorum unavailable: %d of %d required", len(seen), quorum)
	}
	return nil
}
