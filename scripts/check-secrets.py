#!/usr/bin/env python3
"""Check publishable files without printing matched credentials."""
from pathlib import Path
import re
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
paths = subprocess.check_output(["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"], cwd=root).decode().split("\0")
patterns = [
    re.compile(rb"(?m)^-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----\r?$"),
    re.compile(rb"\bsk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{40,}"),
    re.compile(rb"\bgh[pousr]_[A-Za-z0-9]{32,}"),
    re.compile(rb"\bgithub_pat_[A-Za-z0-9_]{40,}"),
    re.compile(rb"\bAKIA[A-Z0-9]{16}\b"),
]
failed = False
for name in sorted(set(paths)):
    if not name:
        continue
    path = root / name
    if path.is_symlink():
        continue
    if path.name.startswith(".env") and path.name != ".env.example":
        print(f"Private environment file would be published: {name}")
        failed = True
    if path.is_file() and any(pattern.search(path.read_bytes()) for pattern in patterns):
        print(f"Credential-shaped material found in: {name} (value suppressed)")
        failed = True
if failed:
    sys.exit(1)
print("No credential-shaped material in publishable files.")
