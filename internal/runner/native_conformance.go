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
	"strings"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

// NativeHardwareProbe accepts no user-supplied source. It exercises fixed hostile
// candidate programs inside the new boundary without claiming advisory approval,
// external security review, scientific progress or competitive acceptance.
func (r *Runtime) NativeHardwareProbe(ctx context.Context, diagnostics io.Writer) (protocol.Envelope, error) {
	if err := r.Config.CheckHost(r.Keys); err != nil {
		return protocol.Envelope{}, err
	}
	if r.Signer == nil || r.Config.Capabilities == nil {
		return protocol.Envelope{}, errors.New("native probe requires enrolled capabilities and host signer")
	}
	workspace, err := os.MkdirTemp(r.Config.WorkRoot, "native-hardware-probe-")
	if err != nil {
		return protocol.Envelope{}, err
	}
	defer os.RemoveAll(workspace)
	start := time.Now().UTC()
	runtime := *r
	runtime.probeDiagnostics = diagnostics
	checks := []map[string]any{}
	var probeErr error
	type probeCase struct {
		name, filename, source string
		build                  []string
		cases                  string
	}
	tests := []probeCase{
		{"candidate-isolation", "probe.c", nativeIsolationC, []string{"/usr/bin/gcc", "-O2", "probe.c", "-o", "/work/probe"}, `[(b"isolation", "valid", b"isolated\n"), (b"memory", "resource_limit", None), (b"timeout", "resource_limit", None), (b"output", "output_limit", None), (b"descendant", "valid", b"done\n")]`},
		{"cpp-toolchain", "probe.cpp", "#include <iostream>\n#include <Eigen/Dense>\nint main(){Eigen::Matrix2d a; a<<2,1,1,2; std::cout<<a.determinant()<<'\\n';}\n", []string{"/usr/bin/g++", "-O2", "-I/usr/include/eigen3", "probe.cpp", "-o", "/work/probe"}, `[(b"", "valid", b"3\n")]`},
		{"rust-toolchain", "probe.rs", "fn main(){let h=std::thread::spawn(|| 6*7); println!(\"{}\",h.join().unwrap());}\n", []string{"/usr/bin/rustc", "-O", "probe.rs", "-o", "/work/probe"}, `[(b"", "valid", b"42\n")]`},
		{"python-numerics", "probe.py", "import numpy as np; from scipy.sparse import csc_matrix; from scipy.sparse.linalg import spsolve; print(round(float(np.sum(spsolve(csc_matrix([[2.,1.],[1.,2.]]),np.array([3.,3.]))))))\n", []string{"/usr/local/bin/python3", "-I", "-c", "import py_compile; py_compile.compile('probe.py', cfile='/work/probe.pyc', doraise=True)"}, `[(b"", "valid", b"2\n")]`},
		{"paired-timing", "probe.c", nativeTimingCandidate, []string{"/usr/bin/gcc", "-O2", "probe.c", "-o", "/work/probe"}, ""},
	}
	for _, feature := range r.Config.Capabilities.Features {
		if feature == "proof-timing-composition" {
			tests = append(tests, probeCase{"paired-timing-proof", "probe.c", nativeTimingCandidate, []string{"/usr/bin/gcc", "-O2", "probe.c", "-o", "/work/probe"}, ""})
		}
	}
	for _, test := range tests {
		root := filepath.Join(workspace, test.name)
		if err := os.Mkdir(root, 0700); err != nil {
			return protocol.Envelope{}, err
		}
		m := nativeProbeManifest(r.Config.RuntimeImageDigest, test.filename, test.build)
		privateProc := false
		for _, feature := range r.Config.Capabilities.Features {
			if feature == "private-process-view" {
				privateProc = true
				m.Evaluation.Executor.Features = append(m.Evaluation.Executor.Features, feature)
			}
		}
		if privateProc && test.name == "candidate-isolation" {
			test.source = "#define PRIVATE_PROC 1\n" + test.source
		}
		if test.name == "python-numerics" {
			m.Evaluation.Program.Run = []string{"/usr/local/bin/python3", "-I", "/work/probe.py"}
			m.Evaluation.Program.RunBudget.MemoryMB = 512
		}
		if strings.HasPrefix(test.name, "paired-timing") {
			m = nativeTimingManifest(m, r.Config.Capabilities.HardwareClass)
		}
		if test.name == "paired-timing-proof" {
			m.Evaluation.Executor.Features = append(m.Evaluation.Executor.Features, "native-proof-checker", "sealed-products", "proof-timing-composition")
			m.Evaluation.Proof = &protocol.ProofContract{Format: "drat", StatementPath: "statements/target.cnf", StatementDigest: protocol.DigestBytes([]byte(nativeProofFormula)), CertificatePath: "certificate", AllowedAxioms: []string{}, CheckDescription: "Replay the exact fixed CNF certificate before accepting paired quality measurements; this conformance does not assert that the formula proves program semantics."}
			m.Evaluation.Program.Products = []protocol.BuildProduct{{Path: "certificate", MaxBytes: 1024}}
			m.Evaluation.Program.Build = []string{"/usr/local/bin/python3", "-I", "-c", "import subprocess; from pathlib import Path; subprocess.run(['/usr/bin/gcc','-O2','probe.c','-o','/work/probe'],check=True); Path('/work/certificate').write_text('1 0\\n0\\n')"}
			for _, asset := range r.Config.Assets {
				if asset.Asset.Name == "proof-tools" {
					m.Evaluation.Assets = append(m.Evaluation.Assets, asset.Asset)
					m.Evaluation.Executor.Features = append(m.Evaluation.Executor.Features, "asset-domains")
				}
			}
		}
		if err := protocol.ValidateManifest(m); err != nil {
			return protocol.Envelope{}, err
		}
		if err := matchConfiguredEvaluation(m, r.Config); err != nil {
			return protocol.Envelope{}, err
		}
		mBytes, _ := json.Marshal(m)
		files := map[string]map[string][]byte{
			"submission": {test.filename: []byte(test.source)},
			"suite":      {"canary.txt": []byte("HIDDEN_NATIVE_BOUNDARY_CANARY")},
			"validator":  {"empty.txt": []byte("No third-party checker dependencies")},
			"challenge":  {"check.py": []byte(fmt.Sprintf(nativeProbeChecker, test.cases)), "science-ladder.yaml": mBytes, "requirements.lock": []byte("# pinned platform tools only\n")},
		}
		if test.name == "python-numerics" {
			files["challenge"]["check.py"] = append([]byte("import numpy; import scipy.sparse.linalg\n"), files["challenge"]["check.py"]...)
		}
		if strings.HasPrefix(test.name, "paired-timing") {
			files["challenge"]["check.py"] = []byte(nativeTimingChecker)
			files["challenge"]["baseline/source/probe.c"] = []byte(nativeTimingBaseline)
		}
		if test.name == "paired-timing-proof" {
			files["challenge"]["statements/target.cnf"] = []byte(nativeProofFormula)
			check := strings.Replace(nativeTimingChecker, "if passed:\n for i in range(11):", "if passed:\n passed=subprocess.run(['/sl/assets/proof-tools/drat-trim','/sl/challenge/statements/target.cnf','/sl/products/certificate'],stdout=subprocess.DEVNULL).returncode==0\nif passed:\n for i in range(11):", 1)
			files["challenge"]["check.py"] = []byte("import subprocess\n" + check)
		}
		b := Builder{MakeSquashFS: r.Config.MakeSquashFS}
		refs := map[string]protocol.ObjectRef{}
		runtime.localObjects = map[string]string{}
		for name, tree := range files {
			dir := filepath.Join(root, name)
			if err := os.Mkdir(dir, 0755); err != nil {
				return protocol.Envelope{}, err
			}
			if err := writeTree(dir, tree); err != nil {
				return protocol.Envelope{}, err
			}
			out := filepath.Join(root, name+".squashfs")
			ref, err := b.disk(ctx, dir, out)
			if err != nil {
				return protocol.Envelope{}, err
			}
			refs[name] = ref
			runtime.localObjects[ref.Digest] = out
		}
		_, artifactDigest, err := protocol.ArtifactFromFiles(files["submission"], m.Submission)
		if err != nil {
			return protocol.Envelope{}, err
		}
		job := protocol.RunnerJob{APIVersion: protocol.APIVersion, Kind: "ValidationJob", ID: fmt.Sprintf("native-probe-%d", time.Now().UnixNano()), CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(5 * time.Minute), Producer: r.Config.HostID, Purpose: "preflight", DeploymentMode: "controlled-demo", OfficialAcceptance: false, VerificationPolicy: protocol.VerificationPlatform, Manifest: m, RunnerEpoch: r.Config.RunnerEpoch, ExecutionProfileDigest: r.Config.ExecutionProfileDigest, FencingToken: 1, ArtifactDigest: artifactDigest, ValidatorDisk: refs["validator"], ChallengeDisk: refs["challenge"], SuiteDisk: refs["suite"], SubmissionDisk: refs["submission"], SuiteDigest: refs["suite"].Digest}
		envelope, runErr := runtime.runJob(ctx, job)
		var run protocol.RunReceipt
		if runErr == nil {
			payload, e := protocol.Verify(envelope, map[string]crypto.PublicKey{r.KeyID: r.Signer.Public()})
			if e != nil {
				runErr = e
			} else {
				runErr = protocol.DecodeStrict(payload, &run)
			}
		}
		passed := runErr == nil && run.Outcome == "valid" && run.Gates["isolation"] && run.CleanupAttested
		if strings.HasPrefix(test.name, "paired-timing") {
			passed = passed && run.ValidatorResult != nil && run.ValidatorResult.Timing != nil && protocol.ValidateRunMeasurementEvidence(run, m) == nil
		}
		checks = append(checks, map[string]any{"name": test.name, "passed": passed, "outcome": run.Outcome, "receipt": envelope})
		if !passed {
			probeErr = fmt.Errorf("native conformance %s failed: %s (%v)", test.name, run.Outcome, runErr)
			break
		}
	}
	if err := os.RemoveAll(workspace); err != nil {
		return protocol.Envelope{}, errors.New("native probe cleanup failed")
	}
	data := map[string]any{"hostId": r.Config.HostID, "hostGroup": r.Config.HostGroup, "passed": probeErr == nil, "checks": checks, "crossHostVerified": false, "advisoryGateSatisfied": false, "cleanupAttested": true, "durationMillis": time.Since(start).Milliseconds(), "scope": "fixed first-party native candidate/judge isolation, resource and C++/Rust toolchain corpus"}
	if probeErr != nil {
		data["failure"] = probeErr.Error()
	}
	receipt := protocol.Receipt{APIVersion: protocol.APIVersion, Kind: "NativeHostConformanceReceipt", ID: fmt.Sprintf("native-host-conformance-%d", start.UnixNano()), CreatedAt: time.Now().UTC(), Producer: r.Config.HostID, SubjectDigest: r.Config.ExecutionProfileDigest, EconomicMode: "none", DeploymentMode: "controlled-demo", OfficialAcceptance: false, VerificationPolicy: protocol.VerificationPlatform, Data: data}
	envelope, err := protocol.Sign(r.KeyID, r.Signer, receipt)
	if err != nil {
		return envelope, err
	}
	return envelope, probeErr
}

