package protocol

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestCanonicalRejectsAmbiguity(t *testing.T) {
	for _, input := range []string{`{"a":1,"a":2}`, `{"a":{"x":1,"x":2}}`, `{} {}`, `{"x":NaN}`, `{"x":1e9999}`, strings.Repeat("[", 40) + "0" + strings.Repeat("]", 40)} {
		if _, err := CanonicalJSON([]byte(input)); err == nil {
			t.Errorf("accepted ambiguous input %s", input)
		}
	}
	a, err := CanonicalJSON([]byte(`{"z":1,"a":"<>&","nested":{"y":2,"x":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != `{"a":"<>&","nested":{"x":1,"y":2},"z":1}` {
		t.Fatal(string(a))
	}
}

func TestScoutCanTruthfullyAbstainWithoutInventingSources(t *testing.T) {
	c := Candidate{APIVersion: APIVersion, Kind: "ChallengeCandidate", ID: "abstention-fixture", CreatedAt: time.Now(), Producer: "protocol-test", PromptVersion: ScoutVersion, Disposition: "rejected", Sources: []Source{}, Uncertainties: []string{"Necessary primary evidence could not be inspected"}}
	if err := ValidateCandidate(c); err != nil {
		t.Fatal(err)
	}
	c.Disposition = "viable"
	if ValidateCandidate(c) == nil {
		t.Fatal("viable candidate accepted without evidence or manifest")
	}
}

func TestStrictYAML(t *testing.T) {
	for _, input := range []string{"a: 1\na: 2", "a: &x hello\nb: *x", "a: !!str hi", "date: 2026-01-01", "score: .nan", "n: 0x10", "a: 1\n---\nb: 2"} {
		if _, err := AuthoringJSON([]byte(input)); err == nil {
			t.Errorf("accepted unsafe YAML %q", input)
		}
	}
	if _, err := AuthoringJSON([]byte("score: \"0.1\"\ncreatedAt: \"2026-01-01T00:00:00Z\"")); err != nil {
		t.Fatal(err)
	}
}

func TestScoreRoundingAgainstSolver(t *testing.T) {
	for _, test := range []struct{ score, quantum, direction, want string }{{"1.29", "0.1", "maximize", "12"}, {"1.29", "0.1", "minimize", "13"}, {"-1.29", "0.1", "maximize", "-13"}, {"-1.29", "0.1", "minimize", "-12"}, {"100000000000000000000000000000.2", "0.1", "maximize", "1000000000000000000000000000002"}} {
		got, err := NormalizeScore(test.score, Metric{Quantum: test.quantum, Direction: test.direction})
		if err != nil || got != test.want {
			t.Errorf("%+v got %s %v", test, got, err)
		}
	}
	for _, score := range []string{"NaN", "Infinity", "1e3", "01", "-0", "-0.000", "1.", ".1", strings.Repeat("9", 101)} {
		if _, err := NormalizeScore(score, Metric{Quantum: "1", Direction: "maximize"}); err == nil {
			t.Errorf("accepted %s", score)
		}
	}
}

func TestConfirmationConservative(t *testing.T) {
	m := Metric{Direction: "maximize", ToleranceTicks: "2"}
	if score, err := ConfirmScores("9", "11", m); err != nil || score != "9" {
		t.Fatalf("%s %v", score, err)
	}
	if _, err := ConfirmScores("9", "12", m); err == nil {
		t.Fatal("accepted divergent result")
	}
}

func TestDSSETamperAndType(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := Sign("test-root", key, map[string]any{"economicMode": "none", "ticks": "123"})
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]crypto.PublicKey{"test-root": key.Public()}
	if _, err := Verify(envelope, keys); err != nil {
		t.Fatal(err)
	}
	copy := envelope
	copy.Payload = base64.StdEncoding.EncodeToString([]byte(`{"economicMode":"none","ticks":"124"}`))
	if _, err := Verify(copy, keys); err == nil {
		t.Fatal("tampered payload verified")
	}
	copy = envelope
	copy.PayloadType = "text/plain"
	if _, err := Verify(copy, keys); err == nil {
		t.Fatal("wrong payload type verified")
	}
	copy = envelope
	copy.Signatures = append(copy.Signatures, copy.Signatures[0])
	if _, err := Verify(copy, keys); err == nil {
		t.Fatal("duplicate signer verified")
	}
}

func contract() SubmissionContract {
	return SubmissionContract{AllowedPaths: []string{"data/"}, AllowedExtensions: []string{".json", ".txt"}, MaxBytes: 4 << 20, MaxFiles: 10, License: "MIT"}
}
func archive(t *testing.T, headers []*tar.Header, contents [][]byte, compressed bool) []byte {
	t.Helper()
	var data bytes.Buffer
	var writer *tar.Writer
	var compressedWriter *gzip.Writer
	if compressed {
		compressedWriter = gzip.NewWriter(&data)
		writer = tar.NewWriter(compressedWriter)
	} else {
		writer = tar.NewWriter(&data)
	}
	for i, h := range headers {
		if err := writer.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if len(contents) > i {
			if _, err := writer.Write(contents[i]); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if compressed {
		if err := compressedWriter.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return data.Bytes()
}

func TestArchiveDecompressionBomb(t *testing.T) {
	contents := bytes.Repeat([]byte("x"), 2<<20)
	raw := archive(t, []*tar.Header{{Name: "data/bomb.txt", Mode: 0644, Typeflag: tar.TypeReg, Size: int64(len(contents))}}, [][]byte{contents}, true)
	if _, _, _, err := ReadArtifactArchive(bytes.NewReader(raw), contract()); err == nil {
		t.Fatal("decompression bomb accepted")
	}
}

func TestArtifactDigestStableAndSensitive(t *testing.T) {
	files := map[string][]byte{"data/a.json": []byte(`{"a":1}`), "data/b.txt": []byte("two")}
	_, a, err := ArtifactFromFiles(files, contract())
	if err != nil {
		t.Fatal(err)
	}
	reordered := map[string][]byte{"data/b.txt": []byte("two"), "data/a.json": []byte(`{"a":1}`)}
	_, b, err := ArtifactFromFiles(reordered, contract())
	if err != nil || a != b {
		t.Fatal("order changed digest")
	}
	reordered["data/b.txt"] = []byte("three")
	_, b, _ = ArtifactFromFiles(reordered, contract())
	if a == b {
		t.Fatal("mutation failed to change digest")
	}
}

func TestArchiveTrailerIntegrityAndFramingBudget(t *testing.T) {
	content := bytes.Repeat([]byte("x"), 2048)
	c := contract()
	c.MaxBytes = int64(len(content))
	c.MaxFiles = 1
	header := &tar.Header{Name: "data/input.txt", Typeflag: tar.TypeReg, Mode: 0644, Size: int64(len(content))}
	raw := archive(t, []*tar.Header{header}, [][]byte{content}, false)
	if _, _, _, err := ReadArtifactArchive(bytes.NewReader(raw), c); err != nil {
		t.Fatal("valid payload at limit lost its framing budget", err)
	}
	if _, _, _, err := ReadArtifactArchive(bytes.NewReader(append(raw, []byte("smuggled")...)), c); err == nil {
		t.Fatal("nonzero trailing data accepted")
	}
	gz := archive(t, []*tar.Header{header}, [][]byte{content}, true)
	if _, _, _, err := ReadArtifactArchive(bytes.NewReader(gz), c); err != nil {
		t.Fatal(err)
	}
	gz[len(gz)-8] ^= 1
	if _, _, _, err := ReadArtifactArchive(bytes.NewReader(gz), c); err == nil {
		t.Fatal("corrupt gzip CRC accepted")
	}
}

func TestMaliciousArchives(t *testing.T) {
	tests := []*tar.Header{{Name: "../escape.txt", Typeflag: tar.TypeReg, Mode: 0644}, {Name: "data/link.txt", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}, {Name: "data/hard.txt", Typeflag: tar.TypeLink, Linkname: "data/a.txt"}, {Name: "data/program.txt", Typeflag: tar.TypeReg, Mode: 0755}, {Name: "data/device.txt", Typeflag: tar.TypeChar}, {Name: "/data/absolute.txt", Typeflag: tar.TypeReg, Mode: 0644}}
	for _, h := range tests {
		raw := archive(t, []*tar.Header{h}, nil, false)
		if _, _, _, err := ReadArtifactArchive(bytes.NewReader(raw), contract()); err == nil {
			t.Errorf("accepted %+v", h)
		}
	}
	h := &tar.Header{Name: "data/a.txt", Typeflag: tar.TypeReg, Mode: 0644}
	raw := archive(t, []*tar.Header{h, h}, nil, false)
	if _, _, _, err := ReadArtifactArchive(bytes.NewReader(raw), contract()); err == nil {
		t.Fatal("accepted duplicate")
	}
}

func TestArtifactActiveContentAndNames(t *testing.T) {
	for _, name := range []string{"data/../a.txt", "data/a\\b.txt", "data/e\u0301.txt", "data/.git/config.txt"} {
		if _, _, err := ArtifactFromFiles(map[string][]byte{name: []byte("x")}, contract()); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	for _, contents := range []string{"#!/bin/sh\necho x", "<svg onload='x' />", "version https://git-lfs.github.com/spec/v1\n"} {
		if _, _, err := ArtifactFromFiles(map[string][]byte{"data/a.txt": []byte(contents)}, contract()); err == nil {
			t.Fatal("accepted active content")
		}
	}
	if _, _, err := ArtifactFromFiles(map[string][]byte{"data/A.txt": nil, "data/a.txt": nil}, contract()); err == nil {
		t.Fatal("accepted case collision")
	}
}

func TestGatesRejectMissingUnknownDuplicate(t *testing.T) {
	m := Manifest{Metric: Metric{Quantum: "1", Direction: "maximize"}, HardGates: []string{"valid"}}
	for _, value := range []string{`{"apiVersion":"science-ladder/v1","kind":"ValidatorResult","score":"1","gates":{}}`, `{"apiVersion":"science-ladder/v1","kind":"ValidatorResult","score":"1","gates":{"valid":true,"other":true}}`, `{"apiVersion":"science-ladder/v1","kind":"ValidatorResult","score":"1","gates":{"valid":true,"valid":false}}`} {
		if _, _, err := ValidateResult([]byte(value), m); err == nil {
			t.Fatal("invalid gate set accepted")
		}
	}
}
