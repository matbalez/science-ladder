#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Create/resume one reviewed repository, push normally, then enroll its exact ID.

Uses locally configured GitHub CLI authentication. Never reads or prints tokens.
No browser fallback, destructive recovery, installation-wide grants, or force push.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
from dataclasses import dataclass

API_VERSION = "2022-11-28"
ENROLLMENT_DOC = "https://docs.github.com/en/rest/apps/installations#add-a-repository-to-an-app-installation"


class ProvisionError(Exception):
    def __init__(self, message: str, *, status: int | None = None, code: int = 1):
        super().__init__(message)
        self.status = status
        self.code = code


@dataclass
class Response:
    status: int
    data: object = None


class Commands:
    """Capture command output without exposing remote error text or credentials."""
    def __init__(self):
        self.env = os.environ.copy()
        for key in list(self.env):
            if key == "GH_DEBUG" or key.startswith("GIT_TRACE") or key == "GIT_CURL_VERBOSE":
                self.env.pop(key)
        # Hooks and parent Git invocations can export these. -C alone does not
        # override them, so never let them select a different repository/index.
        for key in ("GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
                    "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
                    "GIT_NAMESPACE", "GIT_PREFIX", "GIT_SUPER_PREFIX"):
            self.env.pop(key, None)
        self.env.update(GH_PROMPT_DISABLED="1", GIT_TERMINAL_PROMPT="0",
                        GIT_ASKPASS="", SSH_ASKPASS="")

    def run(self, arguments, *, cwd=None, payload=None, allowed=(0,)):
        try:
            result = subprocess.run(arguments, cwd=cwd, input=payload, text=True,
                                    capture_output=True, timeout=180, env=self.env)
        except FileNotFoundError:
            raise ProvisionError(f"Required executable is unavailable: {arguments[0]}") from None
        except subprocess.TimeoutExpired:
            raise ProvisionError(f"{arguments[0]} timed out. Remote state may have changed; rerun the same inputs to inspect and resume.") from None
        if result.returncode not in allowed:
            raise ProvisionError(f"{arguments[0]} operation failed (exit {result.returncode}); no captured command output was printed. Check local authentication and repository state, then resume.")
        return result

    def api(self, method: str, endpoint: str, payload=None) -> Response:
        args = ["gh", "api", "--hostname", "github.com", "--method", method,
                "--include", "-H", "Accept: application/vnd.github+json",
                "-H", "X-GitHub-Api-Version: " + API_VERSION, endpoint]
        if payload is not None:
            args += ["--input", "-"]
        result = self.run(args, payload=json.dumps(payload) if payload is not None else None,
                          allowed=(0, 1))
        output = result.stdout.replace("\r\n", "\n")
        match = re.match(r"HTTP/\S+ (\d{3})[^\n]*\n", output)
        if not match:
            raise ProvisionError("GitHub API returned no readable HTTP status. Check authenticated gh access locally; no credential or raw response was printed.")
        status = int(match.group(1))
        body = output.partition("\n\n")[2]
        try:
            data = json.loads(body) if body.strip() else None
        except json.JSONDecodeError:
            raise ProvisionError("GitHub API returned an unexpected response format.", status=status) from None
        if result.returncode and status < 400 and status != 304:
            raise ProvisionError("GitHub CLI failed despite an HTTP success status; inspect state before resuming.", status=status)
        return Response(status, data)

    def git(self, source: Path, *args, allowed=(0,)):
        return self.run(["git", "-C", str(source), *args], allowed=allowed).stdout.strip()

    def network_git(self, source: Path, *args):
        # Command-local helper only; does not change global git configuration.
        return self.git(source, "-c", "http.followRedirects=false", "-c", "credential.helper=", "-c",
                        "credential.https://github.com.helper=!gh auth git-credential", *args)


def require_response(response: Response, expected: tuple[int, ...], purpose: str):
    if response.status not in expected:
        raise ProvisionError(f"{purpose} failed (HTTP {response.status}). Check gh authentication, ownership and permissions; no browser fallback was opened.", status=response.status)
    return response.data


