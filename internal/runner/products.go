package runner

import (
	"errors"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Caller has stopped all writers and sealed source. Only declared regular files
// cross the boundary; links, directories, devices and oversized outputs fail.
func copyBuildProducts(source, target string, products []protocol.BuildProduct) error {
	for _, product := range products {
		if err := protocol.ValidatePath(product.Path); err != nil {
			return err
		}
		parts := strings.Split(product.Path, "/")
		name := source
		for i, part := range parts {
			name = filepath.Join(name, part)
			info, err := os.Lstat(name)
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				return errors.New("missing or linked build product")
			}
			if i < len(parts)-1 && !info.IsDir() {
				return errors.New("invalid product parent")
			}
			if i == len(parts)-1 && (!info.Mode().IsRegular() || info.Size() > product.MaxBytes) {
				return errors.New("invalid or oversized build product")
			}
		}
		if err := copyBuildProduct(name, filepath.Join(target, filepath.FromSlash(product.Path)), product.MaxBytes); err != nil {
			return err
		}
	}
	return nil
}
func copyBuildProduct(source, target string, limit int64) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0444)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(out, io.LimitReader(in, limit+1))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n > limit {
		return errors.New("product exceeds frozen limit")
	}
	return nil
}
