//go:build linux

package runner

import (
	"errors"
	"syscall"
	"testing"
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