func nativeProbeManifest(digest, filename string, build []string) protocol.Manifest {
	m := hostProbeManifest(digest)
	m.APIVersion = protocol.ManifestV2
	m.Validator.Profile = "native-evaluator-v2"
	m.Validator.Entrypoint = []string{"/usr/local/bin/python3", "/sl/challenge/check.py"}
	m.Metric.Name = "checks"
	m.Metric.Unit = "passed checks"
	m.Resources.MemoryMB = 2048
	m.Resources.TimeoutSeconds = 120
	m.Submission = protocol.SubmissionContract{Format: "source-v2", AllowedPaths: []string{filename}, AllowedExtensions: []string{filepath.Ext(filename)}, MaxBytes: 65536, MaxFiles: 1, License: "MIT"}
	m.Evaluation = &protocol.EvaluationContract{Version: protocol.EvaluationVersion, Mode: "program", ComparisonID: "internal-native-conformance-v1", Executor: protocol.ExecutorRequirements{OS: "linux", Architecture: "amd64", Accelerator: "none", Features: []string{"isolated-checker", "isolated-candidate", "separated-toolchain"}}, Measurements: []protocol.MeasurementDefinition{{Name: "checks", Type: "integer", Unit: m.Metric.Unit, Role: "primary", Interpretation: "direct", Definition: "Fixed first-party execution-boundary checks, never scientific progress.", Minimum: "0", Maximum: "1"}}, Rationale: protocol.MetricRationale{Objective: "Exercise a first-party platform security test, with no scientific claim.", ImprovementMeaning: "Passing means only that the fixed probes returned the expected results.", EvidenceURLs: []string{m.Evidence[0].URL}, PreservedConditions: []string{"Candidate source must not access checker state or hidden answer files."}, BaselineReason: "This is a conformance test with deliberately hostile program behavior.", MeaningfulDelta: "One passed fixed test is not a quantified security assurance level.", ProxyAttacks: []string{"Forged candidate stdout must never become an authoritative score frame."}, PermittedClaim: "These first-party conformance cases passed on the stated runtime.", ExcludedClaims: []string{"No external security review or scientific result is implied by this probe."}}, Program: &protocol.CandidateProgram{Build: build, Run: []string{"/work/probe"}, BuildBudget: protocol.StageBudget{TimeoutSeconds: 60, MemoryMB: 1024, MaxOutputBytes: 64 << 20, MaxProcesses: 64}, RunBudget: protocol.StageBudget{TimeoutSeconds: 2, MemoryMB: 64, MaxOutputBytes: 4096, MaxProcesses: 16}, MinRuns: 1, MaxRuns: 8, ScratchMB: 128}}
	return m
}

