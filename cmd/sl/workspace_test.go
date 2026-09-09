package main

import (
	"encoding/json"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestWorkspaceCheckerHelper(t *testing.T) {
	if os.Getenv("SL_WORKSPACE_HELPER") != "1" {
		return
	}
	if os.Getenv("SL_WORKSPACE_FAIL") == "1" {
		os.Exit(7)
	}
	report := os.Args[len(os.Args)-1]
	if err := os.WriteFile(report, []byte(os.Getenv("SL_WORKSPACE_RESULT")), 0600); err != nil {
		os.Exit(8)
	}
	os.Exit(0)
}
func TestWorkspaceSubmissionRechecksAndRejectsNonImprovementBeforeAdmission(t *testing.T) {
	sourceManifest, err := os.ReadFile("../../web/public/examples/quiet-echoes-manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := protocol.ParseManifest(sourceManifest)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	source := filepath.Join(root, "challenge")
	artifact := filepath.Join(root, "artifact")
	for _, d := range []string{source, artifact, filepath.Join(root, ".sl")} {
		if err = os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(source, "science-ladder.yaml"), sourceManifest, 0644))
	must(os.WriteFile(filepath.Join(artifact, "sequence.txt"), []byte(strings.Repeat("+", 512)+"\n"), 0644))
	executable, err := os.Executable()
	must(err)
	recipe := LocalRecipe{Version: 1, Baseline: []string{executable, "-test.run=^TestWorkspaceCheckerHelper$", "--", "{report}"}, Check: []string{executable, "-test.run=^TestWorkspaceCheckerHelper$", "--", "{artifact}", "{report}"}}
	data, _ := json.Marshal(recipe)
	must(os.WriteFile(filepath.Join(source, "science-ladder-local.json"), data, 0644))
	for _, d := range []string{source, artifact} {
		must(commandAt(d, "git", "init", "-q"))
		must(commandAt(d, "git", "add", "."))
		must(commandAt(d, "git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fixture"))
	}
	must(commandAt(artifact, "git", "remote", "add", "origin", "https://github.com/test/fixture.git"))
	head, err := gitOutput(source, "rev-parse", "HEAD")
	must(err)
	digest, _ := protocol.Digest(m)
	var posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			posts.Add(1)
			http.Error(w, "test admission denied", 403)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"manifest": m, "lockDigest": protocol.DigestBytes([]byte("test-lock")), "frontierTicks": m.Metric.BaselineTicks, "mode": "frontier"})
	}))
	defer server.Close()
	workspace := Workspace{API: server.URL, VersionID: "test-version", Repository: "test/fixture", SourceCommit: head, ManifestDigest: digest}
	data, _ = json.Marshal(workspace)
	must(os.WriteFile(filepath.Join(root, workspaceFile), data, 0600))
	t.Setenv("SL_API_TOKEN", "")
	t.Setenv("SL_WORKSPACE_HELPER", "1")
	result := protocol.ValidatorResult{APIVersion: m.APIVersion, Kind: "ValidatorResult", Score: "17996", Gates: map[string]bool{}}
	for _, g := range m.HardGates {
		result.Gates[g] = true
	}
	data, _ = json.Marshal(result)
	t.Setenv("SL_WORKSPACE_RESULT", string(data))
	t.Chdir(artifact)
	found, w, err := findWorkspace()
	must(err)
	if found != root || w.VersionID != "test-version" {
		t.Fatal("workspace discovery lost pinned identity")
	}
	must(workspaceCommand("run", []string{"--baseline"}))
	must(workspaceCommand("run", nil))
	err = workspaceCommand("submit", nil)
	if err == nil || !strings.Contains(err.Error(), "does not improve") || posts.Load() != 0 {
		t.Fatalf("baseline reached admission: %v, posts=%d", err, posts.Load())
	}
	t.Setenv("SL_WORKSPACE_FAIL", "1")
	err = workspaceCommand("submit", nil)
	if err == nil || !strings.Contains(err.Error(), "checker failed") || posts.Load() != 0 {
		t.Fatalf("failed checker reached admission: %v", err)
	}
	t.Setenv("SL_WORKSPACE_FAIL", "")
	result.Score = "17992"
	data, _ = json.Marshal(result)
	t.Setenv("SL_WORKSPACE_RESULT", string(data))
	err = workspaceCommand("submit", nil)
	if err == nil || !strings.Contains(err.Error(), "403") || posts.Load() != 1 {
		t.Fatalf("qualifying candidate did not request exactly one admission: %v, posts=%d", err, posts.Load())
	}
	// Tracked changes cannot silently change the local check contract.
	must(os.WriteFile(filepath.Join(source, "science-ladder-local.json"), []byte("{}"), 0644))
	if err = workspaceCommand("run", nil); err == nil || !strings.Contains(err.Error(), "tracked edits") {
		t.Fatalf("source edit accepted: %v", err)
	}
}
func TestLocalRecipeAndRepositoryContracts(t *testing.T) {
	r := programRecipe(nil)
	if err := validateRecipe(r); err != nil {
		t.Fatal(err)
	}
	r.Check = []string{"python3", "local.py", "--output", "{report}"}
	if validateRecipe(r) == nil {
		t.Fatal("artifact-free check accepted")
	}
	r = programRecipe(nil)
	r.Baseline = []string{"python3", "local.py"}
	if validateRecipe(r) == nil {
		t.Fatal("report-free baseline accepted")
	}
	for _, remote := range []string{"git@github.com:owner/repo.git", "https://github.com/owner/repo.git", "ssh://git@github.com/owner/repo"} {
		if repo, e := githubRepository(remote); e != nil || repo != "owner/repo" {
			t.Fatal(remote, e)
		}
	}
	for _, remote := range []string{"https://token@github.com/owner/repo", "https://evil.test/owner/repo", "https://github.com/owner/repo?x=1"} {
		if _, e := githubRepository(remote); e == nil {
			t.Fatal("unsafe origin accepted", remote)
		}
	}
	root := t.TempDir()
	os.Symlink("/etc/passwd", filepath.Join(root, "link"))
	if copyBaseline(root, t.TempDir()) == nil {
		t.Fatal("baseline symlink accepted")
	}
}

func TestWorkingArtifactExcludesOnlyRootGitMetadata(t *testing.T) {
	m, err := readManifest("../../web/public/examples/quiet-echoes-manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "sequence.txt")
	os.WriteFile(file, []byte(strings.Repeat("+", 512)+"\n"), 0644)
	_, expected, err := protocol.CanonicalArtifact(dir, m.Submission)
	if err != nil {
		t.Fatal(err)
	}
	os.Mkdir(filepath.Join(dir, ".git"), 0700)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("private metadata"), 0600)
	_, got, err := localArtifact(dir, m.Submission)
	if err != nil || got != expected {
		t.Fatal("Git metadata changed scored bytes", err)
	}
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not permitted"), 0644)
	if _, _, err = localArtifact(dir, m.Submission); err == nil {
		t.Fatal("untracked forbidden file ignored")
	}
	os.Remove(filepath.Join(dir, "notes.txt"))
	os.MkdirAll(filepath.Join(dir, "nested", ".git"), 0700)
	if _, _, err = localArtifact(dir, m.Submission); err == nil {
		t.Fatal("nested .git was ignored")
	}
}
