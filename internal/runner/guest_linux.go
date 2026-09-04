//go:build linux

package runner

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

// GuestInit is installed as /sbin/sl-init in a pinned read-only guest rootfs.
// It is never invoked by the hosted API. Kernel+rootfs profile review is required.
func GuestInit() error {
	if os.Getpid() != 1 || os.Geteuid() != 0 {
		return errors.New("guest init only runs as guest PID 1")
	}
	for _, directory := range []string{"/proc", "/sys", "/dev", "/sl/validator", "/sl/submission", "/sl/suite", "/sl/challenge", "/sl/config", "/sl/work", "/sl/output", "/tmp"} {
		if err := os.MkdirAll(directory, 0755); err != nil {
			return err
		}
	}
	for _, mount := range []struct {
		source, target, kind string
		flags                uintptr
		data                 string
	}{{"proc", "/proc", "proc", syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC, ""}, {"sysfs", "/sys", "sysfs", syscall.MS_RDONLY | syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC, ""}, {"devtmpfs", "/dev", "devtmpfs", syscall.MS_NOSUID, ""}} {
		if err := syscall.Mount(mount.source, mount.target, mount.kind, mount.flags, mount.data); err != nil {
			return err
		}
	}
	for _, disk := range []struct{ device, target string }{{"/dev/vdb", "/sl/validator"}, {"/dev/vdc", "/sl/submission"}, {"/dev/vdd", "/sl/suite"}, {"/dev/vde", "/sl/challenge"}, {"/dev/vdf", "/sl/config"}} {
		if err := syscall.Mount(disk.device, disk.target, "squashfs", syscall.MS_RDONLY|syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, ""); err != nil {
			return err
		}
	}
	for _, directory := range []string{"/sl/work", "/sl/output", "/tmp"} {
		if err := syscall.Mount("tmpfs", directory, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, "size=64m,mode=1777"); err != nil {
			return err
		}
	}
	data, err := os.ReadFile("/sl/config/manifest.json")
	if err != nil {
		return err
	}
	manifest, err := protocol.ParseManifest(data)
	if err != nil {
		return err
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{Cur: 128, Max: 128}); err != nil {
		return err
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 65536, Max: 65536}); err != nil {
		return err
	}
	if _, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, 38, 1, 0, 0, 0, 0); errno != 0 {
		return errno
	}
	if err := os.MkdirAll("/sys/fs/cgroup", 0755); err != nil {
		return err
	}
	if err := syscall.Mount("cgroup2", "/sys/fs/cgroup", "cgroup2", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, ""); err != nil {
		return err
	}
	if err := os.WriteFile("/sys/fs/cgroup/cgroup.subtree_control", []byte("+pids +memory"), 0600); err != nil {
		return err
	}
	if err := os.Mkdir("/sys/fs/cgroup/validator", 0700); err != nil {
		return err
	}
	if err := os.WriteFile("/sys/fs/cgroup/validator/pids.max", []byte("64"), 0600); err != nil {
		return err
	}
	if err := os.WriteFile("/sys/fs/cgroup/validator/memory.max", []byte(fmt.Sprint(int64(manifest.Resources.MemoryMB-64)*1024*1024)), 0600); err != nil {
		return err
	}
	cgroup, err := os.Open("/sys/fs/cgroup/validator")
	if err != nil {
		return err
	}
	defer cgroup.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(manifest.Resources.TimeoutSeconds)*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, manifest.Validator.Entrypoint[0], manifest.Validator.Entrypoint[1:]...)
	command.Dir = "/sl/challenge"
	command.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/sl/work", "PYTHONPATH=/sl/validator/site-packages", "PYTHONHASHSEED=0", "PYTHONDONTWRITEBYTECODE=1", "TZ=UTC", "LC_ALL=C.UTF-8", "OPENBLAS_NUM_THREADS=1", "OMP_NUM_THREADS=1", "SOURCE_DATE_EPOCH=0"}
	command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}, Setpgid: true, UseCgroupFD: true, CgroupFD: int(cgroup.Fd())}
	log := &boundedBuffer{max: 65536}
	command.Stdout = log
	command.Stderr = log
	err = command.Run()
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	if ctx.Err() != nil {
		return guestFailure("resource_limit")
	}
	if err != nil {
		return guestFailure("challenge_fault")
	}
	entries, err := os.ReadDir("/sl/output")
	if err != nil || len(entries) != 1 || entries[0].Name() != "result.json" {
		return guestFailure("invalid_output")
	}
	filename := filepath.Join("/sl/output", "result.json")
	info, err := os.Lstat(filename)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		return guestFailure("invalid_output")
	}
	file, err := os.Open(filename)
	if err != nil {
		return guestFailure("invalid_output")
	}
	result, err := io.ReadAll(io.LimitReader(file, 65537))
	_ = file.Close()
	if err != nil {
		return guestFailure("invalid_output")
	}
	if _, _, err := protocol.ValidateResult(result, manifest); err != nil {
		return guestFailure("invalid_output")
	}
	fmt.Println("SL_RESULT " + base64.StdEncoding.EncodeToString(result))
	syscall.Sync()
	return syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
}

func guestFailure(outcome string) error {
	fmt.Println("SL_ERROR " + outcome)
	syscall.Sync()
	return syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
}
