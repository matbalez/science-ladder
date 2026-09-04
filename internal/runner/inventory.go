package runner

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

// This is a fixed platform recipe, run only against the operator-approved,
// digest-pinned interpreter image. No challenge source is mounted or executed.
const inventoryScript = `import hashlib, importlib.metadata, json, pathlib, re, sys
components = pathlib.Path('/usr/share/science-ladder/runtime-components.json')
if components.exists():
    raw = components.read_bytes()
    document = json.loads(raw)
    print(json.dumps({'apiVersion':'science-ladder-runtime-inventory/v1','runtimeImageDigest':sys.argv[1],'componentInventoryDigest':'sha256:'+hashlib.sha256(raw).hexdigest(),'packages':document['packages']},separators=(',',':')))
    sys.exit(0)
packages = []
for paragraph in pathlib.Path('/var/lib/dpkg/status').read_text().split('\n\n'):
    fields = {}
    for line in paragraph.splitlines():
        if line and not line[0].isspace() and ': ' in line:
            key, value = line.split(': ', 1)
            fields[key] = value
    if fields.get('Status') == 'install ok installed':
        packages.append({'ecosystem':'Debian','name':fields['Package'],'version':fields['Version']})
for distribution in importlib.metadata.distributions():
    packages.append({'ecosystem':'PyPI','name':distribution.metadata['Name'],'version':distribution.version})
    if distribution.metadata['Name'].lower() == 'pip':
        vendor_file = pathlib.Path(distribution.locate_file('pip/_vendor/vendor.txt'))
        for line in vendor_file.read_text().splitlines():
            line = line.split('#',1)[0].strip()
            if not line:
                continue
            match = re.fullmatch(r'([A-Za-z0-9_.-]+)==([A-Za-z0-9_.+-]+)',line)
            if not match:
                raise ValueError('unrecognized pip vendored dependency inventory')
            packages.append({'ecosystem':'PyPI','name':match[1],'version':match[2]})
packages.append({'ecosystem':'CPython','name':'cpython','version':'.'.join(map(str,sys.version_info[:3])), 'digest':'sha256:'+hashlib.sha256(pathlib.Path(sys.executable).read_bytes()).hexdigest()})
print(json.dumps({'apiVersion':'science-ladder-runtime-inventory/v1','runtimeImageDigest':sys.argv[1],'packages':packages},separators=(',',':')))
`

func InventoryRuntime(ctx context.Context, reference string) (RuntimeInventory, error) {
	var inventory RuntimeInventory
	parts := strings.Split(reference, "@")
	if len(parts) != 2 || parts[0] == "" || !protocol.ValidDigest(parts[1]) || strings.ContainsAny(reference, " \n\r\t") {
		return inventory, errors.New("runtime inventory requires an approved digest-pinned OCI image")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	output := &boundedBuffer{max: 1 << 20}
	logs := &boundedBuffer{max: 8192}
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--pids-limit=32", "--memory=256m", "--cpus=1", "--user=65534:65534", "--platform=linux/amd64", "--entrypoint=/usr/local/bin/python3", reference, "-I", "-c", inventoryScript, parts[1])
	command.Stdout = output
	command.Stderr = logs
	if err := command.Run(); err != nil {
		return inventory, errors.New("fixed runtime inventory recipe failed")
	}
	if output.overflow {
		return inventory, errors.New("runtime inventory exceeds limit")
	}
	if err := protocol.DecodeStrict(output.b.Bytes(), &inventory); err != nil {
		return inventory, err
	}
	packages, err := DependencyInventory(map[string][]byte{"requirements.lock": {}}, "requirements.lock", inventory)
	if err != nil {
		return inventory, err
	}
	inventory.Packages = packages
	return inventory, nil
}
