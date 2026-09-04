#!/usr/bin/env python3
"""Start isolated local services; store generated credentials in ignored work/."""
import argparse
import os
from pathlib import Path
import secrets
import subprocess

ROOT = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("action", choices=["up", "stop", "exec"], nargs="?", default="up")
parser.add_argument("command", nargs=argparse.REMAINDER)
args = parser.parse_args()
directory = ROOT / "work"
directory.mkdir(mode=0o700, exist_ok=True)
configuration = directory / "local-services.env"
if not configuration.exists():
    db_password, storage_password = secrets.token_hex(24), secrets.token_hex(24)
    values = {
        "DEV_DATABASE_PASSWORD": db_password,
        "DEV_STORAGE_PASSWORD": storage_password,
        "DATABASE_URL": f"postgres://science_ladder:{db_password}@127.0.0.1:55432/science_ladder?sslmode=disable",
        "TEST_DATABASE_URL": f"postgres://science_ladder:{db_password}@127.0.0.1:55432/science_ladder?sslmode=disable",
        "S3_BUCKET": "science-ladder-local",
        "S3_REGION": "us-east-1",
        "S3_ENDPOINT": "http://127.0.0.1:59000",
        "AWS_ACCESS_KEY_ID": "science-ladder-local",
        "AWS_SECRET_ACCESS_KEY": storage_password,
        "DEPLOYMENT_MODE": "local",
        "PUBLIC_ORIGIN": "http://localhost:3000",
        "LISTEN_ADDR": "127.0.0.1:8080",
    }
    fd = os.open(configuration, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as output:
        output.write("".join(f"{key}={value}\n" for key, value in values.items()))
if configuration.is_symlink() or configuration.stat().st_mode & 0o077:
    raise SystemExit("Local configuration must be a private regular file (mode 600).")
environment = os.environ.copy()
for line in configuration.read_text().splitlines():
    key, value = line.split("=", 1)
    environment[key] = value
if args.action == "exec":
    if not args.command:
        parser.error("exec requires a command")
    raise SystemExit(subprocess.call(args.command, cwd=ROOT, env=environment))
command = ["docker", "compose", "--env-file", str(configuration)]
command += ["up", "--detach", "--wait"] if args.action == "up" else ["stop"]
subprocess.run(command, cwd=ROOT, env=environment, check=True)
if args.action == "up":
    subprocess.run(["go", "run", "scripts/local-bucket.go"], cwd=ROOT, env=environment, check=True)
    print("Local PostgreSQL and private object storage are ready. Credentials remain in work/local-services.env.")
    print("Run application commands with: python3 scripts/dev-services.py exec <command>")
