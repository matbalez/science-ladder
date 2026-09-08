package runner

import (
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

// ProofHardwareProbe runs only fixed first-party fixtures. It is not a challenge
// approval, independent security audit or evidence of a scientific achievement.
func (r *Runtime) ProofHardwareProbe(ctx context.Context, diagnostics io.Writer) (protocol.Envelope, error) {
	if err := r.Config.CheckHost(r.Keys); err != nil {
		return protocol.Envelope{}, err
	}
	if r.Signer == nil || r.Config.Capabilities == nil {
		return protocol.Envelope{}, errors.New("proof probe needs signed capabilities")
	}
	started := time.Now().UTC()
	checks := []map[string]any{}
	var failure error
	tests := []struct{ name, source, build, expected string }{
		{"lean-valid", "theorem target (n : Nat) : n + 0 = n := Nat.add_zero n\n", "lean", "valid"},
		{"lean-changed-statement", "theorem target (n : Nat) : n = n := rfl\n", "lean", "valid"},
		{"lean-sorry", "theorem target (n : Nat) : n + 0 = n := by sorry\n", "lean", "valid"},
		{"lean-extra-axiom", "axiom fake (n : Nat) : n + 0 = n\ntheorem target (n : Nat) : n + 0 = n := fake n\n", "lean", "valid"},
		{"lean-truncated", "theorem target (n : Nat) : n + 0 = n := Nat.add_zero n\n", "truncated", "valid"},
		{"lean-malformed", "{\"not\":\"a certificate\"}\n", "copy", "valid"},
		{"drat-valid", "1 0\n0\n", "copy", "valid"},
		{"drat-invalid", "3 0\n0\n", "copy", "valid"},
		{"drat-other-statement", "1 0\n0\n", "copy", "valid"},
		{"product-large", "", "large", "valid"},
		{"product-writer", "", "writer", "valid"},
		{"product-missing", "", "missing", "invalid_output"},
		{"product-symlink", "", "symlink", "invalid_output"},
		{"product-oversize", "", "oversize", "invalid_output"},
	}
	for _, test := range tests {
		m := nativeProbeManifest(r.Config.RuntimeImageDigest, "Proof.lean", []string{"/usr/local/bin/python3", "-I", "build.py"})
		m.Resources.TimeoutSeconds = 240
		m.Resources.MemoryMB = 4096
		m.Evaluation.Mode = "proof"
		m.Evaluation.Executor.Features = append(m.Evaluation.Executor.Features, "sealed-products", "native-proof-checker", "asset-domains", "private-process-view")
		m.Evaluation.Program.MinRuns = 0
		m.Evaluation.Program.BuildBudget.TimeoutSeconds = 180
		m.Evaluation.Program.BuildBudget.MemoryMB = 2048
		m.Evaluation.Program.BuildBudget.MaxFileBytes = 4 << 20
		m.Evaluation.Program.BuildBudget.MaxOutputBytes = 65536
		m.Evaluation.Program.Products = []protocol.BuildProduct{{Path: "certificate", MaxBytes: 2 << 20}}
		m.Submission.AllowedPaths = []string{"Proof.lean", "build.py"}
		m.Submission.AllowedExtensions = []string{".lean", ".py"}
		m.Submission.MaxFiles = 2
		statement := []byte("theorem target (n : Nat) : n + 0 = n := by sorry\n")
		format := "lean"
		if len(test.name) >= 4 && test.name[:4] == "drat" {
			format = "drat"
			statement = []byte("p cnf 2 4\n1 2 0\n-1 2 0\n1 -2 0\n-1 -2 0\n")
		}
		if test.name == "drat-other-statement" {
			statement = []byte("p cnf 2 3\n1 2 0\n-1 2 0\n1 -2 0\n")
		}
		m.Evaluation.Proof = &protocol.ProofContract{Format: format, StatementPath: "statements/target.txt", StatementDigest: protocol.DigestBytes(statement), CertificatePath: "certificate", AllowedAxioms: []string{}, CheckDescription: "Replay fixed first-party certificate fixtures against the exact statement and an empty axiom policy."}
		for _, asset := range r.Config.Assets {
			m.Evaluation.Assets = append(m.Evaluation.Assets, asset.Asset)
		}
		if len(m.Evaluation.Assets) != 4 {
			return protocol.Envelope{}, errors.New("proof corpus requires lean, proof-tools, public-data and hidden-data assets")
		}
		if err := protocol.ValidateManifest(m); err != nil {
			return protocol.Envelope{}, err
		}
		if err := matchConfiguredEvaluation(m, r.Config); err != nil {
			return protocol.Envelope{}, err
		}
		build := fmt.Sprintf(proofCandidateBuild, test.build)
		checker := fmt.Sprintf(proofProbeChecker, test.name)
		mb, _ := json.Marshal(m)
		files := map[string]map[string][]byte{
			"submission": {"Proof.lean": []byte(test.source), "build.py": []byte(build)},
			"suite":      {"canary.txt": []byte("HIDDEN_PROOF_CANARY")},
			"validator":  {"empty.txt": []byte("Pinned proof asset supplies checker")},
			"challenge":  {"check.py": []byte(checker), "science-ladder.yaml": mb, "requirements.lock": []byte("# pinned platform runtime only\n"), "statements/target.txt": statement},
		}
		envelope, run, err := r.runProofProbeCase(ctx, diagnostics, m, files)
		passed := err == nil && run.Outcome == test.expected && run.CleanupAttested
		if test.expected == "valid" {
			passed = passed && run.Gates["isolation"]
		}
		checks = append(checks, map[string]any{"name": test.name, "passed": passed, "outcome": run.Outcome, "receipt": envelope})
		if !passed {
			failure = fmt.Errorf("proof conformance %s failed: %s (%v)", test.name, run.Outcome, err)
			break
		}
	}
	data := map[string]any{"passed": failure == nil, "checks": checks, "hostId": r.Config.HostID, "hostGroup": r.Config.HostGroup, "crossHostVerified": false, "advisoryGateSatisfied": false, "durationMillis": time.Since(started).Milliseconds(), "scope": "first-party serialized Lean replay, DRAT, asset isolation and sealed-product corpus"}
	if failure != nil {
		data["failure"] = failure.Error()
	}
	receipt := protocol.Receipt{APIVersion: protocol.APIVersion, Kind: "ProofHostConformanceReceipt", ID: fmt.Sprintf("proof-conformance-%d", started.UnixNano()), CreatedAt: time.Now().UTC(), Producer: r.Config.HostID, SubjectDigest: r.Config.ExecutionProfileDigest, EconomicMode: "none", DeploymentMode: "controlled-demo", OfficialAcceptance: false, VerificationPolicy: protocol.VerificationPlatform, Data: data}
	env, err := protocol.Sign(r.KeyID, r.Signer, receipt)
	if err != nil {
		return env, err
	}
	return env, failure
}

func (r *Runtime) runProofProbeCase(ctx context.Context, diagnostics io.Writer, m protocol.Manifest, files map[string]map[string][]byte) (protocol.Envelope, protocol.RunReceipt, error) {
	var run protocol.RunReceipt
	root, err := os.MkdirTemp(r.Config.WorkRoot, "proof-probe-")
	if err != nil {
		return protocol.Envelope{}, run, err
	}
	defer os.RemoveAll(root)
	runtime := *r
	runtime.probeDiagnostics = diagnostics
	runtime.localObjects = map[string]string{}
	refs := map[string]protocol.ObjectRef{}
	builder := Builder{MakeSquashFS: r.Config.MakeSquashFS}
	for name, tree := range files {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0755); err != nil {
			return protocol.Envelope{}, run, err
		}
		if err := writeTree(dir, tree); err != nil {
			return protocol.Envelope{}, run, err
		}
		out := filepath.Join(root, name+".squashfs")
		ref, err := builder.disk(ctx, dir, out)
		if err != nil {
			return protocol.Envelope{}, run, err
		}
		refs[name] = ref
		runtime.localObjects[ref.Digest] = out
	}
	_, digest, err := protocol.ArtifactFromFiles(files["submission"], m.Submission)
	if err != nil {
		return protocol.Envelope{}, run, err
	}
	job := protocol.RunnerJob{APIVersion: protocol.APIVersion, Kind: "ValidationJob", ID: fmt.Sprintf("proof-probe-%d", time.Now().UnixNano()), CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(6 * time.Minute), Producer: r.Config.HostID, Purpose: "preflight", DeploymentMode: "controlled-demo", OfficialAcceptance: false, VerificationPolicy: protocol.VerificationPlatform, Manifest: m, RunnerEpoch: r.Config.RunnerEpoch, ExecutionProfileDigest: r.Config.ExecutionProfileDigest, FencingToken: 1, ArtifactDigest: digest, ValidatorDisk: refs["validator"], ChallengeDisk: refs["challenge"], SuiteDisk: refs["suite"], SubmissionDisk: refs["submission"], SuiteDigest: refs["suite"].Digest}
	env, err := runtime.runJob(ctx, job)
	if err != nil {
		return env, run, err
	}
	payload, err := protocol.Verify(env, map[string]crypto.PublicKey{r.KeyID: r.Signer.Public()})
	if err == nil {
		err = protocol.DecodeStrict(payload, &run)
	}
	return env, run, err
}

