package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func TestResultSpoolIsExactAndReplaysOnlyOnAcknowledgment(t *testing.T) {
	dir := t.TempDir()
	value := delivery{JobID: "test-job", ResultToken: "unit-test-token", Envelope: protocol.Envelope{PayloadType: protocol.PayloadType, Payload: "test-only", Signatures: []protocol.Signature{{KeyID: "test", Sig: "test"}}}}
	filename, err := persistResult(dir, value)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filename)
	if info.Mode().Perm() != 0600 {
		t.Fatal("spool permissions expose receipt token")
	}
	if _, err := persistResult(dir, value); err != nil {
		t.Fatal("exact persisted result not idempotent", err)
	}
	conflict := value
	conflict.ResultToken = "different"
	if _, err := persistResult(dir, conflict); err == nil {
		t.Fatal("conflicting retry overwrote immutable result")
	}
	accepted := false
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/internal/v1/runner/jobs/test-job/result" {
			t.Error(r.URL.Path)
		}
		if !accepted {
			http.Error(w, "retry", 503)
			return
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	if err := replayResults(context.Background(), server.Client(), server.URL, dir); err == nil {
		t.Fatal("unacknowledged result deleted")
	}
	if _, err := os.Stat(filename); err != nil {
		t.Fatal(err)
	}
	accepted = true
	if err := replayResults(context.Background(), server.Client(), server.URL, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Fatal("acknowledged result not removed")
	}
	if requests != 2 {
		t.Fatal(requests)
	}
}

func TestResultSpoolRejectsPathAndSymlink(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"../escape", "a/b", "a.b", "a b", ""} {
		if _, err := persistResult(dir, delivery{JobID: id}); err == nil {
			t.Fatal(id)
		}
	}
	target := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(target, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "test.json")); err != nil {
		t.Fatal(err)
	}
	if err := replayResults(context.Background(), http.DefaultClient, "http://localhost", dir); err == nil {
		t.Fatal("spool symlink accepted")
	}
}
