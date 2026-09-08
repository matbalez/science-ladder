package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func TestClaimCheckerHelper(t *testing.T) {
	if os.Getenv("SL_CLAIM_TEST_HELPER") != "1" {
		return
	}
	mode := os.Getenv("SL_CLAIM_TEST_MODE")
	if mode == "fail" {
		os.Exit(3)
	}
	if mode == "mutate" {
		_ = os.WriteFile(os.Getenv("SL_CLAIM_TEST_ARTIFACT"), []byte(strings.Repeat("-", 512)+"\n"), 0644)
	}
	fmt.Print(os.Getenv("SL_CLAIM_TEST_RESULT"))
	os.Exit(0)
}
func TestClaimFinalCheckAndSubmissionBinding(t *testing.T) {
	path, err := filepath.Abs("../../web/public/examples/quiet-echoes-manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := readManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	var posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			posts.Add(1)
			http.Error(w, "unexpected mutation", 500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"manifest": m, "lockDigest": protocol.DigestBytes([]byte("lock")), "frontierTicks": m.Metric.BaselineTicks, "mode": "frontier"})
	}))
	defer server.Close()
	dir := t.TempDir()
	artifact := filepath.Join(dir, "artifact")
	os.Mkdir(artifact, 0755)
	file := filepath.Join(artifact, "sequence.txt")
	os.WriteFile(file, []byte(strings.Repeat("+", 512)+"\n"), 0644)
	result := protocol.ValidatorResult{APIVersion: m.APIVersion, Kind: "ValidatorResult", Score: "17992", Gates: map[string]bool{}}
	for _, g := range m.HardGates {
		result.Gates[g] = true
	}
	data, _ := json.Marshal(result)
	t.Setenv("SL_CLAIM_TEST_HELPER", "1")
	t.Setenv("SL_CLAIM_TEST_RESULT", string(data))
	t.Setenv("SL_CLAIM_TEST_ARTIFACT", file)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args := func(out string) []string {
		return []string{"--api", server.URL, "--version", "version-test", "--manifest", path, "--artifact", artifact, "--out", out, "--", executable, "-test.run=^TestClaimCheckerHelper$"}
	}
	claimPath := filepath.Join(dir, "claim.json")
	if err = claimCommand(args(claimPath)); err != nil {
		t.Fatal(err)
	}
	// A changed candidate must be rejected before the API receives any admission request.
	os.WriteFile(file, []byte(strings.Repeat("-", 512)+"\n"), 0644)
	err = remoteCommand([]string{"submit", "--api", server.URL, "--version", "version-test", "--manifest", path, "--artifact", artifact, "--claim", claimPath, "--repository", "test/repo", "--commit", strings.Repeat("a", 40)})
	if err == nil || !strings.Contains(err.Error(), "artifact changed") {
		t.Fatal(err)
	}
	if posts.Load() != 0 {
		t.Fatal("changed candidate reached admission")
	}
	for _, mode := range []string{"fail", "mutate", "loser"} {
		os.WriteFile(file, []byte(strings.Repeat("+", 512)+"\n"), 0644)
		t.Setenv("SL_CLAIM_TEST_MODE", mode)
		if mode == "loser" {
			result.Score = "17996"
			data, _ = json.Marshal(result)
			t.Setenv("SL_CLAIM_TEST_RESULT", string(data))
		}
		out := filepath.Join(dir, mode+".json")
		if err = claimCommand(args(out)); err == nil {
			t.Fatal("claim created for", mode)
		}
		if _, e := os.Stat(out); !os.IsNotExist(e) {
			t.Fatal("failed check left claim file", mode)
		}
	}
}
