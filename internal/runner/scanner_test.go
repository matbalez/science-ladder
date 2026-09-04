package runner

import (
	"archive/zip"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func scannerFixture(now time.Time) (RuntimeInventory, AdvisorySnapshot) {
	p := PackageCoordinate{Ecosystem: "PyPI", Name: "test-dependency", Version: "1.0"}
	source := "https://api.osv.dev/v1/query"
	inventory := RuntimeInventory{APIVersion: "science-ladder-runtime-inventory/v1", RuntimeImageDigest: protocol.DigestBytes([]byte("test image")), Packages: []PackageCoordinate{p}}
	snapshot := AdvisorySnapshot{APIVersion: "science-ladder-advisories/v1", Kind: "AdvisorySnapshot", ID: "unit-test-only", GeneratedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), Sources: []AdvisorySource{{URL: source, FetchedAt: now.Add(-time.Hour), ContentDigest: protocol.DigestBytes([]byte("explicit test fixture"))}}, Coverage: []AdvisoryCoverage{{Package: p, Status: "complete", SourceURL: source, Advisories: []Advisory{}}}}
	return inventory, snapshot
}

func TestAdvisoryCoverageFailsClosed(t *testing.T) {
	now := time.Now().UTC()
	inventory, snapshot := scannerFixture(now)
	if _, status := ScanAdvisories(inventory.Packages, snapshot, now); status != "pass" {
		t.Fatal(status)
	}
	tests := map[string]func(*AdvisorySnapshot){
		"stale":                 func(s *AdvisorySnapshot) { s.GeneratedAt = now.Add(-8 * 24 * time.Hour) },
		"expired":               func(s *AdvisorySnapshot) { s.ExpiresAt = now },
		"future":                func(s *AdvisorySnapshot) { s.GeneratedAt = now.Add(time.Hour) },
		"untrusted source":      func(s *AdvisorySnapshot) { s.Sources[0].URL = "https://creator.example/clean" },
		"missing source digest": func(s *AdvisorySnapshot) { s.Sources[0].ContentDigest = "" },
		"missing package":       func(s *AdvisorySnapshot) { s.Coverage = nil },
		"unknown package":       func(s *AdvisorySnapshot) { s.Coverage[0].Status = "unknown" },
		"wrong package bytes": func(s *AdvisorySnapshot) {
			s.Coverage[0].Package.Digest = protocol.DigestBytes([]byte("different package bytes"))
		},
		"wrong source metadata": func(s *AdvisorySnapshot) {
			s.Coverage[0].Package.SourceName = "different-source"
			s.Coverage[0].Package.SourceVersion = "1.0"
		},
		"wrong version":        func(s *AdvisorySnapshot) { s.Coverage[0].Package.Version = "1.1" },
		"duplicate coordinate": func(s *AdvisorySnapshot) { s.Coverage = append(s.Coverage, s.Coverage[0]) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			_, s := scannerFixture(now)
			mutate(&s)
			findings, status := ScanAdvisories(inventory.Packages, s, now)
			if status != "unknown" || len(findings) == 0 {
				t.Fatalf("%s %+v", status, findings)
			}
		})
	}
	for _, severity := range []string{"low", "medium", "moderate", "high", "critical", "unrecognized"} {
		t.Run(severity, func(t *testing.T) {
			_, s := scannerFixture(now)
			s.Coverage[0].Advisories = []Advisory{{ID: "TEST-ADVISORY", Severity: severity, SourceURL: s.Sources[0].URL}}
			_, status := ScanAdvisories(inventory.Packages, s, now)
			expected := "pass"
			if severity == "high" || severity == "critical" {
				expected = "fail"
			}
			if severity == "unrecognized" {
				expected = "unknown"
			}
			if status != expected {
				t.Fatalf("%s != %s", status, expected)
			}
		})
	}
}

func wheelWithMetadata(t *testing.T, name, version, requires string) []byte {
	t.Helper()
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	f, err := w.Create(name + "-" + version + ".dist-info/METADATA")
	if err != nil {
		t.Fatal(err)
	}
	metadata := "Metadata-Version: 2.1\nName: " + name + "\nVersion: " + version + "\n"
	if requires != "" {
		metadata += "Requires-Dist: " + requires + "\n"
	}
	_, _ = f.Write([]byte(metadata + "\n"))
	_ = w.Close()
	return out.Bytes()
}

