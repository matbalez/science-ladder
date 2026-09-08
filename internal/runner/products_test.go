package runner

import (
	"github.com/matbalez/science-ladder/pkg/protocol"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildProductsRejectLinksAndOversize(t *testing.T) {
	for _, kind := range []string{"regular", "oversize", "symlink", "parent-link", "directory", "missing"} {
		t.Run(kind, func(t *testing.T) {
			source, target := t.TempDir(), t.TempDir()
			name := "proof.olean"
			switch kind {
			case "regular", "oversize":
				os.WriteFile(filepath.Join(source, name), []byte("sealed proof bytes"), 0700)
			case "symlink":
				os.Symlink("/etc/passwd", filepath.Join(source, name))
			case "parent-link":
				os.Symlink(t.TempDir(), filepath.Join(source, "escape"))
				name = "escape/proof.olean"
			case "directory":
				os.Mkdir(filepath.Join(source, name), 0755)
			}
			limit := int64(100)
			if kind == "oversize" {
				limit = 2
			}
			err := copyBuildProducts(source, target, []protocol.BuildProduct{{Path: name, MaxBytes: limit}})
			if kind == "regular" {
				if err != nil {
					t.Fatal(err)
				}
				info, _ := os.Stat(filepath.Join(target, name))
				if info.Mode().Perm()&0222 != 0 || info.Mode().Perm()&0111 != 0 {
					t.Fatal("product was writable or executable")
				}
			} else if err == nil {
				t.Fatal("unsafe product accepted")
			}
		})
	}
}
