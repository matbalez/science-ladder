//go:build linux

package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func (b *candidateBroker) publishProducts() error {
	if len(b.program.Products) == 0 {
		return nil
	}
	const target = "/sl/products"
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	var maximum int64
	for _, p := range b.program.Products {
		maximum += p.MaxBytes
	}
	// Space for small directory metadata in addition to the exact byte budget.
	maximum += int64(len(b.program.Products)) * 65536
	flags := uintptr(syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC)
	if err := syscall.Mount("tmpfs", target, "tmpfs", flags, fmt.Sprintf("size=%d,mode=0755", maximum)); err != nil {
		return err
	}
	if err := copyBuildProducts(filepath.Join(b.root, "work"), target, b.program.Products); err != nil {
		return err
	}
	return syscall.Mount("", target, "", syscall.MS_REMOUNT|syscall.MS_RDONLY|flags, "")
}
