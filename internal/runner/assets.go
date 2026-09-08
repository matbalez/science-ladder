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

func assetGuestPath(a protocol.EvaluationAsset) string {
	if a.Domain == "candidate" {
		return filepath.Join("/opt/sl-private/assets", a.Name)
	}
	return filepath.Join("/sl/assets", a.Name)
}

// Asset components are part of the same signed runtime inventory and advisory
// coverage. A candidate-only disposition cannot cover checker-visible tools.
func validateAssetInventory(c Config, inventory RuntimeInventory) error {
	if len(c.Assets) != len(inventory.Assets) {
		return errors.New("asset component inventory does not match configured disks")
	}
	packages := map[string]PackageCoordinate{}
	for _, p := range inventory.Packages {
		packages[packageKey(p)] = p
	}
	seen := map[string]bool{}
	for _, binding := range inventory.Assets {
		if !protocol.ValidDigest(binding.ComponentInventoryDigest) || seen[binding.Asset.Name] {
			return errors.New("invalid asset component inventory binding")
		}
		seen[binding.Asset.Name] = true
		matched := false
		for _, disk := range c.Assets {
			matched = matched || disk.Asset == binding.Asset
		}
		if !matched {
			return errors.New("asset inventory is for different immutable bytes or disclosure policy")
		}
		if binding.Asset.Domain != "" && len(binding.PackageKeys) == 0 {
			return errors.New("executable asset lacks package coverage")
		}
		keys := map[string]bool{}
		for _, key := range binding.PackageKeys {
			p, ok := packages[key]
			if !ok || keys[key] {
				return errors.New("asset package coverage is missing or repeated")
			}
			keys[key] = true
			if binding.Asset.Domain != "candidate" && p.ExecutionDomain == "candidate-only" {
				return errors.New("checker-visible asset was classified candidate-only")
			}
		}
	}
	return nil
}