const nativeProbeChecker = `import base64, json, socket, os
from pathlib import Path
def call(action, data=b''):
    s=socket.socket(socket.AF_UNIX); s.connect('/sl/broker/control.sock')
    s.sendall(json.dumps({'action':action,'input':base64.b64encode(data).decode()}).encode()); s.shutdown(socket.SHUT_WR)
    chunks=[]
    while True:
        chunk=s.recv(65536)
        if not chunk: break
        chunks.append(chunk)
    s.close(); return json.loads(b''.join(chunks))
build=call('build'); print('build',build,flush=True)
passed=build['outcome']=='valid' and not os.access('/opt/sl-private/toolchain/usr',os.R_OK) and not Path('/usr/bin/gcc').exists() and not Path('/usr/bin/rustc').exists()
if passed:
    for payload,outcome,stdout in %s:
        response=call('run',payload); print('case',payload,response,flush=True)
        passed=passed and response['outcome']==outcome
        if stdout is not None: passed=passed and base64.b64decode(response.get('stdout',''))==stdout
Path('/sl/output/result.json').write_text(json.dumps({'apiVersion':'science-ladder/v2','kind':'ValidatorResult','comparisonId':'internal-native-conformance-v1','score':'1' if passed else '0','measurements':{'checks':'1' if passed else '0'},'gates':{'isolation':passed}}))
`

