# SPDX-License-Identifier: MIT
"""No network: mocked gh responses, real disposable git repositories."""
import contextlib
import importlib.util
import io
import http.server
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch

PATH = Path(__file__).resolve().parents[1] / "github-repository.py"
SPEC = importlib.util.spec_from_file_location("github_repository", PATH)
m = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = m
SPEC.loader.exec_module(m)


class FakeGitHub(m.Commands):
    def __init__(self, bare):
        super().__init__()
        self.bare = str(bare)
        self.calls = []
        self.network = []
        self.repo = None
        self.enrollment_status = 204
        self.account_id = 42
        self.user_id = 42

    def api(self, method, endpoint, payload=None):
        self.calls.append((method, endpoint, payload))
        if (method, endpoint) == ("GET", "user"):
            return m.Response(200, {"id": self.user_id, "login": "operator"})
        if (method, endpoint) == ("GET", "users/operator"):
            return m.Response(200, {"id": self.account_id, "login": "operator", "type": "User"})
        if (method, endpoint) == ("GET", "repos/operator/dedicated"):
            return m.Response(200, self.repo) if self.repo else m.Response(404)
        if (method, endpoint) == ("POST", "user/repos"):
            assert self.repo is None
            self.repo = {"id": 71, "full_name": "operator/dedicated", "owner": {"id": self.account_id},
                         "private": payload["private"], "permissions": {"admin": True}, "fork": False}
            return m.Response(201, self.repo)
        if (method, endpoint) == ("PUT", "user/installations/99/repositories/71"):
            return m.Response(self.enrollment_status)
        raise AssertionError((method, endpoint, payload))

    def network_git(self, source, *args):
        self.network.append(args)
        # Only GitHub transport is mocked; ancestry, normal pushes and refs are real.
        return self.git(source, *[self.bare if a == "https://github.com/operator/dedicated.git" else a for a in args])


class ProvisionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.source = self.root / "source"
        self.source.mkdir()
        (self.source / "README.md").write_text("Reviewed standalone source\n")
        self.bare = self.root / "remote.git"
        subprocess.run(["git", "init", "--bare", str(self.bare)], capture_output=True, check=True)
        self.gh = FakeGitHub(self.bare)
        self.args = ["--source", str(self.source), "--repository", "operator/dedicated", "--installation-id", "99"]

    def tearDown(self):
        self.temp.cleanup()

    def invoke(self, extra=()):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            code = m.main(self.args + list(extra), self.gh)
        return code, json.loads(out.getvalue())

    def commit(self, directory=None, text="Changed source\n"):
        directory = directory or self.source
        (directory / "README.md").write_text(text)
        self.gh.git(directory, "add", "README.md")
        self.gh.git(directory, "-c", "user.name=Test", "-c", "user.email=test@example.invalid",
                    "-c", "commit.gpgSign=false", "-c", "core.hooksPath=/dev/null", "commit", "-m", "Review change")

    def remote_head(self):
        return self.gh.git(self.bare, "rev-parse", "refs/heads/main")

    def test_create_defaults_private_and_enrolls_exact_id(self):
        code, result = self.invoke()
        self.assertEqual(code, 0)
        self.assertEqual(result["status"], "complete")
        self.assertTrue(result["created"] and result["pushed"] and result["enrolled"])
        self.assertEqual(result["commit"], self.remote_head())
        self.assertIn(("POST", "user/repos", {"name": "dedicated", "private": True, "auto_init": False}), self.gh.calls)
        self.assertEqual(self.gh.calls[-1], ("PUT", "user/installations/99/repositories/71", None))
        self.assertFalse(any("installations" in call[1] and call[0] == "GET" for call in self.gh.calls))
        self.assertFalse(any("--force" in arg for call in self.gh.network for arg in call))

    def test_idempotent_resume_accepts_304_without_second_push_or_creation(self):
        _, first = self.invoke()
        self.gh.enrollment_status = 304
        code, second = self.invoke()
        self.assertEqual(code, 0)
        self.assertFalse(second["created"] or second["pushed"])
        self.assertEqual(first["commit"], second["commit"])
        self.assertEqual(second["enrollmentHttpStatus"], 304)
        self.assertEqual(sum(c[0] == "POST" for c in self.gh.calls), 1)
        self.assertEqual(sum("push" in c for c in self.gh.network), 1)

    def test_403_preserves_pushed_repository_then_resumes(self):
        self.gh.enrollment_status = 403
        code, blocked = self.invoke()
        self.assertEqual(code, 3)
        self.assertEqual(blocked["status"], "enrollment_blocked")
        self.assertTrue(blocked["remoteCommitVerified"])
        self.assertFalse(blocked["enrolled"])
        self.assertIn("classic PAT", blocked["error"])
        self.assertEqual(blocked["commit"], self.remote_head())
        self.gh.enrollment_status = 204
        code, resumed = self.invoke()
        self.assertEqual(code, 0)
        self.assertFalse(resumed["created"] or resumed["pushed"])
        self.assertEqual(blocked["commit"], resumed["commit"])

    def test_conflicting_origin_stops_before_api_and_never_prints_url_token(self):
        self.gh.git(self.source, "init", "--initial-branch=main")
        self.commit()
        self.gh.git(self.source, "remote", "add", "origin", "https://ghp_NEVER_PRINT@github.com/operator/dedicated.git")
        code, result = self.invoke()
        self.assertEqual(code, 1)
        self.assertIn("conflicts", result["error"])
        self.assertNotIn("ghp_NEVER_PRINT", json.dumps(result))
        self.assertEqual(self.gh.calls, [])

    def test_visibility_mismatch_is_not_changed(self):
        self.invoke()
        code, result = self.invoke(("--visibility", "public"))
        self.assertEqual(code, 1)
        self.assertIn("visibility", result["error"])
        self.assertTrue(self.gh.repo["private"])

    def test_fast_forward_updates_only_requested_branch(self):
        self.invoke()
        first = self.remote_head()
        self.commit()
        self.gh.git(self.source, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "tag", "-a", "unrequested", "-m", "Do not push")
        self.gh.git(self.source, "config", "push.followTags", "true")
        code, result = self.invoke()
        self.assertEqual(code, 0)
        self.assertNotEqual(first, self.remote_head())
        self.assertEqual(self.gh.git(self.bare, "tag", "--list"), "")
        self.assertTrue(result["pushed"])

    def test_divergent_remote_history_is_never_overwritten(self):
        self.invoke()
        self.commit(text="Local continuation\n")
        other = self.root / "other"
        subprocess.run(["git", "clone", "--branch", "main", str(self.bare), str(other)], capture_output=True, check=True)
        self.commit(other, "Remote continuation\n")
        self.gh.git(other, "push", "origin", "main")
        before = self.remote_head()
        code, result = self.invoke()
        self.assertEqual(code, 1)
        self.assertIn("not an ancestor", result["error"])
        self.assertEqual(before, self.remote_head())

    def test_existing_repo_without_matching_local_origin_is_not_adopted(self):
        self.gh.repo = {"id": 71, "full_name": "operator/dedicated", "owner": {"id": 42}, "private": True, "permissions": {"admin": True}}
        code, result = self.invoke()
        self.assertEqual(code, 1)
        self.assertIn("no matching origin", result["error"])
        self.assertFalse((self.source / ".git").exists())

    def test_other_personal_owner_is_denied_even_with_collaborator_access(self):
        self.gh.account_id = 999
        code, result = self.invoke()
        self.assertEqual(code, 1)
        self.assertIn("authenticated user", result["error"])
        self.assertFalse(any(c[0] != "GET" for c in self.gh.calls))

    def test_wrong_repository_identity_is_rejected(self):
        self.invoke()
        self.gh.repo["owner"]["id"] = 123
        code, result = self.invoke()
        self.assertEqual(code, 1)
        self.assertIn("identity differs", result["error"])

    def test_suspended_style_404_enrollment_is_not_claimed_successful(self):
        self.gh.enrollment_status = 404
        code, result = self.invoke()
        self.assertEqual(code, 1)
        self.assertTrue(result["remoteCommitVerified"])
        self.assertFalse(result["enrolled"])
        self.assertEqual(result["httpStatus"], 404)

    def test_nested_source_does_not_publish_parent_repository(self):
        self.gh.git(self.root, "init")
        code, result = self.invoke()
        self.assertEqual(code, 1)
        self.assertIn("nested", result["error"])
        self.assertEqual(self.gh.calls, [])

    def test_dirty_existing_source_requires_reviewed_commit(self):
        self.invoke()
        (self.source / "README.md").write_text("Unreviewed changes\n")
        code, result = self.invoke()
        self.assertEqual(code, 1)
        self.assertIn("not clean", result["error"])

    def test_credential_filename_stops_before_creation(self):
        (self.source / ".env.local").write_text("TOKEN=do-not-publish\n")
        code, result = self.invoke()
        self.assertEqual(code, 1)
        self.assertIn("credential-like", result["error"])
        self.assertNotIn("do-not-publish", json.dumps(result))
        self.assertFalse(any(c[0] != "GET" for c in self.gh.calls))

    def test_push_url_rewrite_is_rejected(self):
        self.invoke()
        self.gh.git(self.source, "config", "url.https://example.invalid/.pushInsteadOf", "https://github.com/")
        code, result = self.invoke()
        self.assertEqual(code, 1)
        self.assertTrue("conflicts" in result["error"] or "rewrites" in result["error"])


