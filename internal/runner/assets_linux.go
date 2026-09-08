//go:build linux

package runner

import (
	"os"
	"path/filepath"
	"syscall"
)

func mountAssetDisks(root string, disks []AssetDisk) (func() error, error) {
	mounted := []string{}
	cleanup := func() error {
		for len(mounted) > 0 {
			i := len(mounted) - 1
			if err := syscall.Unmount(mounted[i], 0); err != nil {
				return err
			}
			mounted = mounted[:i]
		}
		return nil
	}
	for i, d := range disks {
		target := filepath.Join(root, assetDiskFilename(i))
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL, 0444)
		if err == nil {
			err = f.Close()
		}
		if err != nil {
			_ = cleanup()
			return cleanup, err
		}
		if err = bindReadOnly(d.File.Path, target); err != nil {
			_ = syscall.Unmount(target, 0)
			_ = cleanup()
			return cleanup, err
		}
		mounted = append(mounted, target)
	}
	return cleanup, nil
}
