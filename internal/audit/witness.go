package audit

import (
	"bufio"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

const MaxBundleBytes = 8 << 20

type witnessedRecord struct {
	Bundle    Bundle            `json:"bundle"`
	Signature protocol.Envelope `json:"signature"`
}

// Witness serializes observations, pins its predecessor durably, and will never
// countersign two competing successors. Its journal belongs to its own operator.
type Witness struct {
	mu         sync.Mutex
	journal    *os.File
	keyID      string
	key        crypto.Signer
	history    History
	last       *Checkpoint
	lastRecord *witnessedRecord
	poisoned   bool
}

func OpenWitness(path, keyID string, key crypto.Signer, verifiedHistory History) (*Witness, error) {
	if key == nil {
		return nil, errors.New("witness signing key required")
	}
	expected := verifiedHistory.KeysAt("audit-witness", time.Now())[keyID]
	a, e := Fingerprint(expected)
	if e != nil {
		return nil, errors.New("active witness delegation required")
	}
	b, e := Fingerprint(key.Public())
	if e != nil || a != b {
		return nil, errors.New("witness key does not match root delegation")
	}
	if e = os.MkdirAll(filepath.Dir(path), 0o700); e != nil {
		return nil, e
	}
	if info, e := os.Lstat(path); e == nil && (!info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0) {
		return nil, errors.New("witness journal must be a private regular file")
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND|syscall.O_NOFOLLOW, 0o600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, errors.New("another witness process owns this journal")
	}
	w := &Witness{journal: f, keyID: keyID, key: key, history: verifiedHistory}
	info, e := f.Stat()
	if e != nil {
		w.Close()
		return nil, e
	}
	if info.Size() > 0 {
		tail := make([]byte, 1)
		if _, e = f.ReadAt(tail, info.Size()-1); e != nil || tail[0] != '\n' {
			w.Close()
			return nil, errors.New("partial witness journal tail; recover from externally retained checkpoint before restarting")
		}
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), MaxBundleBytes)
	for scanner.Scan() {
		var record witnessedRecord
		if e = protocol.DecodeStrictBounded(scanner.Bytes(), &record, MaxBundleBytes); e != nil {
			w.Close()
			return nil, errors.New("invalid witness journal record")
		}
		cp, e := w.validate(record.Bundle, time.Now())
		if e != nil {
			w.Close()
			return nil, e
		}
		payload, e := protocol.Verify(record.Signature, verifiedHistory.KeysAt("audit-witness", cp.IssuedAt))
		if e != nil || Hash(payload) != mustDigest(cp) || record.Signature.Payload != record.Bundle.Checkpoint.Payload {
			w.Close()
			return nil, errors.New("witness journal signature does not match checkpoint")
		}
		w.last = &cp
		w.lastRecord = &record
	}
	if e = scanner.Err(); e != nil {
		w.Close()
		return nil, e
	}
	return w, nil
}
func mustDigest(v any) string { d, _ := CanonicalDigest(v); return d }
func (w *Witness) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.journal == nil {
		return nil
	}
	e := w.journal.Close()
	w.journal = nil
	return e
}

func (w *Witness) validate(bundle Bundle, now time.Time) (Checkpoint, error) {
	payload, e := base64.StdEncoding.Strict().DecodeString(bundle.Checkpoint.Payload)
	if e != nil {
		return Checkpoint{}, e
	}
	var untrusted Checkpoint
	if e = protocol.DecodeStrict(payload, &untrusted); e != nil {
		return Checkpoint{}, e
	}
	cp, e := VerifyCheckpoint(bundle.Checkpoint, w.history.KeysAt("audit-checkpoint", untrusted.IssuedAt), bundle.Events, w.last, now)
	if e != nil {
		return cp, e
	}
	if w.last == nil && mustDigest(cp) != w.history.GenesisCheckpointDigest {
		return cp, errors.New("first checkpoint does not match externally pinned genesis")
	}
	return cp, nil
}

func (w *Witness) Observe(bundle Bundle, now time.Time) (protocol.Envelope, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.poisoned || w.journal == nil {
		return protocol.Envelope{}, errors.New("witness journal unavailable; signing paused")
	}
	// Exact retries return the same persisted signature, including after restart.
	if w.lastRecord != nil && bundle.Checkpoint.Payload == w.lastRecord.Bundle.Checkpoint.Payload {
		if mustDigest(bundle.Events) != mustDigest(w.lastRecord.Bundle.Events) {
			return protocol.Envelope{}, errors.New("checkpoint retry changed its interval events")
		}
		if _, e := protocol.Verify(bundle.Checkpoint, w.history.KeysAt("audit-checkpoint", w.last.IssuedAt)); e != nil {
			return protocol.Envelope{}, e
		}
		return w.lastRecord.Signature, nil
	}
	cp, e := w.validate(bundle, now)
	if e != nil {
		return protocol.Envelope{}, e
	}
	for _, at := range []time.Time{cp.IssuedAt, now} {
		if w.history.KeysAt("audit-witness", at)[w.keyID] == nil {
			return protocol.Envelope{}, errors.New("witness delegation is not active at checkpoint and signing time")
		}
	}
	signature, e := protocol.Sign(w.keyID, w.key, cp)
	if e != nil {
		return signature, e
	}
	// Other witnesses are verified by the control plane; this journal retains only
	// the platform statement and independently observed events, not untrusted extras.
	bundle.Witnesses = nil
	record := witnessedRecord{bundle, signature}
	encoded, e := json.Marshal(record)
	if e != nil || len(encoded)+1 > MaxBundleBytes {
		return signature, errors.New("witness journal entry exceeds bound")
	}
	encoded = append(encoded, '\n')
	n, e := w.journal.Write(encoded)
	if e != nil || n != len(encoded) {
		w.poisoned = true
		return protocol.Envelope{}, errors.New("durable witness journal write failed; signing paused")
	}
	if e = w.journal.Sync(); e != nil {
		w.poisoned = true
		return protocol.Envelope{}, errors.New("durable witness journal sync failed; signing paused")
	}
	w.last = &cp
	w.lastRecord = &record
	return signature, nil
}

func (w *Witness) Latest() (Bundle, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.lastRecord == nil {
		return Bundle{}, false
	}
	b := w.lastRecord.Bundle
	b.Witnesses = []protocol.Envelope{w.lastRecord.Signature}
	return b, true
}
