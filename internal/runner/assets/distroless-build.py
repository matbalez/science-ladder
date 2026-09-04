"""Fixed platform recipe: copy CPython and its actual ELF dependency closure.

Input is the operator-approved immutable official Python image. No creator input
is read, no dependency downloads occur, and all upstream copyright files remain.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

destination = Path(sys.argv[1])
destination.mkdir(parents=True, exist_ok=False)
package_rows = subprocess.run([
    '/usr/bin/dpkg-query', '-W',
    '-f=${binary:Package}\t${Version}\t${source:Package}\t${source:Version}\n',
], check=True, capture_output=True, text=True).stdout.splitlines()
packages = {}
for row in package_rows:
    binary, version, source, source_version = row.split('\t')
    packages[binary.split(':', 1)[0]] = {
        'ecosystem': 'Debian', 'name': binary.split(':', 1)[0], 'version': version,
        'sourceName': source, 'sourceVersion': source_version,
    }

copied = set()
owned = {}
needed = {}
excluded_unavailable = []

def owner(path):
    generated = {'/etc/ssl/certs/ca-certificates.crt':'ca-certificates', '/etc/passwd':'base-passwd', '/etc/group':'base-passwd', '/etc/nsswitch.conf':'libc-bin'}
    if str(path) in generated:
        return generated[str(path)]
    for candidate in [str(path), str(path.resolve()), str(path).replace('/usr/lib/', '/lib/', 1)]:
        result = subprocess.run(['/usr/bin/dpkg-query', '-S', candidate], capture_output=True, text=True)
        if result.returncode == 0:
            name = result.stdout.split(': ', 1)[0].split(':', 1)[0]
            if name in packages:
                return name
    raise ValueError('Cannot establish source package ownership: ' + str(path))

def copy(path, package=None):
    path = Path(path)
    if str(path) in copied:
        return
    copied.add(str(path))
    target = destination / str(path).lstrip('/')
    target.parent.mkdir(parents=True, exist_ok=True)
    if path.is_symlink():
        link = os.readlink(path)
        target.symlink_to(link)
        # Preserve the link's immediate path. Resolving all parent symlinks here
        # would lose /lib loader aliases when copying out of a merged-/usr image.
        linked_path = Path(link) if os.path.isabs(link) else path.parent / link
        copy(Path(os.path.normpath(linked_path)), package)
    elif path.is_dir():
        target.mkdir(exist_ok=True)
        for child in sorted(path.iterdir()):
            copy(child, package)
    elif path.is_file():
        if package is None:
            package = 'cpython' if str(path).startswith('/usr/local/') else owner(path)
        shutil.copy2(path, target)
        owned[str(path)] = package
        if package != 'cpython':
            needed[package] = packages[package]
    else:
        raise ValueError('Unexpected platform image special file: ' + str(path))

# Retain the interpreter, full installed standard library and shared interpreter
# library. The pinned parent already defines which optional stdlib modules exist.
for source in ['/usr/local/bin/python3', '/usr/local/bin/python3.13', '/usr/local/lib/libpython3.13.so.1.0', '/usr/local/lib/python3.13']:
    copy(source, 'cpython')

# Delete the unused installer and every bundled installer dependency together.
stdlib = destination / 'usr/local/lib/python3.13'
for path in [stdlib / 'site-packages/pip', *stdlib.glob('site-packages/pip-*.dist-info')]:
    if path.exists():
        shutil.rmtree(path)

elf_files = [Path('/usr/local/bin/python3.13'), Path('/usr/local/lib/libpython3.13.so.1.0')]
elf_files += sorted(Path('/usr/local/lib/python3.13/lib-dynload').glob('*.so'))
for source in elf_files:
    result = subprocess.run(['/usr/bin/ldd', str(source)], check=True, capture_output=True, text=True)
    missing = [line.strip() for line in result.stdout.splitlines() if 'not found' in line]
    if missing and source.parent.name == 'lib-dynload':
        # The slim parent may carry optional extensions whose libraries it does
        # not ship (notably Tk). Record and omit only those already unusable files.
        excluded_unavailable.append({'path':str(source), 'missing':missing})
        (destination / str(source).lstrip('/')).unlink()
        continue
    for line in result.stdout.splitlines():
        if 'not found' in line:
            raise ValueError('Unresolved runtime library: ' + line)
        match = re.search(r'(?:=>\s*)?(/[^\s]+)\s+\(', line)
        if match:
            copy(match[1])

for source in ['/etc/ssl/certs/ca-certificates.crt', '/usr/share/zoneinfo', '/etc/passwd', '/etc/group', '/etc/nsswitch.conf', '/usr/lib/locale/C.utf8']:
    if Path(source).exists():
        copy(source)

# Include copyright/license material for every retained Debian component.
for package in list(needed):
    copyright_file = Path('/usr/share/doc') / package / 'copyright'
    if not copyright_file.exists():
        raise ValueError('Missing upstream license for ' + package)
    copy(copyright_file, package)

for directory in ['sbin', 'proc', 'sys', 'dev', 'tmp', 'sl/challenge', 'sl/validator', 'sl/suite', 'sl/submission', 'sl/config', 'sl/work', 'sl/output']:
    (destination / directory).mkdir(parents=True, exist_ok=True)

files = []
for path in sorted(destination.rglob('*')):
    if path.is_symlink():
        files.append({'path': '/' + str(path.relative_to(destination)), 'symlink': os.readlink(path)})
    elif path.is_file():
        original = '/' + str(path.relative_to(destination))
        files.append({'path': original, 'digest': 'sha256:' + hashlib.sha256(path.read_bytes()).hexdigest(), 'package': owned[original]})
interpreter = {'ecosystem': 'CPython', 'name': 'cpython', 'version': '.'.join(map(str, sys.version_info[:3])), 'digest': 'sha256:' + hashlib.sha256(Path(sys.executable).read_bytes()).hexdigest()}
document = {'kind': 'PlatformRuntimeComponents', 'version': 1, 'packages': sorted([interpreter, *needed.values()], key=lambda p: (p['ecosystem'], p['name'])), 'files': files, 'excludedAlreadyUnavailableExtensions':excluded_unavailable}
output = destination / 'usr/share/science-ladder/runtime-components.json'
output.parent.mkdir(parents=True, exist_ok=True)
output.write_text(json.dumps(document, sort_keys=True, separators=(',', ':')))