const proofCandidateBuild = `import os,subprocess,time
from pathlib import Path
assert Path('/assets/public-data/canary.txt').read_text()=='public asset visible'
for p in ('/assets/proof-tools','/assets/hidden-data','/sl','/proc/sys','/proc/meminfo','/proc/1/root/sl'):
 assert not Path(p).exists(),p
assert os.getuid()==65533 and os.getpid()==1 and os.getppid()==0
assert Path('/proc/self/exe').exists()
mode=%q
if mode in ('lean','truncated'):
 env=dict(os.environ,LEAN_PATH='/work:/assets/lean/lean/lib/lean',LEAN_SYSROOT='/assets/lean/lean',LD_LIBRARY_PATH='/assets/lean/lean/lib/lean',PATH='/assets/lean/lean/bin:/usr/local/bin:/usr/bin:/bin')
 subprocess.run(['/assets/lean/lean/bin/lean','-o','Proof.olean','Proof.lean'],env=env,check=True)
 with open('certificate','wb') as out:
  subprocess.run(['/assets/lean/bin/lean4export','Proof','--','target','Nat.add','Nat.sub','Nat.mul','Nat.pow','Nat.gcd','Nat.div','Nat.mod','Nat.beq','Nat.ble','Nat.land','Nat.lor','Nat.xor','Nat.shiftLeft','Nat.shiftRight','String.ofList','Char.ofNat','List','eagerReduce'],env=env,stdout=out,check=True)
 if mode=='truncated':
  with open('certificate','r+b') as f:f.truncate(Path('certificate').stat().st_size-32)
elif mode=='copy':Path('certificate').write_bytes(Path('Proof.lean').read_bytes())
elif mode in ('large','writer'):
 Path('certificate').write_bytes(b'x'*(1024*1024+17))
 if mode=='writer':
  rd,wr=os.pipe()
  if os.fork()==0:
   os.close(rd);f=open('certificate','r+b');os.write(wr,b'r');os.close(wr)
   for fd in (0,1,2):os.close(fd)
   time.sleep(1);f.write(b'forged');f.flush();os._exit(0)
  os.close(wr);assert os.read(rd,1)==b'r';os.close(rd)
elif mode=='symlink':Path('certificate').symlink_to('Proof.lean')
elif mode=='oversize':Path('certificate').write_bytes(b'x'*(2*1024*1024+1))
`

