//go:build linux

package runner

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestGuestKernelCreatedMountRequiresTypeAndFlags(t *testing.T) {
	for _, tc := range []struct {
		name       string
		kind       int64
		remountErr error
		wantErr    bool
	}{
		{"kernel devtmpfs", 0x01021994, nil, false},
		{"wrong filesystem", 0xEF53, nil, true},
		{"cannot enforce flags", 0x01021994, syscall.EPERM, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			mount := func(source, target, kind string, flags uintptr, data string) error {
				calls++
				if calls == 1 {
					return syscall.EBUSY
				}
				if flags != syscall.MS_NOSUID|syscall.MS_REMOUNT || target != "/dev" || kind != "devtmpfs" {
					t.Fatal("remount lost enforced policy")
				}
				return tc.remountErr
			}
			stat := func(path string, value *syscall.Statfs_t) error { value.Type = tc.kind; return nil }
			err := mountGuestFilesystem("devtmpfs", "/dev", "devtmpfs", syscall.MS_NOSUID, "", mount, stat)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected mount error: %v", err)
			}
			if tc.kind == 0xEF53 && calls != 1 {
				t.Fatal("remounted unrelated filesystem")
			}
		})
	}
	unexpected := errors.New("kernel refused mount")
	err := mountGuestFilesystem("devtmpfs", "/dev", "devtmpfs", syscall.MS_NOSUID, "", func(string, string, string, uintptr, string) error { return unexpected }, func(string, *syscall.Statfs_t) error {
		t.Fatal("non-busy error inspected existing filesystem")
		return nil
	})
	if !errors.Is(err, unexpected) {
		t.Fatal("lost initial mount failure")
	}
}

func TestGuestCgroupTeardownWaitsForAllDescendants(t *testing.T) {
	dir := t.TempDir()
	events := filepath.Join(dir, "cgroup.events")
	if err := os.WriteFile(events, []byte("populated 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	updated := make(chan error, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		updated <- os.WriteFile(events, []byte("populated 0\n"), 0600)
	}()
	if err := killGuestValidatorCgroup(dir); err != nil {
		t.Fatal(err)
	}
	if err := <-updated; err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "cgroup.kill"))
	if err != nil || string(data) != "1" {
		t.Fatal("whole-cgroup kill was not requested")
	}
	if err := os.Remove(events); err != nil {
		t.Fatal(err)
	}
	if err := killGuestValidatorCgroup(dir); err == nil {
		t.Fatal("cleanup accepted missing completion evidence")
	}
}

func TestGuestResourceFailureUsesKernelEvents(t *testing.T) {
	for _, tc := range []struct {
		name, memory, pids string
		limited, wantErr   bool
	}{
		{"ordinary checker error", "oom_kill 0\n", "max 0\n", false, false},
		{"memory limit", "oom_kill 1\n", "max 0\n", true, false},
		{"process limit", "oom_kill 0\n", "max 1\n", true, false},
		{"invalid counter", "oom_kill invalid\n", "max 0\n", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "memory.events"), []byte(tc.memory), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "pids.events"), []byte(tc.pids), 0600); err != nil {
				t.Fatal(err)
			}
			limited, err := guestResourceLimitExceeded(dir, errors.New("checker failed"))
			if limited != tc.limited || (err != nil) != tc.wantErr {
				t.Fatalf("limited=%v error=%v", limited, err)
			}
		})
	}
}
