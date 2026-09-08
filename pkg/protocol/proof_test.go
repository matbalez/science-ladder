package protocol

import "testing"

func TestProofContractBindsStatementAndSealedCertificate(t *testing.T) {
	p := &ProofContract{Format: "lean", StatementPath: "statements/target.lean", StatementDigest: DigestBytes([]byte("target")), CertificatePath: "Proof.ndjson", AllowedAxioms: []string{"propext", "Quot.sound"}, CheckDescription: "Replay the kernel and compare the exact target type; reject undeclared axioms."}
	e := EvaluationContract{Mode: "proof", Proof: p, Program: &CandidateProgram{ScratchMB: 32, Products: []BuildProduct{{Path: "Proof.ndjson", MaxBytes: 1 << 20}}}}
	if err := validateProofContract(e); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*ProofContract){func(p *ProofContract) { p.StatementPath = "../candidate.lean" }, func(p *ProofContract) { p.StatementDigest = "" }, func(p *ProofContract) { p.CertificatePath = "different.ndjson" }, func(p *ProofContract) { p.AllowedAxioms = []string{"sorryAx", "sorryAx"} }} {
		q := *p
		mutate(&q)
		e.Proof = &q
		if validateProofContract(e) == nil {
			t.Fatal("unbound proof accepted")
		}
	}
	e.Proof = p
	e.Mode = "program"
	if validateProofContract(e) == nil {
		t.Fatal("proof contract silently ignored")
	}
}
