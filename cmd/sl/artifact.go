package main

import (
	"errors"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"io/fs"
	"os"
	"path/filepath"
)

// localArtifact matches a GitHub source snapshot: root Git metadata is not an
// artifact. Everything else, including untracked files, must satisfy the contract.
// The protocol/archive parser stays strict and never accepts .git in uploads.
func localArtifact(root string, c protocol.SubmissionContract) (protocol.ArtifactTree, string, error) {
	files := map[string][]byte{}
	var total int64
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return protocol.ArtifactTree{}, "", errors.New("artifact root must be a real directory")
	}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ".git" && d.Type()&os.ModeSymlink == 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if info.Mode().IsRegular() {
				return nil
			}
		}
		if err = protocol.ValidatePath(rel); err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("artifact symlinks forbidden")
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0111 != 0 {
			return errors.New("only non-executable regular artifact files allowed")
		}
		if info.Size() > c.MaxBytes-total || len(files) >= c.MaxFiles {
			return errors.New("artifact exceeds limits")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		total += int64(len(data))
		if total > c.MaxBytes {
			return errors.New("artifact exceeds byte limit")
		}
		files[rel] = data
		return nil
	})
	if err != nil {
		return protocol.ArtifactTree{}, "", err
	}
	return protocol.ArtifactFromFiles(files, c)
}