def repository_name(value: str):
    if not re.fullmatch(r"[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})/[A-Za-z0-9_.-]{1,100}", value):
        raise ProvisionError("Repository must be an explicit GitHub owner/name.")
    owner, name = value.split("/")
    if name in (".", "..") or name.endswith(".git"):
        raise ProvisionError("Use a repository name without .git, '.' or '..'.")
    return owner, name


def same_remote(value: str, full_name: str) -> bool:
    # Userinfo in HTTPS, query strings, foreign hosts and extra path parts fail.
    patterns = (r"https://github\.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+?)(?:\.git)?/?",
                r"git@github\.com:([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+?)(?:\.git)?",
                r"ssh://git@github\.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+?)(?:\.git)?/?")
    return any((m := re.fullmatch(pattern, value)) and m[1].casefold() == full_name.casefold()
               for pattern in patterns)


def inspect_source(commands: Commands, source: Path, full_name: str):
    if not source.is_dir() or source.is_symlink():
        raise ProvisionError("Source must be a real directory, not a symlink.")
    top = commands.git(source, "rev-parse", "--show-toplevel", allowed=(0, 128))
    if top and Path(top).resolve() != source.resolve():
        raise ProvisionError("Source is nested inside another git repository. Prepare a dedicated source directory outside that checkout.")
    if not top:
        if (source / ".git").exists():
            raise ProvisionError("Source has an unreadable or unsupported .git entry.")
        return {"initialized": False, "origin": False, "branch": None}
    urls = commands.git(source, "remote", "get-url", "--all", "origin", allowed=(0, 2, 128)).splitlines()
    push_urls = commands.git(source, "remote", "get-url", "--push", "--all", "origin", allowed=(0, 2, 128)).splitlines()
    if urls or push_urls:
        if len(urls) != 1 or len(push_urls) != 1 or not same_remote(urls[0], full_name) or not same_remote(push_urls[0], full_name):
            raise ProvisionError("Local origin fetch/push destination conflicts with the explicit owner/repository. No remote was changed.")
    branch = commands.git(source, "symbolic-ref", "--quiet", "--short", "HEAD", allowed=(0, 1)) or None
    head = commands.git(source, "rev-parse", "--verify", "HEAD", allowed=(0, 128))
    if head and commands.git(source, "status", "--porcelain", "--untracked-files=all"):
        raise ProvisionError("Existing source repository is not clean. Review and commit its changes before publishing.")
    return {"initialized": True, "origin": bool(urls), "branch": branch, "head": head}


def verify_owner(commands: Commands, owner: str):
    user = require_response(commands.api("GET", "user"), (200,), "Authenticated user lookup")
    account = require_response(commands.api("GET", "users/" + owner), (200,), "Destination account lookup")
    if not isinstance(user, dict) or not isinstance(account, dict) or not isinstance(user.get("id"), int) or not isinstance(account.get("id"), int):
        raise ProvisionError("GitHub returned incomplete account identity data.")
    if account.get("login", "").casefold() != owner.casefold():
        raise ProvisionError("Destination account resolved to a different login; use its exact current name.")
    if account.get("type") == "User":
        if account["id"] != user["id"]:
            raise ProvisionError("Personal repositories must belong to the authenticated user; collaborator access is insufficient.")
    elif account.get("type") == "Organization":
        membership = require_response(commands.api("GET", "user/memberships/orgs/" + owner), (200,), "Organization ownership lookup")
        if membership.get("state") != "active" or membership.get("role") != "admin":
            raise ProvisionError("Organization destination requires an active organization owner. This helper does not create organizations or broaden installation grants.")
    else:
        raise ProvisionError("Unsupported destination account type.")
    return user, account


def verify_repository(repo, account, full_name: str, visibility: str):
    if (not isinstance(repo, dict) or repo.get("full_name", "").casefold() != full_name.casefold()
            or repo.get("owner", {}).get("id") != account["id"] or not isinstance(repo.get("id"), int)):
        raise ProvisionError("GitHub repository identity differs from the explicitly owned destination.")
    if repo.get("private") is not (visibility == "private"):
        raise ProvisionError("Existing repository visibility differs from the requested visibility; the helper will not change it.")
    if not repo.get("permissions", {}).get("admin"):
        raise ProvisionError("Repository admin permission is required for this dedicated repository and installation enrollment.")
    if repo.get("fork") or repo.get("archived") or repo.get("disabled"):
        raise ProvisionError("Refusing a fork, archived repository or disabled destination.")