const proofProbeChecker = `import base64,json,socket,subprocess,os,time
from pathlib import Path
def call(action):
 s=socket.socket(socket.AF_UNIX);s.connect('/sl/broker/control.sock');s.sendall(json.dumps({'action':action}).encode());s.shutdown(socket.SHUT_WR);parts=[]
 while True:
  b=s.recv(65536)
  if not b:break
  parts.append(b)
 return json.loads(b''.join(parts))
name=%q
build=call('build');print('build',build,flush=True)
passed=build['outcome']=='valid' and not os.access('/opt/sl-private/assets/lean',os.R_OK) and Path('/sl/assets/hidden-data/canary.txt').read_text()=='hidden asset canary'
if passed:
 certificate=Path('/sl/products/certificate')
 passed=passed and bool(os.statvfs(certificate).f_flag & os.ST_RDONLY) and bool(os.statvfs(certificate).f_flag & os.ST_NOEXEC)
 try:
  certificate.write_bytes(b'forged');passed=False
 except OSError:pass
 if name in ('product-large','product-writer'):
  if name=='product-writer':time.sleep(2)
  passed=passed and certificate.read_bytes()==b'x'*(1024*1024+17)
 else:
  if name.startswith('lean'):
   args=['/sl/assets/proof-tools/sl-lean-check','/sl/assets/proof-tools/policy.json','/sl/assets/proof-tools/reference.ndjson',str(certificate)]
  else:args=['/sl/assets/proof-tools/drat-trim','/sl/challenge/statements/target.txt',str(certificate)]
  checked=subprocess.run(args,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=120)
  print('checker',checked.returncode,checked.stdout[:8192].decode(errors='replace'),flush=True)
  expected=name in ('lean-valid','drat-valid')
  passed=passed and ((checked.returncode==0)==expected)
Path('/sl/output/result.json').write_text(json.dumps({'apiVersion':'science-ladder/v2','kind':'ValidatorResult','comparisonId':'internal-native-conformance-v1','score':'1' if passed else '0','measurements':{'checks':'1' if passed else '0'},'gates':{'isolation':passed}}))
`
