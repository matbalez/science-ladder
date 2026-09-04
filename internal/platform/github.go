package platform

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"regexp"
	"strings"
	"time"
)

var repoRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var commitRE = regexp.MustCompile(`^[a-f0-9]{40}$`)

type Snapshot struct {
	OwnerGitHubID int64
	RepositoryID  int64
	Commit        string
	Private       bool
	Files         map[string][]byte
	Digest        string
}

func (s *Server) appJWT() (string, error) {
	if s.Config.GitHubAppID == "" || s.Config.GitHubAppPrivateKey == "" {
		return "", nil
	}
	block, _ := pem.Decode([]byte(s.Config.GitHubAppPrivateKey))
	if block == nil {
		return "", errors.New("GitHub App private key is not PEM")
	}
	var key *rsa.PrivateKey
	if k, e := x509.ParsePKCS1PrivateKey(block.Bytes); e == nil {
		key = k
	} else {
		k, e := x509.ParsePKCS8PrivateKey(block.Bytes)
		if e != nil {
			return "", errors.New("invalid GitHub App private key")
		}
		var ok bool
		key, ok = k.(*rsa.PrivateKey)
		if !ok {
			return "", errors.New("GitHub App private key must be RSA")
		}
	}
	enc := base64.RawURLEncoding.EncodeToString
	head := enc(raw(map[string]string{"alg": "RS256", "typ": "JWT"}))
	body := enc(raw(map[string]any{"iat": time.Now().Unix() - 60, "exp": time.Now().Unix() + 540, "iss": s.Config.GitHubAppID}))
	message := head + "." + body
	digest := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return message + "." + enc(sig), nil
}
func (s *Server) installationToken(ctx context.Context, repository string) (string, error) {
	jwt, err := s.appJWT()
	if err != nil {
		return "", err
	}
	if jwt == "" {
		return "", nil
	}
	var installation struct {
		ID int64 `json:"id"`
	}
	if err = s.github(ctx, "GET", "/repos/"+repository+"/installation", jwt, nil, &installation); err != nil {
		return "", err
	}
	var t struct {
		Token string `json:"token"`
	}
	if err = s.github(ctx, "POST", "/app/installations/"+formatInt(installation.ID)+"/access_tokens", jwt, map[string]any{"repositories": []string{strings.Split(repository, "/")[1]}, "permissions": map[string]string{"contents": "read", "metadata": "read"}}, &t); err != nil {
		return "", err
	}
	if t.Token == "" {
		return "", errors.New("GitHub installation token missing")
	}
	return t.Token, nil
}
func (s *Server) fetchSnapshot(ctx context.Context, repository, commit string, contract *protocol.SubmissionContract) (Snapshot, error) {
	var result Snapshot
	if !repoRE.MatchString(repository) || !commitRE.MatchString(commit) {
		return result, fail(400, "invalid_github_ref", "Use owner/repository and the complete 40-character lowercase Git commit SHA")
	}
	token, err := s.installationToken(ctx, repository)
	if err != nil {
		return result, err
	}
	var repo struct {
		Owner struct {
			ID int64 `json:"id"`
		} `json:"owner"`
		ID      int64 `json:"id"`
		Private bool  `json:"private"`
	}
	if err = s.github(ctx, "GET", "/repos/"+repository, token, nil, &repo); err != nil {
		return result, err
	}
	var resolved struct {
		SHA    string `json:"sha"`
		Commit struct {
			Tree struct {
				SHA string `json:"sha"`
			} `json:"tree"`
		} `json:"commit"`
	}
	if err = s.github(ctx, "GET", "/repos/"+repository+"/commits/"+commit, token, nil, &resolved); err != nil {
		return result, err
	}
	if resolved.SHA != commit {
		return result, fail(422, "commit_mismatch", "GitHub did not resolve the exact requested commit")
	}
	var tree struct {
		Truncated bool `json:"truncated"`
		Tree      []struct {
			Path, Mode, Type, SHA string
			Size                  int64
		} `json:"tree"`
	}
	if err = s.github(ctx, "GET", "/repos/"+repository+"/git/trees/"+resolved.Commit.Tree.SHA+"?recursive=1", token, nil, &tree); err != nil {
		return result, err
	}
	if tree.Truncated || len(tree.Tree) > 5000 {
		return result, fail(422, "repository_too_large", "Repository exceeds snapshot limits")
	}
	files := map[string][]byte{}
	var total int64
	for _, entry := range tree.Tree {
		if entry.Type == "tree" {
			continue
		}
		if entry.Type != "blob" || entry.Mode == "120000" || entry.Mode == "160000" {
			return result, fail(422, "unsafe_git_tree", "Symbolic links and submodules are not accepted")
		}
		if protocol.ValidatePath(entry.Path) != nil {
			return result, fail(422, "unsafe_path", "Repository contains an unsafe path")
		}
		if contract != nil {
			allowed := false
			for _, path := range contract.AllowedPaths {
				prefix := path
				if prefix == "*" || entry.Path == prefix || strings.HasSuffix(prefix, "/") && strings.HasPrefix(entry.Path, prefix) {
					allowed = true
				}
			}
			if !allowed {
				continue
			}
			if entry.Mode != "100644" {
				return result, fail(422, "artifact_mode_invalid", "Submitted artifacts cannot carry executable file modes")
			}
		}
		if entry.Size > 20<<20 || total+entry.Size > 40<<20 {
			return result, fail(422, "snapshot_too_large", "Snapshot exceeds the configured byte limit")
		}
		var blob struct {
			Content, Encoding string
			Size              int64
		}
		if err = s.github(ctx, "GET", "/repos/"+repository+"/git/blobs/"+entry.SHA, token, nil, &blob); err != nil {
			return result, err
		}
		if blob.Encoding != "base64" {
			return result, fail(422, "unsupported_git_blob", "GitHub blob encoding is not supported")
		}
		data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
		if err != nil {
			return result, err
		}
		if int64(len(data)) != entry.Size {
			return result, fail(422, "git_blob_mismatch", "GitHub blob size did not match its tree descriptor")
		}
		if bytes.HasPrefix(data, []byte("version https://git-lfs.github.com/spec/v1")) {
			return result, fail(422, "git_lfs_unresolved", "Resolve Git LFS pointers before submission")
		}
		if bytes.Contains(data, []byte("-----BEGIN PRIVATE KEY-----")) || bytes.Contains(data, []byte("-----BEGIN RSA PRIVATE KEY-----")) {
			return result, fail(422, "credential_detected", "A private-key marker was detected in repository content")
		}
		files[entry.Path] = data
		total += int64(len(data))
	}
	if len(files) == 0 {
		return result, fail(422, "empty_snapshot", "No files matched the artifact contract")
	}
	encoded := map[string]string{}
	for p, b := range files {
		encoded[p] = base64.StdEncoding.EncodeToString(b)
	}
	digest, err := protocol.Digest(map[string]any{"kind": "GitSourceSnapshot", "repositoryId": repo.ID, "commit": commit, "files": encoded})
	if err != nil {
		return result, err
	}
	return Snapshot{OwnerGitHubID: repo.Owner.ID, RepositoryID: repo.ID, Commit: commit, Private: repo.Private, Files: files, Digest: digest}, nil
}
func formatInt(n int64) string { b, _ := json.Marshal(n); return string(b) }
func snapshotBytes(snapshot Snapshot) []byte {
	files := map[string]string{}
	for p, b := range snapshot.Files {
		files[p] = base64.StdEncoding.EncodeToString(b)
	}
	return raw(map[string]any{"repositoryId": snapshot.RepositoryID, "sourceCommit": snapshot.Commit, "files": files})
}
func artifactBytes(snapshot Snapshot, contract protocol.SubmissionContract) ([]byte, string, error) {
	tree, digest, err := protocol.ArtifactFromFiles(snapshot.Files, contract)
	if err != nil {
		return nil, "", err
	}
	files := map[string]string{}
	for p, b := range snapshot.Files {
		files[p] = base64.StdEncoding.EncodeToString(b)
	}
	return raw(map[string]any{"tree": tree, "files": files}), digest, nil
}