def reject_credential_paths(paths: list[str]):
    # A narrow tripwire, not a claim that committed content or history is secret-free.
    for path in paths:
        name = Path(path).name.lower()
        if ((name == ".env" or name.startswith(".env.")) and name not in (".env.example", ".env.sample", ".env.template")) or name in ("id_rsa", "id_ed25519", "credentials.json", ".netrc") or name.endswith((".p12", ".pfx", ".key")):
            raise ProvisionError("Source contains a credential-like filename. Remove secrets from the publishable source/history or exclude untracked private files before continuing; no filename or contents were printed.")


def prepare_commit(commands: Commands, source: Path, state: dict, user: dict, branch: str):
    if not state["initialized"]:
        commands.git(source, "init", "--initial-branch=" + branch)
    head = commands.git(source, "rev-parse", "--verify", "HEAD", allowed=(0, 128))
    if not head:
        paths = commands.git(source, "ls-files", "--cached", "--others", "--exclude-standard", "-z").split("\0")
        reject_credential_paths([p for p in paths if p])
        commands.git(source, "add", "--all", "--", ".")
        if not commands.git(source, "ls-files"):
            raise ProvisionError("Source has no publishable files after .gitignore exclusions.")
        commands.git(source, "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgSign=false",
                     "-c", "user.name=" + user["login"], "-c",
                     f"user.email={user['id']}+{user['login']}@users.noreply.github.com",
                     "commit", "-m", "Initial source import")
        head = commands.git(source, "rev-parse", "--verify", "HEAD")
    else:
        paths = commands.git(source, "ls-tree", "-r", "--name-only", "-z", "HEAD").split("\0")
        reject_credential_paths([p for p in paths if p])
    if not re.fullmatch(r"[a-f0-9]{40}", head):
        raise ProvisionError("Source needs a valid GitHub-compatible commit.")
    return head


def remote_refs(commands: Commands, source: Path, url: str):
    # Avoid silently honoring a configured URL rewrite to a different destination.
    resolved = commands.git(source, "ls-remote", "--get-url", url)
    if resolved != url:
        raise ProvisionError("Git configuration rewrites the explicit GitHub URL. Remove that rewrite before publishing.")
    rules = commands.git(source, "config", "--null", "--get-regexp", r"^url\..*\.pushinsteadof$", allowed=(0, 1))
    rewrites = []
    for rule in rules.split("\0"):
        key, sep, prefix = rule.partition("\n")
        if sep and url.startswith(prefix):
            base = key[len("url."):-len(".pushinsteadof")]
            rewrites.append((len(prefix), base + url[len(prefix):]))
    if any(rewritten != url for _, rewritten in rewrites):
        raise ProvisionError("Git configuration rewrites the GitHub push URL. Remove that rewrite before publishing.")
    refs = {}
    for line in commands.network_git(source, "ls-remote", "--refs", url).splitlines():
        sha, sep, ref = line.partition("\t")
        if not sep or not re.fullmatch(r"[a-f0-9]{40}", sha) or not ref.startswith("refs/"):
            raise ProvisionError("Remote returned malformed reference data.")
        refs[ref] = sha
    return refs


def push_commit(commands: Commands, source: Path, url: str, branch: str, head: str):
    target = "refs/heads/" + branch
    refs = remote_refs(commands, source, url)
    previous = refs.get(target)
    if refs and not previous:
        raise ProvisionError("Existing destination has history but lacks the selected branch. This helper will not add an unrelated branch to an existing repository.")
    if previous and previous != head:
        commands.network_git(source, "fetch", "--no-tags", url, target)
        result = commands.run(["git", "-C", str(source), "merge-base", "--is-ancestor", previous, head], allowed=(0, 1))
        if result.returncode:
            raise ProvisionError("Remote history is not an ancestor of the reviewed source. No force push was attempted; reconcile history explicitly.")
    if previous != head:
        commands.network_git(source, "-c", "push.followTags=false", "-c", "push.recurseSubmodules=no",
                             "push", "--no-verify", "--porcelain", url, head + ":" + target)
    if remote_refs(commands, source, url).get(target) != head:
        raise ProvisionError("Remote commit changed or could not be confirmed after push. Resume to inspect its state.")
    return previous != head


