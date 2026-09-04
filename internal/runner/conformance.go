package runner

import (
	"context"
	"crypto"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

// isolationCorpus executes fixed platform-owned hostile probes inside the same
// checker boundary as science validators. It cannot prove absence of escapes;
// external security review remains a separate release gate.
func (b *Builder) isolationCorpus(ctx context.Context, parent protocol.RunnerJob, outputRoot string) (bool, error) {
	root, err := os.MkdirTemp(outputRoot, "isolation-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(root)
	m := isolationManifest(parent.Manifest)
	artifactFiles := map[string][]byte{"data/input.txt": []byte("INTERNAL_CONFORMANCE_ONLY")}
	_, artifactDigest, _ := protocol.ArtifactFromFiles(artifactFiles, m.Submission)
	artifactRoot := filepath.Join(root, "artifact")
	suiteRoot := filepath.Join(root, "suite")
	validatorRoot := filepath.Join(root, "validator")
	for _, directory := range []string{artifactRoot, suiteRoot, validatorRoot} {
		if err := os.Mkdir(directory, 0755); err != nil {
			return false, err
		}
	}
	if err := writeTree(artifactRoot, artifactFiles); err != nil {
		return false, err
	}
	if err := writeTree(suiteRoot, map[string][]byte{"canary.txt": []byte("SL_HIDDEN_OUTPUT_CANARY_NOT_FOR_PUBLIC_OUTPUT")}); err != nil {
		return false, err
	}
	objects := map[string]string{}
	makeDisk := func(name, source string) (protocol.ObjectRef, error) {
		filename := filepath.Join(root, name+".squashfs")
		ref, err := b.disk(ctx, source, filename)
		if err == nil {
			objects[ref.Digest] = filename
		}
		return ref, err
	}
	artifactDisk, err := makeDisk("submission", artifactRoot)
	if err != nil {
		return false, err
	}
	suiteDisk, err := makeDisk("suite", suiteRoot)
	if err != nil {
		return false, err
	}
	validatorDisk, err := makeDisk("validator", validatorRoot)
	if err != nil {
		return false, err
	}
	for index, test := range []struct{ name, code, outcome string }{{"isolation", isolationProbe, "valid"}, {"cpu-limit", "while True: pass\n", "resource_limit"}, {"memory-limit", "chunks = []\nwhile True: chunks.append(bytearray(16 * 1024 * 1024))\n", "resource_limit"}, {"invalid-output", "from pathlib import Path\nPath('/sl/output/result.json').write_text('{\"score\": \"NaN\"}')\n", "invalid_output"}, {"output-bound", outputProbe, "valid"}} {
		source := filepath.Join(root, fmt.Sprintf("probe-%d", index))
		if err := os.Mkdir(source, 0755); err != nil {
			return false, err
		}
		manifestJSON, _ := json.Marshal(m)
		files := map[string][]byte{"probe.py": []byte(test.code), "science-ladder.yaml": manifestJSON, "requirements.lock": []byte("# platform fixture\n"), "suite/canary.txt": []byte("SL_HIDDEN_OUTPUT_CANARY_NOT_FOR_PUBLIC_OUTPUT")}
		if err := writeTree(source, files); err != nil {
			return false, err
		}
		challengeDisk, err := makeDisk(fmt.Sprintf("challenge-%d", index), source)
		if err != nil {
			return false, err
		}
		var outcome string
		var gates map[string]bool
		if b.UnsafeLocal {
			report, runErr := LocalValidate(ctx, m, source, artifactRoot, true)
			if runErr != nil && report.Outcome == "infrastructure_fault" {
				return false, runErr
			}
			outcome = report.Outcome
			gates = report.Gates
		} else {
			child := parent
			child.ParentJobDigest, err = protocol.Digest(parent)
			if err != nil {
				return false, err
			}
			child.ID = fmt.Sprintf("isolation-%d-%d", time.Now().UnixNano(), index)
			child.Purpose = "preflight"
			child.Manifest = m
			child.ArtifactDigest = artifactDigest
			child.ValidatorDisk = validatorDisk
			child.SubmissionDisk = artifactDisk
			child.SuiteDisk = suiteDisk
			child.ChallengeDisk = challengeDisk
			child.SuiteDigest = suiteDisk.Digest
			runRuntime := *b.Runtime
			runRuntime.localObjects = objects
			envelope, err := runRuntime.runJob(ctx, child)
			if err != nil {
				return false, err
			}
			payload, err := protocol.Verify(envelope, map[string]crypto.PublicKey{runRuntime.KeyID: runRuntime.Signer.Public()})
			if err != nil {
				return false, err
			}
			var receipt protocol.RunReceipt
			if err := protocol.DecodeStrict(payload, &receipt); err != nil {
				return false, err
			}
			outcome = receipt.Outcome
			gates = receipt.Gates
		}
		if outcome != test.outcome || outcome == "valid" && !gates["isolation"] {
			return false, fmt.Errorf("hostile corpus %s failed: expected %s, got %s", test.name, test.outcome, outcome)
		}
	}
	return true, nil
}

// The fixed corpus has its own coherent manifest, including fixture expectations.
// Inherit only the runtime image being checked, never a creator's scores, suite,
// resource budget, or fixtures. The signed child still binds the parent job.
func isolationManifest(parent protocol.Manifest) protocol.Manifest {
	return hostProbeManifest(parent.Validator.RuntimeImageDigest)
}

const isolationProbe = `import json, os, signal, socket, time
from pathlib import Path
checks = []
for address in [('169.254.169.254', 80), ('1.1.1.1', 53)]:
    try:
        connection = socket.create_connection(address, timeout=0.2)
        connection.close()
        checks.append(False)
    except OSError:
        checks.append(True)
for filename in ['/var/run/docker.sock', '/run/containerd/containerd.sock', '/root/.aws/credentials', '/root/.ssh/id_rsa']:
    checks.append(not Path(filename).exists())
for filename in ['/sl/challenge/escape.txt', '/sl/submission/escape.txt', '/sl/suite/escape.txt']:
    try:
        Path(filename).write_text('must fail')
        checks.append(False)
    except OSError:
        checks.append(True)
checks.append(os.geteuid() == 65534)
status = Path('/proc/self/status').read_text()
checks.append(any(line.startswith('CapEff:') and int(line.split()[1], 16) == 0 for line in status.splitlines()))
checks.append(not any(name in os.environ for name in ['AWS_SECRET_ACCESS_KEY', 'DATABASE_URL', 'SL_API_TOKEN', 'GITHUB_PRIVATE_KEY']))
children = []
denied = False
try:
    for _ in range(80):
        try:
            pid = os.fork()
            if pid == 0:
                time.sleep(10)
                os._exit(0)
            children.append(pid)
        except OSError:
            denied = True
            break
finally:
    for child in children:
        os.kill(child, signal.SIGKILL)
    for child in children:
        os.waitpid(child, 0)
checks.append(denied)
try:
    with open('/sl/work/flood', 'wb') as file:
        file.write(b'x' * 70000)
        file.flush()
    checks.append(False)
except OSError:
    checks.append(True)
Path('/sl/output/result.json').write_text(json.dumps({'apiVersion':'science-ladder/v1','kind':'ValidatorResult','score':'0','gates':{'isolation':all(checks)}}))
`

const outputProbe = `import json
from pathlib import Path
print('SL_HIDDEN_OUTPUT_CANARY_NOT_FOR_PUBLIC_OUTPUT' * 10000)
Path('/sl/output/result.json').write_text(json.dumps({'apiVersion':'science-ladder/v1','kind':'ValidatorResult','score':'0','gates':{'isolation':True}}))
`
