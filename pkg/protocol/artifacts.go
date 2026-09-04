package protocol

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type ArtifactEntry struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Mode   string `json:"mode"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}
type ArtifactTree struct {
	Kind    string          `json:"kind"`
	Version int             `json:"version"`
	Entries []ArtifactEntry `json:"entries"`
}

func ValidatePath(value string) error {
	if value == "" || len(value) > 240 || !utf8.ValidString(value) || !norm.NFC.IsNormalString(value) || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || path.Clean(value) != value || value == "." || value == ".." || strings.HasPrefix(value, "../") {
		return fmt.Errorf("unsafe artifact path %q", value)
	}
	for _, r := range value {
		if unicode.IsControl(r) || r == ':' || r == 0xfffd {
			return errors.New("unsafe path character")
		}
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".git" || strings.HasPrefix(part, ".git") || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return errors.New("reserved path component")
		}
	}
	return nil
}

func allowedFile(name string, c SubmissionContract) bool {
	allowed := false
	for _, prefix := range c.AllowedPaths {
		if prefix == "*" || prefix == name || strings.HasSuffix(prefix, "/") && strings.HasPrefix(name, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return false
	}
	extension := strings.ToLower(path.Ext(name))
	for _, candidate := range c.AllowedExtensions {
		if extension == candidate {
			return true
		}
	}
	return false
}

func rejectActive(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	prefix := trimmed
	if len(prefix) > 512 {
		prefix = prefix[:512]
	}
	lower := bytes.ToLower(prefix)
	if bytes.HasPrefix(data, []byte{0x7f, 'E', 'L', 'F'}) || bytes.HasPrefix(data, []byte("MZ")) || bytes.HasPrefix(trimmed, []byte("#!")) || bytes.HasPrefix(lower, []byte("<svg")) || bytes.HasPrefix(lower, []byte("<!doctype html")) || bytes.HasPrefix(lower, []byte("<html")) || bytes.HasPrefix(lower, []byte("<script")) || bytes.HasPrefix(data, []byte("version https://git-lfs.github.com/spec/v1")) {
		return errors.New("active content or unresolved Git LFS pointer forbidden")
	}
	return nil
}

func treeFor(files map[string][]byte, c SubmissionContract) (ArtifactTree, string, error) {
	tree := ArtifactTree{Kind: "ScienceLadderArtifactTree", Version: 1, Entries: []ArtifactEntry{}}
	if err := ValidateSubmissionContract(c); err != nil {
		return tree, "", err
	}
	if len(files) == 0 || len(files) > c.MaxFiles {
		return tree, "", errors.New("artifact file count outside bounds")
	}
	names := make([]string, 0, len(files))
	seen := map[string]bool{}
	var total int64
	for name, data := range files {
		if err := ValidatePath(name); err != nil {
			return tree, "", err
		}
		fold := cases.Fold().String(name)
		if seen[fold] {
			return tree, "", errors.New("case-colliding artifact paths")
		}
		seen[fold] = true
		if !allowedFile(name, c) {
			return tree, "", fmt.Errorf("file outside submission contract: %s", name)
		}
		total += int64(len(data))
		if total > c.MaxBytes {
			return tree, "", errors.New("artifact exceeds expanded limit")
		}
		if err := rejectActive(data); err != nil {
			return tree, "", err
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data := files[name]
		tree.Entries = append(tree.Entries, ArtifactEntry{Path: name, Type: "file", Mode: "0644", Size: int64(len(data)), Digest: DigestBytes(data)})
	}
	encoded, _ := json.Marshal(tree)
	canonical, err := CanonicalJSON(encoded)
	if err != nil {
		return tree, "", err
	}
	sum := sha256.Sum256(append([]byte("science-ladder-artifact-v1\x00"), canonical...))
	return tree, "sha256:" + hex.EncodeToString(sum[:]), nil
}

// ArtifactFromFiles never interprets data as a program.
func ArtifactFromFiles(files map[string][]byte, c SubmissionContract) (ArtifactTree, string, error) {
	return treeFor(files, c)
}

func CanonicalArtifact(root string, c SubmissionContract) (ArtifactTree, string, error) {
	files := map[string][]byte{}
	var total int64
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ArtifactTree{}, "", errors.New("artifact root must be a real directory")
	}
	err = filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filename == root {
			return nil
		}
		relative, err := filepath.Rel(root, filename)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if err := ValidatePath(relative); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("artifact symlinks forbidden")
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0111 != 0 {
			return errors.New("only non-executable regular files allowed")
		}
		if !allowedFile(relative, c) {
			return fmt.Errorf("file outside submission contract: %s", relative)
		}
		if info.Size() > c.MaxBytes-total {
			return errors.New("artifact exceeds byte limit")
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		total += int64(len(data))
		if total > c.MaxBytes || len(files) >= c.MaxFiles {
			return errors.New("artifact exceeds limits")
		}
		files[relative] = data
		return nil
	})
	if err != nil {
		return ArtifactTree{}, "", err
	}
	return treeFor(files, c)
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// ReadArtifactArchive parses tar/tar.gz without extracting paths to the host.
// It rejects links, special files, executables, duplicate paths and bombs.
func ReadArtifactArchive(input io.Reader, c SubmissionContract) (map[string][]byte, ArtifactTree, string, error) {
	if err := ValidateSubmissionContract(c); err != nil {
		return nil, ArtifactTree{}, "", err
	}
	counted := &countingReader{r: io.LimitReader(input, c.MaxBytes+1)}
	buffered := bufio.NewReader(counted)
	magic, _ := buffered.Peek(2)
	var stream io.Reader = buffered
	compressed := false
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(buffered)
		if err != nil {
			return nil, ArtifactTree{}, "", err
		}
		defer gz.Close()
		stream = gz
		compressed = true
	}
	t := tar.NewReader(io.LimitReader(stream, c.MaxBytes+int64(c.MaxFiles)*4096+1))
	files := map[string][]byte{}
	seen := map[string]bool{}
	var total int64
	fail := func(err error) (map[string][]byte, ArtifactTree, string, error) { return nil, ArtifactTree{}, "", err }
	for {
		header, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fail(err)
		}
		name := strings.TrimSuffix(header.Name, "/")
		if err := ValidatePath(name); err != nil {
			return fail(err)
		}
		fold := cases.Fold().String(name)
		if seen[fold] {
			return fail(errors.New("duplicate or case-colliding archive path"))
		}
		seen[fold] = true
		if len(seen) > c.MaxFiles*4 {
			return fail(errors.New("archive entry limit exceeded"))
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return fail(errors.New("archive links/special files forbidden"))
		}
		if header.Mode&0111 != 0 || header.Size < 0 || header.Size > c.MaxBytes-total || len(files) >= c.MaxFiles {
			return fail(errors.New("archive mode/size limit exceeded"))
		}
		if len(header.PAXRecords) != 0 || len(header.Xattrs) != 0 {
			return fail(errors.New("archive extended metadata forbidden"))
		}
		if !allowedFile(name, c) {
			return fail(errors.New("archive file outside contract"))
		}
		data, err := io.ReadAll(io.LimitReader(t, header.Size+1))
		if err != nil || int64(len(data)) != header.Size {
			return fail(errors.New("truncated archive member"))
		}
		total += int64(len(data))
		files[name] = data
		if compressed && total > 1<<20 && total > counted.n*100 {
			return fail(errors.New("archive decompression ratio exceeds 100"))
		}
	}
	if counted.n > c.MaxBytes {
		return fail(errors.New("compressed archive exceeds limit"))
	}
	tree, digest, err := treeFor(files, c)
	return files, tree, digest, err
}
