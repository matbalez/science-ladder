package runner

import (
	"errors"
	"fmt"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"os"
	"path/filepath"
)

// Asset disks are provisioned by the operator, never downloaded from a creator
// URL. Their exact bytes, disclosure policy and inventory are signed with the
// execution profile. Large immutable disks do not get copied into guest tmpfs.
type AssetDisk struct {
	Asset protocol.EvaluationAsset `json:"asset"`
	File  PinnedFile               `json:"file"`
}

func selectedAssetDisks(m protocol.Manifest, c Config) ([]AssetDisk, error) {
	if m.Evaluation == nil || len(m.Evaluation.Assets) == 0 {
		return nil, nil
	}
	if c.Capabilities == nil {
		return nil, errors.New("assets require enrolled capabilities")
	}
	selected := []AssetDisk{}
	for _, wanted := range m.Evaluation.Assets {
		found := false
		for _, disk := range c.Assets {
			if disk.Asset != wanted {
				continue
			}
			if found || disk.File.Digest != wanted.Digest || !filepath.IsAbs(disk.File.Path) {
				return nil, errors.New("ambiguous or inconsistent asset pin")
			}
			if err := verifyPinned(disk.File); err != nil {
				return nil, err
			}
			info, err := os.Lstat(disk.File.Path)
			if err != nil || !info.Mode().IsRegular() || info.Size() != wanted.Size || info.Mode().Perm()&0222 != 0 || info.Mode().Perm()&0004 == 0 {
				return nil, errors.New("asset requires exact-size read-only regular disk")
			}
			if err := rootOwnedHierarchy(disk.File.Path); err != nil {
				return nil, err
			}
			selected = append(selected, disk)
			found = true
		}
		if !found {
			return nil, fmt.Errorf("immutable asset %s is not provisioned", wanted.Name)
		}
	}
	return selected, nil
}

func assetDiskFilename(i int) string { return fmt.Sprintf("asset-%02d.squashfs", i) }
