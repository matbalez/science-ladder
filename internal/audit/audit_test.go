package audit

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	return k
}
func publicPEM(t *testing.T, k crypto.Signer) string {
	t.Helper()
	b, e := x509.MarshalPKIXPublicKey(k.Public())
	if e != nil {
		t.Fatal(e)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: b}))
}
func signed(t *testing.T, id string, key crypto.Signer, v any) protocol.Envelope {
	t.Helper()
	e, err := protocol.Sign(id, key, v)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func event(t *testing.T, n, previous, kind string) Event {
	t.Helper()
	p := json.RawMessage(`{"z":2,"a":1}`)
	d, e := EventDigest(previous, kind, p)
	if e != nil {
		t.Fatal(e)
	}
	return Event{n, kind, p, previous, d}
}
func TestCheckpointRejectsTamperGapForkAndTime(t *testing.T) {
	now := time.Now().UTC()
	key := newKey(t)
	keys := map[string]crypto.PublicKey{"online": key.Public()}
	genesis, e := Build("test", nil, nil, now)
	if e != nil {
		t.Fatal(e)
	}
	first := event(t, "1", Genesis, "accepted")
	second := event(t, "2", first.Digest, "adjudicated")
	events := []Event{first, second}
	cp, e := Build("test", events, &genesis, now)
	if e != nil {
		t.Fatal(e)
	}
	envelope := signed(t, "online", key, cp)
	if _, e = VerifyCheckpoint(envelope, keys, events, &genesis, now); e != nil {
		t.Fatal(e)
	}
	mutated := append([]Event(nil), events...)
	mutated[1].Payload = json.RawMessage(`{"a":4}`)
	if _, e = VerifyCheckpoint(envelope, keys, mutated, &genesis, now); e == nil {
		t.Fatal("accepted tampered event")
	}
	if _, e = VerifyCheckpoint(envelope, keys, events[1:], &genesis, now); e == nil {
		t.Fatal("accepted sequence gap")
	}
	fork := genesis
	fork.LogID = "another-log"
	if _, e = VerifyCheckpoint(envelope, keys, events, &fork, now); e == nil {
		t.Fatal("accepted alternate history")
	}
	cp.IssuedAt = now.Add(time.Hour)
	if _, e = VerifyCheckpoint(signed(t, "online", key, cp), keys, events, &genesis, now); e == nil {
		t.Fatal("accepted future checkpoint")
	}
	a, _ := EventDigest(Genesis, "accepted", []byte(`{"a":1,"z":2}`))
	if a != first.Digest {
		t.Fatal("event digest depends on JSON key order")
	}
}
func TestMerkleShapeSeparatesThreeAndFourLeaves(t *testing.T) {
	a, b, c := Hash([]byte("a")), Hash([]byte("b")), Hash([]byte("c"))
	x, _ := MerkleRoot([]string{a, b, c})
	y, _ := MerkleRoot([]string{a, b, c, c})
	if x == y {
		t.Fatal("Merkle trees of different size collide")
	}
}
func TestIndependentQuorum(t *testing.T) {
	platform := newKey(t)
	payload := map[string]string{"checkpoint": "same"}
	p := signed(t, "online", platform, payload)
	keys := map[string]crypto.PublicKey{}
	operators := map[string]string{}
	var envelopes []protocol.Envelope
	for _, id := range []string{"a", "b", "c"} {
		k := newKey(t)
		keys[id] = k.Public()
		operators[id] = id
		envelopes = append(envelopes, signed(t, id, k, payload))
	}
	if e := VerifyQuorum(p, envelopes[:2], keys, operators, 2); e != nil {
		t.Fatal(e)
	}
	if e := VerifyQuorum(p, []protocol.Envelope{envelopes[0], envelopes[0]}, keys, operators, 2); e == nil {
		t.Fatal("duplicate witness counted twice")
	}
	altered := envelopes[1]
	altered.Payload = signed(t, "online", platform, map[string]string{"checkpoint": "other"}).Payload
	if e := VerifyQuorum(p, []protocol.Envelope{envelopes[0], altered}, keys, operators, 2); e == nil {
		t.Fatal("counted signature over another checkpoint")
	}
	keys["b"] = keys["a"]
	if e := VerifyQuorum(p, envelopes, keys, operators, 2); e == nil {
		t.Fatal("same key represented as independent witnesses")
	}
}
func fixtureHistory(t *testing.T) (History, *ecdsa.PrivateKey, map[string]*ecdsa.PrivateKey) {
	t.Helper()
	now := time.Now().UTC()
	root := newKey(t)
	fp, _ := Fingerprint(root.Public())
	genesis, _ := Build("test", nil, nil, now)
	gd, _ := CanonicalDigest(genesis)
	h := History{APIVersion: protocol.APIVersion, Kind: "KeyHistory", RootFingerprint: fp, PreviousDigest: Genesis, GenesisCheckpointDigest: gd, IssuedAt: now, WitnessQuorum: 2, OutageGraceSeconds: 3600}
	keys := map[string]*ecdsa.PrivateKey{}
	for _, id := range []string{"online", "a", "b", "c"} {
		k := newKey(t)
		keys[id] = k
		roles := []string{"audit-witness"}
		if id == "online" {
			roles = []string{"audit-checkpoint", "control-plane-receipt"}
		} else {
			h.Witnesses = append(h.Witnesses, WitnessIdentity{id, id})
		}
		h.Delegations = append(h.Delegations, Delegation{KeyID: id, PublicKeyPEM: publicPEM(t, k), Roles: roles, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), Custody: "test-only"})
	}
	return h, root, keys
}
func TestHistoryRotationPreservesPriorValidity(t *testing.T) {
	h, root, _ := fixtureHistory(t)
	now := h.IssuedAt
	if _, e := VerifyHistory(signed(t, "root", root, h), "root", root.Public(), nil, now); e != nil {
		t.Fatal(e)
	}
	next := h
	next.Delegations = append([]Delegation(nil), h.Delegations...)
	next.PreviousDigest, _ = CanonicalDigest(h)
	next.IssuedAt = now.Add(time.Minute)
	revoked := now.Add(30 * time.Second)
	next.Delegations[0].RevokedAt = &revoked
	verified, e := VerifyHistory(signed(t, "root", root, next), "root", root.Public(), &h, next.IssuedAt)
	if e != nil {
		t.Fatal(e)
	}
	if verified.KeysAt("audit-checkpoint", now)["online"] == nil {
		t.Fatal("erased valid pre-revocation history")
	}
	if verified.KeysAt("audit-checkpoint", revoked)["online"] != nil {
		t.Fatal("revocation effective boundary not enforced")
	}
	next.Delegations[0].NotAfter = next.Delegations[0].NotAfter.Add(time.Hour)
	if _, e = VerifyHistory(signed(t, "root", root, next), "root", root.Public(), &h, next.IssuedAt); e == nil {
		t.Fatal("rewrote historical delegation")
	}
	if IntakeAllowed(time.Time{}, now) || IntakeAllowed(now.Add(-time.Hour-time.Nanosecond), now) || !IntakeAllowed(now.Add(-time.Hour), now) {
		t.Fatal("witness outage grace boundary broken")
	}
}