const nativeIsolationC = `#define _GNU_SOURCE
#if __has_include("/sl/suite/canary.txt")
#error Candidate compiler can read suite
#endif
#if __has_include("/sl/challenge/check.py")
#error Candidate compiler can read checker
#endif
#include <errno.h>
#include <fcntl.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ptrace.h>
#include <sys/socket.h>
#include <unistd.h>
int main(void){
 char b[32]={0};if(read(0,b,31)<0)return 2;
 if(!strcmp(b,"memory")){while(1){volatile char *p=malloc(8*1024*1024);if(!p)return 3;for(int i=0;i<8*1024*1024;i++)p[i]=1;}}
 if(!strcmp(b,"timeout")){for(;;)asm volatile("":::"memory");}
 if(!strcmp(b,"output")){for(int i=0;i<20000;i++)puts("overflow");return 0;}
 if(!strcmp(b,"descendant")){if(fork()==0){close(0);close(1);close(2);setsid();for(;;)pause();}puts("done");return 0;}
 if(getuid()!=65533 || geteuid()!=65533 || getgroups(0,NULL)!=0)return 10;
 #ifdef PRIVATE_PROC
 if(getpid()!=1 || getppid()!=0)return 16;
 char exe[4096];int n=readlink("/proc/self/exe",exe,sizeof(exe));if(n<1)return 17;
 const char *forbidden[]={"/proc/sys","/proc/meminfo","/proc/uptime","/proc/1/root/sl",
#else
 const char *forbidden[]={"/proc",
#endif
"/sl/suite/canary.txt","/sl/challenge/check.py","/sl/broker/control.sock","/sl/output/result.json"};
 for(unsigned i=0;i<sizeof(forbidden)/sizeof(*forbidden);i++){if(access(forbidden[i],F_OK)==0)return 11;}
 if(socket(AF_INET,SOCK_STREAM,0)>=0 || errno!=EPERM)return 12;
 if(unshare(CLONE_NEWUSER)==0 || errno!=EPERM)return 13;
 if(ptrace(PTRACE_TRACEME,0,0,0)==0 || errno!=EPERM)return 14;
 if(open("/sl/output/result.json",O_WRONLY|O_CREAT,0644)>=0)return 15;
 puts("isolated");return 0;
}
`

