package protocol

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"testing"
)

func TestHiddenSuiteSealWrapAndTamper(t *testing.T) {
	plain := []byte(`{"files":{"case.txt":"c2VjcmV0"},"tree":{"entries":[],"kind":"ScienceLadderArtifactTree","version":1}}`)
	encrypted, material, err := SealHiddenSuite(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encrypted, []byte("secret")) {
		t.Fatal("plaintext leaked in ciphertext")
	}
	decrypted, err := DecryptSuiteObject(material, encrypted, "source")
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := CanonicalJSON(plain)
	if err != nil || !bytes.Equal(decrypted, canonical) {
		t.Fatal("source roundtrip failed")
	}
	host, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	context := HiddenSuiteContext("job-1", material.Commitment)
	capsule, err := WrapSuiteKey(host.PublicKey().Bytes(), material, context)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := UnwrapSuiteKey(host.Bytes(), capsule, context)
	if err != nil || !bytes.Equal(opened.Key, material.Key) {
		t.Fatal("job key capsule failed")
	}
	if _, err := UnwrapSuiteKey(host.Bytes(), capsule, HiddenSuiteContext("job-2", material.Commitment)); err == nil {
		t.Fatal("capsule reused across job")
	}
	other, _ := ecdh.X25519().GenerateKey(rand.Reader)
	if _, err := UnwrapSuiteKey(other.Bytes(), capsule, context); err == nil {
		t.Fatal("wrong host decrypted suite")
	}
	encrypted[len(encrypted)-1] ^= 1
	if _, err := DecryptSuiteObject(material, encrypted, "source"); err == nil {
		t.Fatal("tampered suite decrypted")
	}
}

func TestHiddenDiskEncryptionReproducibleAndRoleBound(t *testing.T) {
	_, material, err := SealHiddenSuite([]byte(`{"fixture":"private"}`))
	if err != nil {
		t.Fatal(err)
	}
	disk := []byte("a canonical read-only disk")
	one, err := EncryptSuiteObject(material, disk, "disk")
	if err != nil {
		t.Fatal(err)
	}
	two, err := EncryptSuiteObject(material, disk, "disk")
	if err != nil || !bytes.Equal(one, two) {
		t.Fatal("independent rebuild ciphertext differs")
	}
	different, _ := EncryptSuiteObject(material, append(disk, '!'), "disk")
	if bytes.Equal(one[:16], different[:16]) {
		t.Fatal("different plaintext reused deterministic nonce")
	}
	if _, err := DecryptSuiteObject(material, one, "source"); err == nil {
		t.Fatal("disk used as source")
	}
	changed := material
	changed.Salt = append([]byte{}, material.Salt...)
	changed.Salt[0] ^= 1
	if _, err := DecryptSuiteObject(changed, one, "disk"); err == nil {
		t.Fatal("wrong commitment accepted")
	}
}

func TestMasterEnvelopeAADAndKeyIsolation(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	ciphertext, err := SealSecret(key, []byte("private metadata"), []byte("suite-1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenSecret(key, ciphertext, []byte("suite-2")); err == nil {
		t.Fatal("metadata moved between suites")
	}
	other := make([]byte, 32)
	_, _ = rand.Read(other)
	if _, err := OpenSecret(other, ciphertext, []byte("suite-1")); err == nil {
		t.Fatal("wrong master key accepted")
	}
}
