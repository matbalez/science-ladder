#!/usr/bin/env python3
"""Build a four-platform, source-bound CLI release with SHA-256 checksums."""
import hashlib, json, os, pathlib, re, subprocess, sys
version = sys.argv[1]
if not re.fullmatch(r'\d+\.\d+\.\d+', version):
    raise SystemExit('Use a release version such as 0.3.0')
root = pathlib.Path(__file__).resolve().parents[1]
commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
out = pathlib.Path(sys.argv[2]).resolve()
out.mkdir(parents=True, exist_ok=True)
checksums = []
for platform in ('darwin', 'linux'):
    for arch in ('amd64', 'arm64'):
        name = f'sl_{platform}_{arch}'
        subprocess.run(['go', 'build', '-trimpath', '-ldflags',
                        f'-s -w -X main.cliVersion={version} -X main.cliCommit={commit}',
                        '-o', str(out/name), './cmd/sl'], cwd=root,
                       env={**os.environ, 'CGO_ENABLED': '0', 'GOOS': platform, 'GOARCH': arch}, check=True)
        checksums.append(f'{hashlib.sha256((out/name).read_bytes()).hexdigest()}  {name}')
(out/'SHA256SUMS').write_text('\n'.join(checksums)+'\n')
(out/'source.json').write_text(json.dumps({'version': version, 'commit': commit, 'license': 'MIT'}, indent=2)+'\n')
print(out)
