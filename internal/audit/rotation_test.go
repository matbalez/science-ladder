package audit

import (
	"crypto/ecdsa"
	"encoding/json"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func rotatedWitnessHistory(t *testing.T) (History, History, *ecdsa.PrivateKey, map[string]*ecdsa.PrivateKey) {
	t.Helper()
	initial, root, keys := fixtureHistory(t)
	initial.IssuedAt = time.Now().UTC().Add(-2 * time.Hour)
	genesis, err := Build("test", nil, nil, initial.IssuedAt)
	if err != nil {
		t.Fatal(err)
	}
	initial.GenesisCheckpointDigest = mustDigest(genesis)
	for i := range initial.Delegations {
		initial.Delegations[i].NotBefore = initial.IssuedAt.Add(-time.Hour)
		initial.Delegations[i].NotAfter = initial.IssuedAt.Add(24 * time.Hour)
	}
	initial, err = VerifyHistory(signed(t, "root", root, initial), "root", root.Public(), nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	next := initial
	next.IssuedAt = initial.IssuedAt.Add(time.Hour)
	next.PreviousDigest = mustDigest(initial)
	next.Delegations = append([]Delegation(nil), initial.Delegations...)
	next.Witnesses = append([]WitnessIdentity(nil), initial.Witnesses...)
	for i := range next.Delegations {
		if next.Delegations[i].KeyID == "a" {
			at := next.IssuedAt
			next.Delegations[i].RevokedAt = &at
		}
	}
	keys["a2"] = newKey(t)
	next.Delegations = append(next.Delegations, Delegation{KeyID: "a2", PublicKeyPEM: publicPEM(t, keys["a2"]), Roles: []string{"audit-witness"}, NotBefore: next.IssuedAt, NotAfter: initial.IssuedAt.Add(24 * time.Hour), Custody: "test-only"})
	for i := range next.Witnesses {
		if next.Witnesses[i].KeyID == "a" {
			next.Witnesses[i].KeyID = "a2"
		}
	}
	next, err = VerifyHistory(signed(t, "root", root, next), "root", root.Public(), &initial, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return initial, next, root, keys
}

func TestQuorumToleratesOneInactiveWitnessButNotOneVoteOrSharedIdentity(t *testing.T) {
	history, _, private := fixtureHistory(t)
	payload := map[string]string{"checkpoint": "test-only"}
	platform := signed(t, "online", private["online"], payload)
	a := signed(t, "a", private["a"], payload)
	b := signed(t, "b", private["b"], payload)
	keys := history.KeysAt("audit-witness", history.IssuedAt)
	delete(keys, "c") // Expired, revoked, or otherwise inactive third delegation.
	operators := history.WitnessOperatorsAt(history.IssuedAt)
	if err := VerifyQuorum(platform, []protocol.Envelope{a, b}, keys, operators, 2); err != nil {
		t.Fatal("two valid independent votes must survive one inactive key", err)
	}
	for _, votes := range [][]protocol.Envelope{{a}, {a, a}, {a, signed(t, "c", private["c"], payload)}} {
		if err := VerifyQuorum(platform, votes, keys, operators, 2); err == nil {
			t.Fatal("counted insufficient, duplicated, or inactive votes")
		}
	}
	operators["c"] = operators["a"]
	if err := VerifyQuorum(platform, []protocol.Envelope{a, b}, keys, operators, 2); err == nil {
		t.Fatal("accepted registry with a shared operator")
	}
	operators["c"] = "c"
	keys["b"] = keys["a"]
	if err := VerifyQuorum(platform, []protocol.Envelope{a, b}, keys, operators, 2); err == nil {
		t.Fatal("accepted active key aliased across two operators")
	}
}

func TestWitnessMembershipRotationPreservesHistoricalQuorum(t *testing.T) {
	initial, latest, _, private := rotatedWitnessHistory(t)
	before := latest.IssuedAt.Add(-time.Nanosecond)
	for _, tc := range []struct {
		at time.Time
		id string
	}{{initial.IssuedAt.Add(-time.Second), "a"}, {before, "a"}, {latest.IssuedAt, "a2"}, {time.Now(), "a2"}} {
		operators := latest.WitnessOperatorsAt(tc.at)
		if len(operators) != 3 || operators[tc.id] != "a" {
			t.Fatalf("wrong membership at %s: %v", tc.at, operators)
		}
		payload := map[string]string{"at": tc.at.Format(time.RFC3339Nano)}
		platform := signed(t, "online", private["online"], payload)
		votes := []protocol.Envelope{signed(t, tc.id, private[tc.id], payload), signed(t, "b", private["b"], payload)}
		if err := VerifyQuorum(platform, votes, latest.KeysAt("audit-witness", tc.at), operators, 2); err != nil {
			t.Fatal("historical or current quorum lost after rotation", err)
		}
	}
	// Lineage must not change signed bytes or depend on mutable caller slices.
	digest := mustDigest(latest)
	initial.Witnesses[0].Operator = "tampered"
	if latest.WitnessOperatorsAt(before)["a"] != "a" || mustDigest(latest) != digest {
		t.Fatal("historical membership was rewritten or altered signed history")
	}
	b, err := json.Marshal(latest)
	if err != nil {
		t.Fatal(err)
	}
	var incomplete History
	if err = json.Unmarshal(b, &incomplete); err != nil {
		t.Fatal(err)
	}
	if len(incomplete.WitnessOperatorsAt(before)) != 0 {
		t.Fatal("unverified missing lineage invented historical membership")
	}
	// Root authorization of a2 cannot make the revoked a key count today.
	payload := map[string]string{"checkpoint": "after-rotation"}
	platform := signed(t, "online", private["online"], payload)
	votes := []protocol.Envelope{signed(t, "a", private["a"], payload), signed(t, "b", private["b"], payload)}
	if err := VerifyQuorum(platform, votes, latest.KeysAt("audit-witness", time.Now()), latest.WitnessOperatorsAt(time.Now()), 2); err == nil {
		t.Fatal("revoked witness still counted in current quorum")
	}
}
