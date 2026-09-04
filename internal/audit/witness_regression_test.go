package audit

import (
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

type countedWitnessSigner struct {
	crypto.Signer
	calls int
	fail  bool
}

func (s *countedWitnessSigner) Sign(random io.Reader, digest []byte, options crypto.SignerOpts) ([]byte, error) {
	s.calls++
	if s.fail {
		return nil, errors.New("test signing failure")
	}
	return s.Signer.Sign(random, digest, options)
}

func emptyWitnessSignature(t *testing.T, signature protocol.Envelope) {
	t.Helper()
	if signature.Payload != "" || signature.PayloadType != "" || len(signature.Signatures) != 0 {
		t.Fatal("returned a witness signature without a durable accepted vote")
	}
}

func TestWitnessDirectoryChainIsSyncedAndFailurePreventsOpening(t *testing.T) {
	history, _, keys := fixtureHistory(t)
	key := &countedWitnessSigner{Signer: keys["a"]}
	root := t.TempDir()
	path := filepath.Join(root, "new-parent", "new-child", "journal.ndjson")
	var directories []string
	w, err := openWitness(path, "a", key, history, func(directory string) error {
		directories = append(directories, directory)
		return syncDirectory(directory)
	})
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	for _, needed := range []string{filepath.Dir(path), filepath.Join(root, "new-parent"), root} {
		found := false
		for _, directory := range directories {
			found = found || needed == directory
		}
		if !found {
			t.Fatalf("new journal directory entry was not made durable: %s", needed)
		}
	}
	failedPath := filepath.Join(root, "failed-parent", "journal.ndjson")
	w, err = openWitness(failedPath, "a", key, history, func(string) error { return errors.New("injected directory sync failure") })
	if err == nil || w != nil || key.calls != 0 {
		t.Fatal("directory sync failure allowed a witness or a signing operation")
	}
	info, err := os.Stat(failedPath)
	if err != nil || info.Size() != 0 {
		t.Fatal("failed initialization must not contain a vote", err)
	}
}

func TestReplacementWitnessCatchesUpWithoutInventingHistoricalVotes(t *testing.T) {
	initial, history, _, keys := rotatedWitnessHistory(t)
	key := &countedWitnessSigner{Signer: keys["a2"]}
	path := filepath.Join(t.TempDir(), "journal.ndjson")
	w, err := OpenWitness(path, "a2", key, history)
	if err != nil {
		t.Fatal(err)
	}
	genesis, _ := Build("test", nil, nil, initial.IssuedAt)
	first := event(t, "1", Genesis, "accepted")
	oldCheckpoint, _ := Build("test", []Event{first}, &genesis, initial.IssuedAt.Add(time.Minute))
	for _, bundle := range []Bundle{{Checkpoint: signed(t, "online", keys["online"], genesis)}, {Checkpoint: signed(t, "online", keys["online"], oldCheckpoint), Events: []Event{first}}} {
		signature, err := w.Observe(bundle, time.Now())
		if err != nil {
			t.Fatal("historical catch-up failed", err)
		}
		emptyWitnessSignature(t, signature)
		latest, ok := w.Latest()
		if !ok || len(latest.Witnesses) != 0 || key.calls != 0 {
			t.Fatal("catch-up invented a historical witness vote")
		}
	}
	w.Close()
	w, err = OpenWitness(path, "a2", key, history)
	if err != nil {
		t.Fatal("unsigned authenticated history did not survive restart", err)
	}
	defer w.Close()
	latest, ok := w.Latest()
	if !ok || latest.Checkpoint.Payload != signed(t, "online", keys["online"], oldCheckpoint).Payload || len(latest.Witnesses) != 0 {
		t.Fatal("restart forgot historical predecessor or invented a signature")
	}
	// A valid platform signature does not authorize a competing predecessor.
	alternative := event(t, "1", Genesis, "different")
	fork, _ := Build("test", []Event{alternative}, &genesis, oldCheckpoint.IssuedAt)
	signature, err := w.Observe(Bundle{Checkpoint: signed(t, "online", keys["online"], fork), Events: []Event{alternative}}, time.Now())
	if err == nil {
		t.Fatal("historical catch-up allowed a fork after restart")
	}
	emptyWitnessSignature(t, signature)
	second := event(t, "2", first.Digest, "adjudicated")
	current, _ := Build("test", []Event{second}, &oldCheckpoint, history.IssuedAt.Add(time.Minute))
	bundle := Bundle{Checkpoint: signed(t, "online", keys["online"], current), Events: []Event{second}}
	signature, err = w.Observe(bundle, time.Now())
	if err != nil || key.calls != 1 {
		t.Fatal("replacement did not sign its first eligible checkpoint", err)
	}
	if _, err = protocol.Verify(signature, map[string]crypto.PublicKey{"a2": key.Public()}); err != nil {
		t.Fatal(err)
	}
	retry, err := w.Observe(bundle, time.Now())
	if err != nil || mustDigest(retry) != mustDigest(signature) || key.calls != 1 {
		t.Fatal("retry did not return the already durable vote", err)
	}
}

func TestUnsignedJournalRecordMustBeExplicitAndAuthenticated(t *testing.T) {
	initial, history, _, keys := rotatedWitnessHistory(t)
	path := filepath.Join(t.TempDir(), "journal.ndjson")
	w, err := OpenWitness(path, "a2", keys["a2"], history)
	if err != nil {
		t.Fatal(err)
	}
	genesis, _ := Build("test", nil, nil, initial.IssuedAt)
	if _, err = w.Observe(Bundle{Checkpoint: signed(t, "online", keys["online"], genesis)}, time.Now()); err != nil {
		t.Fatal(err)
	}
	w.Close()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record witnessedRecord
	if err = json.Unmarshal(b, &record); err != nil {
		t.Fatal(err)
	}
	if !record.Historical || record.Signature != nil {
		t.Fatal("historical observation was not stored explicitly")
	}
	for _, mutation := range []func(*witnessedRecord){
		func(r *witnessedRecord) { r.Historical = false },
		func(r *witnessedRecord) { r.Bundle.Checkpoint.Signatures[0].Sig = "invalid" },
	} {
		var changed witnessedRecord
		json.Unmarshal(b, &changed)
		mutation(&changed)
		encoded, err := encodeWitnessRecord(changed)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, encoded, 0600); err != nil {
			t.Fatal(err)
		}
		opened, err := OpenWitness(path, "a2", keys["a2"], history)
		if err == nil {
			opened.Close()
			t.Fatal("accepted missing vote without explicit history or unauthenticated history")
		}
	}
}

