package main

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/internal/audit"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

func observerFixture(t *testing.T) (audit.History, map[string]*ecdsa.PrivateKey, []audit.Bundle) {
	t.Helper()
	keys := map[string]*ecdsa.PrivateKey{}
	for _, id := range []string{"root", "online", "a", "b", "c", "a2"} {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		keys[id] = key
	}
	now := time.Now().UTC()
	genesis, err := audit.Build("test-only", nil, nil, now.Add(-2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	genesisDigest, _ := audit.CanonicalDigest(genesis)
	fingerprint, _ := audit.Fingerprint(keys["root"].Public())
	history := audit.History{APIVersion: protocol.APIVersion, Kind: "KeyHistory", RootFingerprint: fingerprint, PreviousDigest: audit.Genesis, GenesisCheckpointDigest: genesisDigest, IssuedAt: genesis.IssuedAt, WitnessQuorum: 2, OutageGraceSeconds: 3600}
	delegation := func(id string, start time.Time) audit.Delegation {
		der, _ := x509.MarshalPKIXPublicKey(keys[id].Public())
		roles := []string{"audit-witness"}
		if id == "online" {
			roles = []string{"audit-checkpoint"}
		}
		return audit.Delegation{KeyID: id, PublicKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), Roles: roles, NotBefore: start, NotAfter: now.Add(time.Hour), Custody: "test-only"}
	}
	for _, id := range []string{"online", "a", "b", "c"} {
		history.Delegations = append(history.Delegations, delegation(id, now.Add(-3*time.Hour)))
		if id != "online" {
			history.Witnesses = append(history.Witnesses, audit.WitnessIdentity{KeyID: id, Operator: id})
		}
	}
	sign := func(id string, value any) protocol.Envelope {
		signature, err := protocol.Sign(id, keys[id], value)
		if err != nil {
			t.Fatal(err)
		}
		return signature
	}
	history, err = audit.VerifyHistory(sign("root", history), "root", keys["root"].Public(), nil, now)
	if err != nil {
		t.Fatal(err)
	}
	previous := history
	history.PreviousDigest, _ = audit.CanonicalDigest(previous)
	history.IssuedAt = now.Add(-time.Hour)
	history.Witnesses = append([]audit.WitnessIdentity(nil), previous.Witnesses...)
	history.Delegations = append(append([]audit.Delegation(nil), previous.Delegations...), delegation("a2", history.IssuedAt))
	for i := range history.Witnesses {
		if history.Witnesses[i].KeyID == "a" {
			history.Witnesses[i].KeyID = "a2"
		}
	}
	history, err = audit.VerifyHistory(sign("root", history), "root", keys["root"].Public(), &previous, now)
	if err != nil {
		t.Fatal(err)
	}
	old, err := audit.Build("test-only", nil, &genesis, now.Add(-90*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	current, err := audit.Build("test-only", nil, &old, now.Add(-30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return history, keys, []audit.Bundle{{Checkpoint: sign("online", genesis)}, {Checkpoint: sign("online", old)}, {Checkpoint: sign("online", current)}}
}

func TestObserverCatchesUpAcrossRotationAndRetriesOnlyItsDurableVote(t *testing.T) {
	history, keys, bundles := observerFixture(t)
	path := filepath.Join(t.TempDir(), "journal.ndjson")
	witness, err := audit.OpenWitness(path, "a2", keys["a2"], history)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { witness.Close() }()
	digests := make([]string, len(bundles))
	for i, bundle := range bundles {
		payload, err := protocol.Verify(bundle.Checkpoint, map[string]crypto.PublicKey{"online": keys["online"].Public()})
		if err != nil {
			t.Fatal(err)
		}
		digests[i] = audit.Hash(payload)
	}
	posts := 0
	var postMutex sync.Mutex
	postCount := func() int {
		postMutex.Lock()
		defer postMutex.Unlock()
		return posts
	}
	var firstVote protocol.Envelope
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/audit/checkpoints", func(out http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("limit") != "1" {
			t.Error("observer failed to bound its checkpoint page")
		}
		index := 0
		if after := request.URL.Query().Get("afterDigest"); after != "" {
			index = -1
			for i, digest := range digests {
				if after == digest {
					index = i + 1
				}
			}
			if index < 0 {
				t.Error("observer used an unknown predecessor cursor")
				http.Error(out, "unknown cursor", 400)
				return
			}
		} else if request.URL.Query().Get("after") != "0" {
			t.Error("observer did not bootstrap at genesis")
		}
		checkpoints := []any{}
		if index < len(bundles) {
			checkpoints = append(checkpoints, map[string]any{"id": "test-only", "digest": digests[index], "bundle": bundles[index]})
		}
		json.NewEncoder(out).Encode(map[string]any{"checkpoints": checkpoints, "deploymentMode": "controlled-demo"})
	})
	mux.HandleFunc("POST /v1/audit/checkpoints/{digest}/witness", func(out http.ResponseWriter, request *http.Request) {
		postMutex.Lock()
		defer postMutex.Unlock()
		posts++
		if request.PathValue("digest") != digests[2] {
			t.Error("observer attempted a vote on a checkpoint before its delegation")
		}
		var body struct {
			Envelope protocol.Envelope `json:"envelope"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		payload, err := protocol.Verify(body.Envelope, map[string]crypto.PublicKey{"a2": keys["a2"].Public()})
		if err != nil || audit.Hash(payload) != digests[2] {
			t.Error("observer did not publish its actual durable current signature", err)
		}
		if posts == 1 {
			firstVote = body.Envelope
			http.Error(out, "test-only lost acknowledgement", 503)
			return
		}
		a, _ := audit.CanonicalDigest(firstVote)
		b, _ := audit.CanonicalDigest(body.Envelope)
		if a != b {
			t.Error("retry changed the durable vote")
		}
		json.NewEncoder(out).Encode(map[string]bool{"accepted": true})
	})
	server := httptest.NewTLSServer(mux)
	defer server.Close()
	for i := 0; i < 2; i++ {
		if err = observeOne(context.Background(), server.Client(), server.URL, witness); err != nil {
			t.Fatal("unsigned catch-up failed", err)
		}
		latest, ok := witness.Latest()
		if !ok || len(latest.Witnesses) != 0 || postCount() != 0 {
			t.Fatal("historical checkpoint produced a vote")
		}
	}
	witness.Close()
	witness, err = audit.OpenWitness(path, "a2", keys["a2"], history)
	if err != nil {
		t.Fatal(err)
	}
	if err = observeOne(context.Background(), server.Client(), server.URL, witness); err != nil {
		t.Fatal("catch-up failed after restart", err)
	}
	latest, _ := witness.Latest()
	if len(latest.Witnesses) != 1 || postCount() != 0 {
		t.Fatal("eligible checkpoint did not become a durable vote")
	}
	if err = observeOne(context.Background(), server.Client(), server.URL, witness); err == nil {
		t.Fatal("failed acknowledgement was hidden")
	}
	if err = observeOne(context.Background(), server.Client(), server.URL, witness); err != nil || postCount() != 2 {
		t.Fatal("durable vote was not retried", err)
	}
}