func TestWitnessDurabilityIdempotencyAndForkRejection(t *testing.T) {
	h, _, keys := fixtureHistory(t)
	path := filepath.Join(t.TempDir(), "journal.ndjson")
	w, err := OpenWitness(path, "a", keys["a"], h)
	if err != nil {
		t.Fatal(err)
	}
	genesis, _ := Build("test", nil, nil, h.IssuedAt)
	initial := Bundle{Checkpoint: signed(t, "online", keys["online"], genesis)}
	if _, err = w.Observe(initial, time.Now()); err != nil {
		t.Fatal(err)
	}
	if other, err := OpenWitness(path, "a", keys["a"], h); err == nil {
		other.Close()
		t.Fatal("allowed two processes to own the same witness journal")
	}
	first := event(t, "1", Genesis, "accepted")
	cp, _ := Build("test", []Event{first}, &genesis, h.IssuedAt.Add(time.Second))
	bundle := Bundle{Checkpoint: signed(t, "online", keys["online"], cp), Events: []Event{first}}
	receipt, err := w.Observe(bundle, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	w, err = OpenWitness(path, "a", keys["a"], h)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := w.Observe(bundle, time.Now())
	if err != nil || mustDigest(retry) != mustDigest(receipt) {
		t.Fatal("restart changed accepted witness signature", err)
	}
	alternative := event(t, "1", Genesis, "different-accepted")
	fork, _ := Build("test", []Event{alternative}, &genesis, cp.IssuedAt)
	if _, err = w.Observe(Bundle{Checkpoint: signed(t, "online", keys["online"], fork), Events: []Event{alternative}}, time.Now()); err == nil {
		t.Fatal("witness signed a fork after restart")
	}
	w.Close()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("{\"partial\"")
	f.Close()
	if w, err = OpenWitness(path, "a", keys["a"], h); err == nil {
		w.Close()
		t.Fatal("signed after incomplete durable journal write")
	}
}