func TestOversizedWitnessRecordDoesNotInvokeSignerOrAdvanceJournal(t *testing.T) {
	history, _, keys := fixtureHistory(t)
	key := &countedWitnessSigner{Signer: keys["a"]}
	path := filepath.Join(t.TempDir(), "journal.ndjson")
	w, err := OpenWitness(path, "a", key, history)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	genesis, _ := Build("test", nil, nil, history.IssuedAt)
	if _, err = w.Observe(Bundle{Checkpoint: signed(t, "online", keys["online"], genesis)}, time.Now()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)
	fields := make([]string, 16)
	for i := range fields {
		fields[i] = strings.Repeat("x", 60<<10)
	}
	data, _ := json.Marshal(fields)
	previous := Genesis
	var events []Event
	for i := 1; i <= 9; i++ {
		digest, err := EventDigest(previous, "test-only", data)
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, Event{Sequence: fmt.Sprint(i), Kind: "test-only", Payload: data, PreviousDigest: previous, Digest: digest})
		previous = digest
	}
	checkpoint, err := Build("test", events, &genesis, history.IssuedAt)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := w.Observe(Bundle{Checkpoint: signed(t, "online", keys["online"], checkpoint), Events: events}, time.Now())
	if err == nil || key.calls != 1 {
		t.Fatal("oversized durable record reached the signing key")
	}
	emptyWitnessSignature(t, signature)
	after, _ := os.Stat(path)
	latest, _ := w.Latest()
	if after.Size() != before.Size() || latest.Checkpoint.Payload != signed(t, "online", keys["online"], genesis).Payload {
		t.Fatal("rejected oversized record advanced durable history")
	}
}

func TestSigningAndJournalFailuresNeverReturnVotes(t *testing.T) {
	history, _, keys := fixtureHistory(t)
	key := &countedWitnessSigner{Signer: keys["a"], fail: true}
	w, err := OpenWitness(filepath.Join(t.TempDir(), "journal.ndjson"), "a", key, history)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	genesis, _ := Build("test", nil, nil, history.IssuedAt)
	bundle := Bundle{Checkpoint: signed(t, "online", keys["online"], genesis)}
	signature, err := w.Observe(bundle, time.Now())
	if err == nil {
		t.Fatal("signing failure was hidden")
	}
	emptyWitnessSignature(t, signature)
	if _, ok := w.Latest(); ok {
		t.Fatal("failed signing advanced witness")
	}
	key.fail = false
	w.syncJournal = func() error { return errors.New("injected journal sync failure") }
	signature, err = w.Observe(bundle, time.Now())
	if err == nil || !w.poisoned {
		t.Fatal("journal failure did not pause signing")
	}
	emptyWitnessSignature(t, signature)
	calls := key.calls
	signature, err = w.Observe(bundle, time.Now())
	if err == nil || key.calls != calls {
		t.Fatal("poisoned journal allowed another signing attempt")
	}
	emptyWitnessSignature(t, signature)
}

func TestLargeCheckpointRetryRejectsChangedEvents(t *testing.T) {
	history, _, keys := fixtureHistory(t)
	w, err := OpenWitness(filepath.Join(t.TempDir(), "journal.ndjson"), "a", keys["a"], history)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	genesis, _ := Build("test", nil, nil, history.IssuedAt)
	if _, err = w.Observe(Bundle{Checkpoint: signed(t, "online", keys["online"], genesis)}, time.Now()); err != nil {
		t.Fatal(err)
	}
	fields := make([]string, 9)
	for i := range fields {
		fields[i] = strings.Repeat("x", 60<<10)
	}
	payload, _ := json.Marshal(fields)
	var events []Event
	previous := Genesis
	for i := 1; i <= 2; i++ {
		digest, err := EventDigest(previous, "test-only", payload)
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, Event{Sequence: fmt.Sprint(i), Kind: "test-only", Payload: payload, PreviousDigest: previous, Digest: digest})
		previous = digest
	}
	checkpoint, err := Build("test", events, &genesis, history.IssuedAt)
	if err != nil {
		t.Fatal(err)
	}
	bundle := Bundle{Checkpoint: signed(t, "online", keys["online"], checkpoint), Events: events}
	signature, err := w.Observe(bundle, time.Now())
	if err != nil {
		t.Fatal("valid multi-document event interval rejected", err)
	}
	retry, err := w.Observe(bundle, time.Now())
	if err != nil || mustDigest(retry) != mustDigest(signature) {
		t.Fatal("valid large interval retry was not idempotent", err)
	}
	changed := bundle
	changed.Events = append([]Event(nil), events...)
	fields[0] = strings.Repeat("y", 60<<10)
	changed.Events[0].Payload, _ = json.Marshal(fields)
	retry, err = w.Observe(changed, time.Now())
	if err == nil {
		t.Fatal("ignored canonicalization error and accepted changed large interval")
	}
	emptyWitnessSignature(t, retry)
}