class TransportTests(unittest.TestCase):
    def test_inherited_git_repository_overrides_cannot_change_source(self):
        overrides = {"GIT_DIR": "/unrelated/.git", "GIT_WORK_TREE": "/unrelated",
                     "GIT_INDEX_FILE": "/unrelated/index", "GIT_COMMON_DIR": "/unrelated/common",
                     "GIT_OBJECT_DIRECTORY": "/unrelated/objects", "GIT_NAMESPACE": "other",
                     "GH_DEBUG": "api", "GIT_TRACE_CURL": "1", "GH_TOKEN": "test-fixture-only"}
        with patch.dict(os.environ, overrides):
            runner = m.Commands()
        for key in overrides:
            if key != "GH_TOKEN":
                self.assertNotIn(key, runner.env)
        self.assertEqual(runner.env["GH_TOKEN"], "test-fixture-only")
        self.assertEqual(runner.env["GIT_ASKPASS"], "")
        self.assertEqual(runner.env["SSH_ASKPASS"], "")

    def test_git_transport_does_not_follow_a_repository_redirect(self):
        requests = []

        class Redirect(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                requests.append(self.path)
                self.send_response(301)
                self.send_header("Location", "/transferred/repo.git/info/refs?service=git-upload-pack")
                self.end_headers()

            def log_message(self, *_):
                pass

        server = http.server.HTTPServer(("127.0.0.1", 0), Redirect)
        worker = threading.Thread(target=server.serve_forever, daemon=True)
        worker.start()
        try:
            with tempfile.TemporaryDirectory() as source:
                runner = m.Commands()
                with self.assertRaises(m.ProvisionError):
                    runner.network_git(Path(source), "ls-remote", f"http://127.0.0.1:{server.server_port}/source.git")
            self.assertEqual(len(requests), 1)
            self.assertTrue(requests[0].startswith("/source.git/"))
        finally:
            server.shutdown()
            server.server_close()
            worker.join()

    def test_network_git_disables_redirects_and_uses_only_local_gh_helper(self):
        runner = m.Commands()
        with patch.object(runner, "git", return_value="") as git:
            runner.network_git(Path("."), "ls-remote", "https://github.com/operator/dedicated.git")
        call = git.call_args.args
        self.assertIn("http.followRedirects=false", call)
        self.assertIn("credential.helper=", call)
        self.assertIn("credential.https://github.com.helper=!gh auth git-credential", call)

    def test_api_parses_403_without_printing_body_or_stderr(self):
        runner = m.Commands()
        response = subprocess.CompletedProcess([], 1, 'HTTP/2.0 403 Forbidden\nContent-Type: application/json\n\n{"secret":"ghp_NEVER_PRINT"}', 'secret ghp_NEVER_PRINT')
        with patch.object(runner, "run", return_value=response) as run:
            received = runner.api("PUT", "user/installations/1/repositories/2")
        self.assertEqual(received.status, 403)
        self.assertNotIn("ghp_NEVER_PRINT", repr(run.call_args))
        self.assertNotIn("auth token", repr(run.call_args))

    def test_accepted_remote_forms_and_foreign_paths(self):
        for url in ["https://github.com/operator/dedicated.git", "git@github.com:operator/dedicated.git", "ssh://git@github.com/operator/dedicated.git"]:
            self.assertTrue(m.same_remote(url, "operator/dedicated"), url)
        for url in ["https://github.com.evil/operator/dedicated", "https://token@github.com/operator/dedicated", "https://github.com/operator/dedicated/extra", "https://github.com/other/dedicated"]:
            self.assertFalse(m.same_remote(url, "operator/dedicated"), url)


if __name__ == "__main__":
    unittest.main()
