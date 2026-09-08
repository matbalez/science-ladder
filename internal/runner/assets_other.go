//go:build !linux

package runner

import "errors"

func mountAssetDisks(root string, disks []AssetDisk) (func() error, error) {
	if len(disks) != 0 {
		return func() error { return nil }, errors.New("asset mounting requires the Linux executor")
	}
	return func() error { return nil }, nil
}