func nativeTimingManifest(m protocol.Manifest, hardware string) protocol.Manifest {
	m.Evaluation.Mode = "performance"
	m.Evaluation.Executor.HardwareClass = hardware
	m.Evaluation.Executor.Features = append(m.Evaluation.Executor.Features, "trusted-timing")
	m.Evaluation.Program.MinRuns = 22
	m.Evaluation.Program.MaxRuns = 22
	m.Metric.Name = "speedup"
	m.Metric.Unit = "baseline / candidate"
	m.Metric.Quantum = "0.000001"
	m.Metric.BaselineTicks = "1000000"
	for i := range m.Milestones {
		m.Milestones[i].ThresholdTicks = "2000000"
	}
	for i := range m.Fixtures {
		m.Fixtures[i].ExpectedTicks = ""
	}
	m.Evaluation.Measurements = []protocol.MeasurementDefinition{{Name: m.Metric.Name, Type: "rational", Role: "primary", Unit: m.Metric.Unit, Interpretation: "direct", Definition: "Conservative paired timing bound for a fixed first-party execution test."}}
	_, digest, _ := protocol.ArtifactFromFiles(map[string][]byte{"probe.c": []byte(nativeTimingBaseline)}, m.Submission)
	m.Evaluation.Measurement = &protocol.MeasurementPolicy{Estimator: "paired-median-ratio", Warmups: 2, Repetitions: 9, Order: "alternating", ConfidencePPM: 950000, MaxRelativeWidth: "1", MinimumSpeedup: "1.01", BaselineDigest: digest, BaselinePath: "baseline/source", BaselineBuild: []string{"/usr/bin/gcc", "-O2", "probe.c", "-o", "/work/probe"}, BaselineRun: []string{"/work/probe"}, TimerBoundary: "Root broker process start through complete output and descendant cleanup.", Population: "Fixed first-party CPU loop conformance only, with no scientific speedup claim."}
	return m
}

const nativeTimingBaseline = "#include <stdio.h>\nint main(){volatile unsigned long x=0;for(unsigned i=0;i<100000000;i++)x+=i;puts(\"42\");}\n"
const nativeTimingCandidate = "#include <stdio.h>\nint main(){volatile unsigned long x=0;for(unsigned i=0;i<50000000;i++)x+=i;puts(\"42\");}\n"
const nativeTimingChecker = `import base64,json,socket,os
from pathlib import Path
def call(request):
 s=socket.socket(socket.AF_UNIX);s.connect('/sl/broker/control.sock');s.sendall(json.dumps(request).encode());s.shutdown(socket.SHUT_WR);parts=[]
 while True:
  data=s.recv(65536)
  if not data:break
  parts.append(data)
 s.close();return json.loads(b''.join(parts))
build=call({'action':'build'});passed=build['outcome']=='valid' and not os.access('/opt/sl-private/toolchain/usr',os.R_OK) and not Path('/usr/bin/gcc').exists() and not Path('/usr/bin/rustc').exists();print('build',build,flush=True)
if passed:
 for i in range(11):
  response=call({'action':'pair','input':base64.b64encode(b'fixed input').decode()})
  if response['outcome']!='valid':passed=False;break
  pair=response['pair'];quality=all(pair[side]['outcome']=='valid' and base64.b64decode(pair[side].get('stdout',''))==b'42\n' for side in ('baseline','candidate'))
  print('pair',i,pair,flush=True)
  passed=passed and quality and call({'action':'assess','qualityPassed':quality})['outcome']=='valid'
Path('/sl/output/result.json').write_text(json.dumps({'apiVersion':'science-ladder/v2','kind':'ValidatorResult','comparisonId':'internal-native-conformance-v1','score':'0/1','measurements':{'speedup':'0/1'},'gates':{'isolation':passed}}))
`

const nativeProofFormula = "p cnf 2 4\n1 2 0\n-1 2 0\n1 -2 0\n-1 -2 0\n"
