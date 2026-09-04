package audit

import (
	"bufio"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

const MaxBundleBytes = 8 << 20

type witnessedRecord struct {
	Bundle     Bundle             `json:"bundle"`
	Signature  *protocol.Envelope `json:"signature,omitempty"`
	Historical bool               `json:"historical,omitempty"`
}

// Witness serializes observations, pins its predecessor durably, and will never
// countersign two competing successors. Its journal belongs to its own operator.
type Witness struct {
	mu          sync.Mutex
	journal     *os.File
	syncJournal func() error
	keyID       string
	key         crypto.Signer
	history     History
	last        *Checkpoint
	lastRecord  *witnessedRecord
	poisoned    bool
}

func OpenWitness(path, keyID string, key crypto.Signer, verifiedHistory History) (*Witness, error) {
	return openWitness(path, keyID, key, verifiedHistory, syncDirectory)
}

func openWitness(path, keyID string, key crypto.Signer, verifiedHistory History, syncDir func(string) error) (*Witness, error) {
	if key == nil {
		return nil, errors.New("witness signing key required")
	}
	now := time.Now()
	if verifiedHistory.WitnessOperatorsAt(now)[keyID] == "" {
		return nil, errors.New("active registered witness identity required")
	}
	expected := verifiedHistory.KeysAt("audit-witness", now)[keyID]
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
	w := &Witness{journal: f, syncJournal: f.Sync, keyID: keyID, key: key, history: verifiedHistory}
	// Persist both the journal inode and every directory entry that may have
	// just been created. Syncing only file contents can lose the journal's name
	// after a crash, allowing a restarted witness to forget a released vote.
	if e = f.Sync(); e != nil {
		w.Close()
		return nil, errors.New("cannot persist witness journal initialization")
	}
	directory, e := filepath.Abs(filepath.Dir(path))
	if e != nil {
		w.Close()
		return nil, e
	}
	for {
		if e = syncDir(directory); e != nil {
			w.Close()
			return nil, errors.New("cannot persist witness journal directory chain")
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}
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
		if record.Historical {
			if record.Signature != nil {
				w.Close()
				return nil, errors.New("historical observation cannot claim a witness signature")
			}
		} else {
			if record.Signature == nil || len(record.Signature.Signatures) != 1 {
				w.Close()
				return nil, errors.New("witness journal signature missing")
			}
			id := record.Signature.Signatures[0].KeyID
			payload, e := protocol.Verify(*record.Signature, verifiedHistory.KeysAt("audit-witness", cp.IssuedAt))
			if e != nil || verifiedHistory.WitnessOperatorsAt(cp.IssuedAt)[id] == "" || Hash(payload) != mustDigest(cp) || record.Signature.Payload != record.Bundle.Checkpoint.Payload {
				w.Close()
				return nil, errors.New("witness journal signature does not match checkpoint")
			}
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

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
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

// Observe retains a verified successor before releasing a signature. Checkpoints
// preceding this witness's delegation are authenticated and retained without a
// new vote so replacements can catch up. In that case the returned envelope is
// empty and err is nil; Latest reports an empty Witnesses list.
func (w *Witness) Observe(bundle Bundle, now time.Time) (protocol.Envelope, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.poisoned || w.journal == nil {
		return protocol.Envelope{}, errors.New("witness journal unavailable; signing paused")
	}
	// Exact retries return the same persisted signature, including after restart.
	if w.lastRecord != nil && bundle.Checkpoint.Payload == w.lastRecord.Bundle.Checkpoint.Payload {
		if !sameAuditEvents(bundle.Events, w.lastRecord.Bundle.Events) {
			return protocol.Envelope{}, errors.New("checkpoint retry changed its interval events")
		}
		if _, e := protocol.Verify(bundle.Checkpoint, w.history.KeysAt("audit-checkpoint", w.last.IssuedAt)); e != nil {
			return protocol.Envelope{}, e
		}
		if w.lastRecord.Signature == nil {
			return protocol.Envelope{}, nil
		}
		return *w.lastRecord.Signature, nil
	}
	cp, e := w.validate(bundle, now)
	if e != nil {
		return protocol.Envelope{}, e
	}
	eligible := func(at time.Time) bool {
		return w.history.KeysAt("audit-witness", at)[w.keyID] != nil && w.history.WitnessOperatorsAt(at)[w.keyID] != ""
	}
	if !eligible(now) {
		return protocol.Envelope{}, errors.New("witness delegation is not active at observation time")
	}
	historical := !eligible(cp.IssuedAt)
	if historical && !cp.IssuedAt.Before(now) {
		return protocol.Envelope{}, errors.New("unsigned catch-up is permitted only for historical checkpoints")
	}
	// Other witnesses are verified by the control plane; this journal retains only
	// the platform statement and independently observed events, not untrusted extras.
	bundle.Witnesses = nil
	record := witnessedRecord{Bundle: bundle, Historical: historical}
	if !historical {
		// Bound the complete durable record before asking the key to sign. A
		// P-256 ASN.1 signature is at most 72 bytes (96 base64 characters).
		estimate := record
		estimate.Signature = &protocol.Envelope{PayloadType: protocol.PayloadType, Payload: bundle.Checkpoint.Payload, Signatures: []protocol.Signature{{KeyID: w.keyID, Sig: strings.Repeat("A", 96)}}}
		if _, e = encodeWitnessRecord(estimate); e != nil {
			return protocol.Envelope{}, e
		}
		signature, err := protocol.Sign(w.keyID, w.key, cp)
		if err != nil {
			return protocol.Envelope{}, err
		}
		record.Signature = &signature
	}
	encoded, e := encodeWitnessRecord(record)
	if e != nil {
		return protocol.Envelope{}, e
	}
	n, e := w.journal.Write(encoded)
	if e != nil || n != len(encoded) {
		w.poisoned = true
		return protocol.Envelope{}, errors.New("durable witness journal write failed; signing paused")
	}
	if e = w.syncJournal(); e != nil {
		w.poisoned = true
		return protocol.Envelope{}, errors.New("durable witness journal sync failed; signing paused")
	}
	w.last = &cp
	w.lastRecord = &record
	if record.Signature == nil {
		return protocol.Envelope{}, nil
	}
	return *record.Signature, nil
}

func encodeWitnessRecord(record witnessedRecord) ([]byte, error) {
	encoded, err := json.Marshal(record)
	if err != nil || len(encoded)+1 > MaxBundleBytes {
		return nil, errors.New("witness journal entry exceeds bound")
	}
	return append(encoded, '\n'), nil
}

// Whole intervals may exceed CanonicalJSON's per-document limit. Compare each
// event's validated canonical digest rather than ignoring a whole-array error.
func sameAuditEvents(a, b []Event) bool {
	if len(a) != len(b) {
		return false
	}
	for i, left := range a {
		right := b[i]
		if left.Sequence != right.Sequence || left.Kind != right.Kind || left.PreviousDigest != right.PreviousDigest || left.Digest != right.Digest {
			return false
		}
		leftDigest, err := EventDigest(left.PreviousDigest, left.Kind, left.Payload)
		if err != nil || leftDigest != left.Digest {
			return false
		}
		rightDigest, err := EventDigest(right.PreviousDigest, right.Kind, right.Payload)
		if err != nil || rightDigest != right.Digest {
			return false
		}
	}
	return true
}

func (w *Witness) Latest() (Bundle, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.lastRecord == nil {
		return Bundle{}, false
	}
	b := w.lastRecord.Bundle
	b.Witnesses = nil
	if w.lastRecord.Signature != nil {
		b.Witnesses = []protocol.Envelope{*w.lastRecord.Signature}
	}
	return b, true
}
