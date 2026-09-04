package platform

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	secretbox "github.com/matbalez/science-ladder/internal/secrets"
	"github.com/matbalez/science-ladder/internal/storage"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHiddenSuitePathAndActiveContentPolicy(t *testing.T) {
	for _, files := range []map[string]string{{"../cases.json": "e30="}, {".git/config": "e30="}, {"a\u0000.json": "e30="}, {"data.json": base64.StdEncoding.EncodeToString([]byte("#!/bin/sh\necho leak"))}, {"A.json": "e30=", "a.json": "e30="}} {
		if err := validateSuiteDocument(raw(map[string]any{"files": files})); err == nil {
			t.Fatal("unsafe hidden suite passed")
		}
	}
	if err := validateSuiteDocument(raw(map[string]any{"files": map[string]string{"cases.json": "WzFd"}})); err != nil {
		t.Fatal(err)
	}
}
func TestHiddenSuiteEncryptedAtRestAndHostBound(t *testing.T) {
	if os.Getenv("S3_BUCKET") == "" {
		t.Skip("local S3-compatible test storage is not configured")
	}
	s := testDB(t)
	u, _ := seed(t, s)
	ctx := context.Background()
	var err error
	s.Store, err = storage.New(ctx, os.Getenv("S3_BUCKET"), os.Getenv("S3_REGION"), os.Getenv("S3_ENDPOINT"))
	if err != nil {
		t.Fatal(err)
	}
	master := make([]byte, 32)
	if _, err = rand.Read(master); err != nil {
		t.Fatal(err)
	}
	s.SuiteSealer, err = secretbox.NewLocal(master, "local")
	if err != nil {
		t.Fatal(err)
	}
	sentinel := "private-suite-" + ID()
	document := raw(map[string]any{"files": map[string]string{"cases.json": base64.StdEncoding.EncodeToString(raw(map[string]string{"case": sentinel}))}})
	input := raw(map[string]any{"document": string(document), "license": "MIT", "provenance": "Synthetic confidential suite used only for this security integration test."})
	r := httptest.NewRequest("POST", "/v1/suites", strings.NewReader(string(input)))
	r.Header.Set("Idempotency-Key", "hidden-suite-create")
	w := httptest.NewRecorder()
	if err = s.createSuite(w, r, u); err != nil {
		t.Fatal(err)
	}
	var response struct {
		ID               string `json:"id"`
		Commitment       string `json:"commitment"`
		CiphertextDigest string `json:"ciphertextDigest"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ID == "" || strings.Contains(w.Body.String(), sentinel) || strings.Contains(w.Body.String(), "salt") {
		t.Fatal("hidden material escaped its creation response")
	}
	var encrypted []byte
	if err = s.DB.QueryRow(ctx, `SELECT encrypted_material FROM hidden_suites WHERE id=$1`, response.ID).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	material, digest, err := s.suiteMaterial(ctx, response.Commitment)
	if err != nil {
		t.Fatal(err)
	}
	defer protocol.ZeroBytes(material.Key)
	defer protocol.ZeroBytes(material.Salt)
	if bytes.Contains(encrypted, material.Key) || bytes.Contains(encrypted, material.Salt) {
		t.Fatal("suite key material persisted in plaintext")
	}
	ciphertext, err := s.Store.Get(ctx, digest, 3<<20)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte(sentinel)) {
		t.Fatal("suite data stored in plaintext")
	}
	plain, err := protocol.DecryptSuiteObject(material, ciphertext, "source")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(plain, []byte(base64.StdEncoding.EncodeToString(raw(map[string]string{"case": sentinel})))) {
		t.Fatal("encrypted suite did not round-trip")
	}
	protocol.ZeroBytes(plain)
	hostKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	job := protocol.RunnerJob{ID: ID(), Purpose: "preflight", Manifest: protocol.Manifest{Suite: protocol.Suite{Visibility: "hidden", Commitment: response.Commitment}}}
	host := runnerIdentity{ID: "r1", EncryptionKey: base64.StdEncoding.EncodeToString(hostKey.PublicKey().Bytes())}
	if err = s.grantHiddenSuite(ctx, &job, host); err != nil {
		t.Fatal(err)
	}
	opened, err := protocol.UnwrapSuiteKey(hostKey.Bytes(), job.HiddenSuite.KeyCapsule, protocol.HiddenSuiteContext(job.ID, response.Commitment))
	if err != nil || opened.Commitment != response.Commitment {
		t.Fatal("registered host could not open its bound suite capsule")
	}
	protocol.ZeroBytes(opened.Key)
	protocol.ZeroBytes(opened.Salt)
	other, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = protocol.UnwrapSuiteKey(other.Bytes(), job.HiddenSuite.KeyCapsule, protocol.HiddenSuiteContext(job.ID, response.Commitment)); err == nil {
		t.Fatal("another host opened a suite capsule")
	}
	if _, err = protocol.UnwrapSuiteKey(hostKey.Bytes(), job.HiddenSuite.KeyCapsule, protocol.HiddenSuiteContext(ID(), response.Commitment)); err == nil {
		t.Fatal("suite capsule replayed in a different job")
	}

	// Reveals expose existing ciphertext plus signed decryption evidence. They
	// never place an unencrypted object in CAS before an eligibility commit.
	var challenge string
	if err = s.DB.QueryRow(ctx, `SELECT id FROM challenges WHERE owner_id=$1`, u.ID).Scan(&challenge); err != nil {
		t.Fatal(err)
	}
	version := ID()
	past := time.Now().Add(-time.Hour)
	manifest := protocol.Manifest{Suite: protocol.Suite{Visibility: "hidden", Commitment: response.Commitment, RevealAt: &past}}
	if _, err = s.DB.Exec(ctx, `INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,deadline) VALUES($1,$2,'local/test',1,$3,'source',$4,now()+interval '1 day')`, version, challenge, strings.Repeat("c", 40), raw(manifest)); err != nil {
		t.Fatal(err)
	}
	reveal := func() (map[string]any, error) {
		r := httptest.NewRequest("POST", "/v1/suites/"+response.ID+"/reveal", strings.NewReader("{}"))
		r.SetPathValue("id", response.ID)
		r.Header.Set("Idempotency-Key", "suite-reveal-test")
		w := httptest.NewRecorder()
		err := s.revealSuite(w, r, u)
		result := map[string]any{}
		json.Unmarshal(w.Body.Bytes(), &result)
		return result, err
	}
	if _, err = reveal(); err == nil {
		t.Fatal("active season suite was revealed")
	}
	if _, err = s.Store.Get(ctx, material.PlaintextDigest, 3<<20); err == nil {
		t.Fatal("failed reveal wrote plaintext to CAS")
	}
	if _, err = s.DB.Exec(ctx, `UPDATE challenge_versions SET intake_status='closed' WHERE id=$1`, version); err != nil {
		t.Fatal(err)
	}
	revealed, err := reveal()
	if err != nil {
		t.Fatal(err)
	}
	var payload []byte
	if err = s.DB.QueryRow(ctx, `SELECT payload FROM receipts WHERE digest=$1 AND public`, revealed["receiptDigest"]).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var evidence protocol.Receipt
	if err = json.Unmarshal(payload, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.Data["sourceFormat"] != "encrypted-suite-object-v1" || evidence.Data["sourceDigest"] != digest {
		t.Fatal("reveal must identify the immutable encrypted source")
	}
	var public bool
	if err = s.DB.QueryRow(ctx, `SELECT public_at IS NOT NULL FROM artifacts WHERE digest=$1`, digest).Scan(&public); err != nil || !public {
		t.Fatal("revealed ciphertext remained inaccessible")
	}
	if _, err = s.Store.Get(ctx, material.PlaintextDigest, 3<<20); err == nil {
		t.Fatal("successful reveal unnecessarily persisted plaintext")
	}
	if err = s.requireSuiteOwner(ctx, manifest, u); err == nil {
		t.Fatal("revealed suite reused as secret in a new version")
	}
}