func TestDependencyInventoryTracksTransitiveWheels(t *testing.T) {
	base, _ := scannerFixture(time.Now())
	wheel := wheelWithMetadata(t, "thing", "1.0", "missing>=1")
	files := map[string][]byte{"requirements.lock": []byte("thing==1.0 --hash=" + protocol.DigestBytes(wheel)), "wheels/thing-1.0-py3-none-any.whl": wheel}
	if _, err := DependencyInventory(files, "requirements.lock", base); err == nil {
		t.Fatal("unaccounted transitive dependency accepted")
	}
	dependency := wheelWithMetadata(t, "missing", "1.5", "")
	files["wheels/missing-1.5-py3-none-any.whl"] = dependency
	files["requirements.lock"] = append(files["requirements.lock"], []byte("\nmissing==1.5 --hash="+protocol.DigestBytes(dependency))...)
	packages, err := DependencyInventory(files, "requirements.lock", base)
	if err != nil || len(packages) != 3 {
		t.Fatalf("%+v %v", packages, err)
	}
	if packages[0].Digest == "" {
		t.Fatal("wheel hash missing from inventory")
	}
	sbom, err := BuildSBOM(base.RuntimeImageDigest, packages)
	if err != nil || !bytes.Contains(sbom, []byte(`"bomFormat":"CycloneDX"`)) {
		t.Fatal(err)
	}
	files["wheels/missing-1.5-py3-none-any.whl"] = wheelWithMetadata(t, "different", "1.5", "")
	if _, err := DependencyInventory(files, "requirements.lock", base); err == nil {
		t.Fatal("mutated pinned dependency accepted")
	}
}

func TestPackageNormalization(t *testing.T) {
	p, err := normalizePackage(PackageCoordinate{Ecosystem: "PyPI", Name: "Some__Package.Name", Version: "v01.02RC03"})
	if err != nil || p.Name != "some-package-name" || p.Version != "1.2rc3" {
		t.Fatalf("%+v %v", p, err)
	}
	for _, v := range []string{"latest", "1.0+unreviewed", "1.0;exec()", "NaN"} {
		if _, err := normalizePythonVersion(v); err == nil {
			t.Fatalf("accepted %q", v)
		}
	}
}

func TestScanRequiresPlatformSignedPinnedPolicy(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	inventory, snapshot := scannerFixture(now)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := protocol.Sign("platform-advisories", key, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	pin := func(name string, value any) PinnedFile {
		t.Helper()
		data, _ := json.Marshal(value)
		filename := filepath.Join(dir, name)
		if err := os.WriteFile(filename, data, 0400); err != nil {
			t.Fatal(err)
		}
		return PinnedFile{Path: filename, Digest: protocol.DigestBytes(data)}
	}
	config := Config{RuntimeInventory: pin("runtime.json", inventory), AdvisorySnapshot: pin("advisory.json", envelope), AdvisoryKeys: pin("keys.json", map[string]string{"platform-advisories": string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))})}
	builder := Builder{Runtime: &Runtime{Config: config}}
	manifest := protocol.Manifest{Validator: protocol.Validator{RuntimeImageDigest: inventory.RuntimeImageDigest, DependencyLock: "requirements.lock"}}
	files := map[string][]byte{"requirements.lock": []byte("# stdlib")}
	scan, sbom, err := builder.Scan(files, manifest, filepath.Join(dir, "sbom.json"))
	if err != nil || scan.Status != "pass" || sbom.Digest != scan.SBOMDigest {
		t.Fatalf("%+v %v", scan, err)
	}
	foreign, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	envelope, _ = protocol.Sign("platform-advisories", foreign, snapshot)
	builder.Runtime.Config.AdvisorySnapshot = pin("foreign.json", envelope)
	if _, _, err := builder.Scan(files, manifest, filepath.Join(dir, "foreign-sbom.json")); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("foreign signer accepted: %v", err)
	}
	builder.Runtime.Config.AdvisorySnapshot.Digest = protocol.DigestBytes([]byte("wrong"))
	if _, _, err := builder.Scan(files, manifest, filepath.Join(dir, "tampered-sbom.json")); err == nil {
		t.Fatal("wrong file digest accepted")
	}
}
