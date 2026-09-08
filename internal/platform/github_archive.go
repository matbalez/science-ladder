package platform

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"io"
	"net/http"
	"strings"
)

// Public archives avoid one rate-limited API call per scientific source file.
// Metadata and tree identities still come from GitHub's API, and every selected
// file is checked against its Git blob hash before snapshotting.
func (s *Server) publicGitArchive(ctx context.Context, repository, commit string) (map[string][]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://codeload.github.com/"+repository+"/zip/"+commit, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/zip")
	client := *s.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("public source archive redirect refused")
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fail(422, "public_archive_unavailable", "GitHub could not provide the exact public source archive")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (64<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 64<<20 {
		return nil, fail(422, "snapshot_too_large", "Public repository archive exceeds 64 MiB")
	}
	return decodePublicGitArchive(data, commit)
}
func decodePublicGitArchive(data []byte, commit string) (map[string][]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	if len(archive.File) > 10000 {
		return nil, errors.New("public source archive has too many entries")
	}
	files := map[string][]byte{}
	seen := map[string]bool{}
	root := ""
	var expanded uint64
	for _, entry := range archive.File {
		parts := strings.SplitN(entry.Name, "/", 2)
		if len(parts) != 2 || !strings.HasSuffix(parts[0], "-"+commit) || protocol.ValidatePath(parts[0]) != nil {
			return nil, errors.New("public source archive root mismatch")
		}
		if root == "" {
			root = parts[0]
		}
		if root != parts[0] {
			return nil, errors.New("multiple public archive roots")
		}
		name := strings.TrimSuffix(parts[1], "/")
		if name == "" && entry.FileInfo().IsDir() {
			continue
		}
		if protocol.ValidatePath(name) != nil {
			return nil, errors.New("unsafe public archive path")
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if !entry.Mode().IsRegular() || seen[strings.ToLower(name)] {
			return nil, errors.New("public source archive contains a link or duplicate")
		}
		seen[strings.ToLower(name)] = true
		expanded += entry.UncompressedSize64
		if entry.UncompressedSize64 > 20<<20 || expanded > 64<<20 {
			return nil, errors.New("public source archive expansion exceeds limit")
		}
		reader, err := entry.Open()
		if err != nil {
			return nil, err
		}
		content, readErr := io.ReadAll(io.LimitReader(reader, int64(entry.UncompressedSize64)+1))
		reader.Close()
		if readErr != nil || uint64(len(content)) != entry.UncompressedSize64 {
			return nil, errors.New("public archive member size or checksum mismatch")
		}
		files[name] = content
	}
	return files, nil
}
func matchesGitBlob(data []byte, want string) bool {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d%c", len(data), 0)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil)) == want
}
