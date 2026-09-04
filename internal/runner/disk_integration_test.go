package runner

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func TestIsolationManifestHasIndependentFixtureContract(t *testing.T) {
	parent := hostProbeManifest("sha256:" + strings.Repeat("a", 64))
	parent.Metric.BaselineTicks = "17996"
	parent.Metric.Direction = "minimize"
	parent.Metric.DomainMinTicks = "256"
	parent.Metric.DomainMaxTicks = "44608256"
	parent.Fixtures[0].ExpectedTicks = "17996"
	parent.Suite = protocol.Suite{Visibility: "hidden", Commitment: "sha256:" + strings.Repeat("b", 64)}
	parent.Resources.MemoryMB = 128
	parent.HardGates = []string{"creator-gate"}
	got := isolationManifest(parent)
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := protocol.ParseManifest(data); err != nil {
		t.Fatalf("fixed isolation contract cannot boot: %v", err)
	}
	if got.Validator.RuntimeImageDigest != parent.Validator.RuntimeImageDigest || got.Suite.Visibility != "public" || got.Metric.BaselineTicks != "0" || got.Fixtures[0].ExpectedTicks != "0" || got.Resources.MemoryMB != 256 || len(got.HardGates) != 1 || got.HardGates[0] != "isolation" {
		t.Fatal("creator fields leaked into the fixed platform corpus")
	}
}

// Run on Linux with the actual pinned filesystem tools. This reproduces the
// service's restrictive source modes and checks the resulting filesystem, not
// merely command arguments. No process-global umask mutation is needed.
func TestDiskNormalizesPrivateSourceModes(t *testing.T) {
	if os.Getenv("SL_TEST_FILESYSTEM_TOOLS") != "1" {
		t.Skip("set SL_TEST_FILESYSTEM_TOOLS=1 with mksquashfs and unsquashfs installed")
	}
	maker, err := exec.LookPath("mksquashfs")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := exec.LookPath("unsquashfs")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(maker)
	if err != nil {
		t.Fatal(err)
	}
	b := Builder{MakeSquashFS: PinnedFile{Path: maker, Digest: protocol.DigestBytes(data)}}
	workspace := t.TempDir()
	var digests []string
	for index, private := range []bool{true, false} {
		root := filepath.Join(workspace, []string{"private", "public"}[index])
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		if err := writeTree(root, map[string][]byte{"nested/input.txt": []byte("data-only"), "checker.py": []byte("# test-only")}); err != nil {
			t.Fatal(err)
		}
		err := filepath.Walk(root, func(name string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			mode := os.FileMode(0644)
			if info.IsDir() {
				mode = 0755
			}
			if private {
				mode &= 0700
			}
			return os.Chmod(name, mode)
		})
		if err != nil {
			t.Fatal(err)
		}
		image := root + ".squashfs"
		ref, err := b.disk(context.Background(), root, image)
		if err != nil {
			t.Fatal(err)
		}
		digests = append(digests, ref.Digest)
		listing, err := exec.Command(reader, "-ll", image).CombinedOutput()
		if err != nil {
			t.Fatalf("inspect built image: %v: %s", err, listing)
		}
		for _, line := range strings.Split(string(listing), "\n") {
			if strings.Contains(line, "squashfs-root") && !strings.HasPrefix(line, "drwxr-xr-x root/root") && !strings.HasPrefix(line, "-rw-r--r-- root/root") {
				t.Fatalf("noncanonical guest inode: %s", line)
			}
		}
		if private {
			info, _ := os.Stat(filepath.Join(root, "nested/input.txt"))
			if info.Mode().Perm() != 0600 {
				t.Fatal("building guest disk changed private host source permissions")
			}
		}
	}
	if digests[0] != digests[1] {
		t.Fatal("filesystem digest depends on host source permissions")
	}
}
