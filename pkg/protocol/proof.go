package protocol

import (
	"errors"
	"strings"
)

// Build products are inert, bounded files copied out after all compiler writers
// have been reaped. They never confer authority on a candidate's own checker.
type BuildProduct struct {
	Path     string `json:"path"`
	MaxBytes int64  `json:"maxBytes"`
}

// ProofContract identifies the proposition independently of submitted search
// code. The frozen validator must replay the certificate against that statement
// and enforce its declared axiom policy; compiler exit status is insufficient.
type ProofContract struct {
	Format           string   `json:"format"` // drat, lean, other
	StatementPath    string   `json:"statementPath"`
	StatementDigest  string   `json:"statementDigest"`
	CertificatePath  string   `json:"certificatePath"`
	AllowedAxioms    []string `json:"allowedAxioms"`
	CheckDescription string   `json:"checkDescription"`
}

func validateBuildProducts(p *CandidateProgram) error {
	if len(p.Products) > 64 {
		return errors.New("too many declared build products")
	}
	seen := map[string]bool{}
	var total int64
	for _, product := range p.Products {
		if ValidatePath(product.Path) != nil || seen[product.Path] || product.MaxBytes < 1 || product.MaxBytes > 16<<30 {
			return errors.New("invalid frozen build product")
		}
		seen[product.Path] = true
		total += product.MaxBytes
	}
	if total > int64(p.ScratchMB)<<20 {
		return errors.New("build products exceed candidate scratch budget")
	}
	return nil
}
func validateProofContract(e EvaluationContract) error {
	p := e.Proof
	if e.Mode != "proof" {
		if p != nil {
			return errors.New("proof contract requires proof evaluation")
		}
		return nil
	}
	if p == nil || ValidatePath(p.StatementPath) != nil || !strings.HasPrefix(p.StatementPath, "statements/") || !ValidDigest(p.StatementDigest) || ValidatePath(p.CertificatePath) != nil || len(strings.TrimSpace(p.CheckDescription)) < 20 || len(p.CheckDescription) > 8192 {
		return errors.New("proof requires a frozen statement, certificate and replay description")
	}
	if p.Format != "drat" && p.Format != "lean" && p.Format != "other" {
		return errors.New("unknown proof format")
	}
	if len(p.AllowedAxioms) > 128 || (p.Format == "drat" && len(p.AllowedAxioms) != 0) {
		return errors.New("invalid axiom policy")
	}
	seen := map[string]bool{}
	for _, name := range p.AllowedAxioms {
		if name == "" || len(name) > 256 || strings.ContainsAny(name, "\n\r\x00") || seen[name] {
			return errors.New("invalid or repeated allowed axiom")
		}
		seen[name] = true
	}
	if e.Program != nil {
		found := false
		for _, product := range e.Program.Products {
			found = found || product.Path == p.CertificatePath
		}
		if !found {
			return errors.New("compiled proof certificate must be a declared sealed product")
		}
	}
	return nil
}
