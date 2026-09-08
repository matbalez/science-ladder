package runner

import (
	"github.com/matbalez/science-ladder/pkg/protocol"
	"strings"
	"testing"
)

func TestAssetDomainsAndAdvisoryBindings(t *testing.T) {
	candidate := protocol.EvaluationAsset{Name: "lean", Domain: "candidate", Purpose: "toolchain", Visibility: "public", Digest: protocol.DigestBytes([]byte("candidate image")), Size: 123}
	checker := protocol.EvaluationAsset{Name: "proof-tools", Domain: "checker", Purpose: "proof", Visibility: "public", Digest: protocol.DigestBytes([]byte("checker image")), Size: 456}
	if assetGuestPath(candidate) != "/opt/sl-private/assets/lean" || assetGuestPath(checker) != "/sl/assets/proof-tools" {
		t.Fatal("toolchain domains share checker-visible paths")
	}
	p := PackageCoordinate{Ecosystem: "Git", Name: "github.com/leanprover/lean4", Version: strings.Repeat("a", 40), Digest: protocol.DigestBytes([]byte("pinned source")), ExecutionDomain: "checker"}
	if _, err := normalizePackage(p); err != nil {
		t.Fatal(err)
	}
	c := Config{Assets: []AssetDisk{{Asset: candidate}, {Asset: checker}}}
	inventory := RuntimeInventory{Packages: []PackageCoordinate{p}, Assets: []AssetInventoryBinding{{Asset: candidate, ComponentInventoryDigest: p.Digest, PackageKeys: []string{packageKey(p)}}, {Asset: checker, ComponentInventoryDigest: p.Digest, PackageKeys: []string{packageKey(p)}}}}
	if err := validateAssetInventory(c, inventory); err != nil {
		t.Fatal(err)
	}
	inventory.Packages[0].ExecutionDomain = "candidate-only"
	if validateAssetInventory(c, inventory) == nil {
		t.Fatal("checker tool borrowed candidate-only advisory exception")
	}
	inventory.Packages[0].ExecutionDomain = "checker"
	inventory.Assets[1].Asset.Digest = protocol.DigestBytes([]byte("drift"))
	if validateAssetInventory(c, inventory) == nil {
		t.Fatal("asset bytes drift accepted")
	}
	inventory.Assets = inventory.Assets[:1]
	if validateAssetInventory(c, inventory) == nil {
		t.Fatal("asset omitted from inventory")
	}
}

func TestStaticLibraryInventoryNeedsBytesAndAdvisoryCoverage(t *testing.T) {
	p := PackageCoordinate{Ecosystem: "Generic", Name: "gmp", Version: "6.3.0", Digest: protocol.DigestBytes([]byte("static library")), ExecutionDomain: "checker"}
	if _, err := normalizePackage(p); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildSBOM(protocol.DigestBytes([]byte("runtime")), []PackageCoordinate{p}); err != nil {
		t.Fatal(err)
	}
	p.Digest = ""
	if _, err := normalizePackage(p); err == nil {
		t.Fatal("unbound static library accepted")
	}
}

func TestProofBuildCanHandOffCertificateWithoutDummyExecution(t *testing.T) {
	m := nativeProbeManifest(protocol.DigestBytes([]byte("runtime")), "proof.py", []string{"/usr/local/bin/python3", "proof.py"})
	m.Evaluation.Mode = "proof"
	m.Evaluation.Executor.Features = append(m.Evaluation.Executor.Features, "sealed-products", "native-proof-checker")
	m.Evaluation.Program.Products = []protocol.BuildProduct{{Path: "certificate", MaxBytes: 1024}}
	m.Evaluation.Program.MinRuns = 0
	m.Evaluation.Proof = &protocol.ProofContract{Format: "drat", StatementPath: "statements/formula.cnf", StatementDigest: protocol.DigestBytes([]byte("formula")), CertificatePath: "certificate", AllowedAxioms: []string{}, CheckDescription: "Replay a serialized DRAT certificate against the exact frozen CNF formula."}
	if err := protocol.ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
	m.Evaluation.Mode = "program"
	m.Evaluation.Proof = nil
	if protocol.ValidateManifest(m) == nil {
		t.Fatal("ordinary program skipped all execution")
	}
}

func TestPairedTimingReferenceCannotBeRelabeledAsAnArbitraryScore(t *testing.T) {
	m := nativeTimingManifest(nativeProbeManifest(protocol.DigestBytes([]byte("runtime")), "probe.c", []string{"/usr/bin/gcc", "probe.c", "-o", "/work/probe"}), "fixed-host")
	if err := protocol.ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
	m.Metric.BaselineTicks = "900000"
	if protocol.ValidateManifest(m) == nil {
		t.Fatal("mislabeled matched baseline accepted")
	}
}

func TestProofAndPairedTimingComposeWithoutDroppingEitherContract(t *testing.T) {
	m := nativeTimingManifest(nativeProbeManifest(protocol.DigestBytes([]byte("runtime")), "probe.c", []string{"/usr/bin/gcc", "probe.c", "-o", "/work/probe"}), "fixed-host")
	m.Evaluation.Executor.Features = append(m.Evaluation.Executor.Features, "sealed-products", "native-proof-checker", "proof-timing-composition")
	m.Evaluation.Program.Products = []protocol.BuildProduct{{Path: "certificate", MaxBytes: 1024}}
	m.Evaluation.Proof = &protocol.ProofContract{Format: "drat", StatementPath: "statements/formula.cnf", StatementDigest: protocol.DigestBytes([]byte("formula")), CertificatePath: "certificate", AllowedAxioms: []string{}, CheckDescription: "Replay a bound certificate before every paired correctness assessment can be accepted."}
	if err := protocol.ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
	m.Evaluation.Executor.Features = m.Evaluation.Executor.Features[:len(m.Evaluation.Executor.Features)-1]
	if protocol.ValidateManifest(m) == nil {
		t.Fatal("uncommissioned proof/timing composition accepted")
	}
}
