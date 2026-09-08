package platform

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestPublicSourceNeedsNoInstallationAndBindsLargeV2File(t *testing.T) {
	data := bytes.Repeat([]byte("# scientific source data\n"), 15000)
	blobHash := func() string {
		h := sha1.New()
		fmt.Fprintf(h, "blob %d%c", len(data), 0)
		h.Write(data)
		return hex.EncodeToString(h.Sum(nil))
	}
	commit := strings.Repeat("a", 40)
	public := true
	visibility := "public"
	calls := 0
	s := &Server{Config: Config{GitHubAppID: "1", GitHubAppPrivateKey: "deliberately unavailable App key"}}
	s.HTTP = &http.Client{Transport: reviewTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Authorization") != "" {
			t.Fatal("public source request gained a credential")
		}
		if r.URL.Host == "codeload.github.com" {
			var buffer bytes.Buffer
			archive := zip.NewWriter(&buffer)
			entry, _ := archive.Create("source-" + commit + "/solver.py")
			entry.Write(data)
			archive.Close()
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(buffer.Bytes())), Header: http.Header{}}, nil
		}
		var value any
		switch r.URL.Path {
		case "/repos/owner/source":
			if !public {
				return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
			}
			value = map[string]any{"id": 123, "full_name": "owner/source", "visibility": visibility, "private": false, "owner": map[string]any{"id": 456}}
		case "/repos/owner/source/commits/" + commit:
			value = map[string]any{"sha": commit, "commit": map[string]any{"tree": map[string]any{"sha": strings.Repeat("b", 40)}}}
		case "/repos/owner/source/git/trees/" + strings.Repeat("b", 40):
			value = map[string]any{"truncated": false, "tree": []any{map[string]any{"path": "solver.py", "mode": "100644", "type": "blob", "sha": blobHash(), "size": len(data)}}}
		case "/repos/owner/source/git/blobs/" + blobHash():
			value = map[string]any{"encoding": "base64", "content": base64.StdEncoding.EncodeToString(data)}
		default:
			t.Fatalf("unexpected GitHub path: %s", r.URL.Path)
		}
		b, _ := json.Marshal(value)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b)), Header: http.Header{}}, nil
	})}
	contract := protocol.SubmissionContract{Format: "source-v2", AllowedPaths: []string{"solver.py"}, AllowedExtensions: []string{".py"}, MaxBytes: 1 << 20, MaxFiles: 1, License: "MIT"}
	first, err := s.fetchSnapshot(context.Background(), "owner/source", commit, &contract)
	if err != nil {
		t.Fatal(err)
	}
	if first.Private || first.RepositoryID != 123 || !bytes.Equal(first.Files["solver.py"], data) || !protocol.ValidDigest(first.Digest) {
		t.Fatal("public snapshot identity lost")
	}
	data = append(data, []byte("# changed\n")...)
	second, err := s.fetchSnapshot(context.Background(), "owner/source", commit, &contract)
	if err != nil || second.Digest == first.Digest {
		t.Fatal("large-file tail not bound")
	}
	for _, mode := range []string{"private", "missing-visibility"} {
		public = mode != "private"
		visibility = ""
		before := calls
		if _, err = s.fetchSnapshot(context.Background(), "owner/source", commit, &contract); err == nil {
			t.Fatal("private/ambiguous source bypassed required installation")
		}
		if calls != before+1 {
			t.Fatal("attempted source fetch after failed permission")
		}
	}
}

func TestPublicArchiveRejectsUnsafeOrAmbiguousMembers(t *testing.T) {
	commit := strings.Repeat("a", 40)
	for _, names := range [][]string{{"repo-" + commit + "/../escape"}, {"repo-" + commit + "/solver.py", "repo-" + commit + "/solver.py"}, {"repo-" + commit + "/solver.py", "repo-" + commit + "/SOLVER.py"}, {"wrong-root/solver.py"}} {
		var buffer bytes.Buffer
		writer := zip.NewWriter(&buffer)
		for _, name := range names {
			w, _ := writer.Create(name)
			w.Write([]byte("test"))
		}
		writer.Close()
		if _, err := decodePublicGitArchive(buffer.Bytes(), commit); err == nil {
			t.Fatal("unsafe archive accepted", names)
		}
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	header := &zip.FileHeader{Name: "repo-" + commit + "/link"}
	header.SetMode(0777 | os.ModeSymlink)
	w, _ := writer.CreateHeader(header)
	w.Write([]byte("/private"))
	writer.Close()
	if _, err := decodePublicGitArchive(buffer.Bytes(), commit); err == nil {
		t.Fatal("symlink accepted")
	}
	if matchesGitBlob([]byte("tampered"), strings.Repeat("a", 40)) {
		t.Fatal("forged tree hash accepted")
	}
}