def provision(args, commands: Commands, result: dict):
    owner, name = repository_name(args.repository)
    source = args.source.absolute()
    state = inspect_source(commands, source, args.repository)
    branch = args.branch or state["branch"] or "main"
    if branch.startswith("-") or not branch:
        raise ProvisionError("Invalid destination branch.")
    commands.git(source, "check-ref-format", "--branch", branch)
    user, account = verify_owner(commands, owner)
    existing = commands.api("GET", "repos/" + args.repository)
    if existing.status == 200:
        verify_repository(existing.data, account, args.repository, args.visibility)
        if not state["origin"]:
            raise ProvisionError("Destination already exists but source has no matching origin. Inspect both repositories and explicitly configure the correct origin before resuming.")
        repo = existing.data
    elif existing.status == 404:
        repo = None
    else:
        require_response(existing, (200, 404), "Repository lookup")
    head = prepare_commit(commands, source, state, user, branch)
    url = "https://github.com/" + args.repository + ".git"
    result.update(repository="https://github.com/" + args.repository, branch=branch,
                  commit=head, visibility=args.visibility, installationId=args.installation_id,
                  created=False, pushed=False, enrolled=False)
    if repo is None:
        endpoint = "user/repos" if account["type"] == "User" else "orgs/" + owner + "/repos"
        response = commands.api("POST", endpoint, {"name": name, "private": args.visibility == "private", "auto_init": False})
        require_response(response, (201,), "Repository creation")
        result["created"] = True
        if not state["origin"]:
            commands.git(source, "remote", "add", "origin", url)
            state["origin"] = True
        # Fresh lookup verifies exact owner/admin/visibility before publishing bytes.
        repo = require_response(commands.api("GET", "repos/" + args.repository), (200,), "Created repository verification")
        verify_repository(repo, account, args.repository, args.visibility)
    result["repositoryId"] = repo["id"]
    if not state["origin"]:
        commands.git(source, "remote", "add", "origin", url)
    result["pushed"] = push_commit(commands, source, url, branch, head)
    result["remoteCommitVerified"] = True
    response = commands.api("PUT", f"user/installations/{args.installation_id}/repositories/{repo['id']}")
    if response.status == 403:
        raise ProvisionError("Repository and exact commit are preserved, but installation enrollment was denied (HTTP 403). GitHub documents this endpoint as requiring a classic PAT with repo scope and repository admin access; ordinary gh OAuth, fine-grained PATs and App tokens cannot perform it. Check local gh credential type, installation ID and applicable organization policy, then rerun the same inputs with authorized credentials. No token was printed, browser opened, or installation-wide grant changed. See " + ENROLLMENT_DOC, status=403, code=3)
    require_response(response, (204, 304), "Exact repository installation enrollment")
    result.update(status="complete", enrolled=True, enrollmentHttpStatus=response.status,
                  installationIdentity="operator-supplied; GitHub authorized the exact repository enrollment")
    return result


def parser():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--source", required=True, type=Path, help="Reviewed standalone directory, or clean dedicated git checkout")
    p.add_argument("--repository", required=True, metavar="OWNER/REPO")
    p.add_argument("--visibility", choices=("private", "public"), default="private")
    p.add_argument("--installation-id", required=True, type=int, help="Explicit existing GitHub App installation ID")
    p.add_argument("--branch", help="Target branch; defaults to source branch, or main for a new directory")
    return p


def main(argv=None, commands=None):
    args = parser().parse_args(argv)
    result = {"status": "incomplete"}
    try:
        if args.installation_id <= 0:
            raise ProvisionError("Installation ID must be a positive integer.")
        provision(args, commands or Commands(), result)
    except ProvisionError as error:
        result.update(status="enrollment_blocked" if error.code == 3 else "incomplete",
                      error=str(error), httpStatus=error.status)
        print(json.dumps(result, sort_keys=True))
        return error.code
    except (OSError, ValueError, KeyError, TypeError):
        # Do not leak credential-bearing environment, subprocess output or paths.
        result.update(error="Unexpected local or API data error; inspect source and authentication locally. No captured output was printed.")
        print(json.dumps(result, sort_keys=True))
        return 1
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
