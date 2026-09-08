//go:build linux

package runner

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
)

// CandidateSetup runs only as PID 1 in the broker-created candidate PID, mount,
// IPC, network and UTS namespaces, already chrooted to the candidate filesystem.
// No candidate source runs until privileges are dropped and the existing native
// seccomp launcher has installed its filter. This is not a setuid executable.
func CandidateSetup(args []string) error {
	if os.Getpid() != 1 || os.Getuid() != 0 || os.Geteuid() != 0 || len(args) < 2 {
		return errors.New("candidate setup requires fresh privileged namespace init")
	}
	runtime.LockOSThread()
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("private mount propagation: %w", err)
	}
	// subset=pid excludes kernel-wide proc files; this proc superblock is bound
	// to the new PID namespace, never a bind mount of the checker's /proc.
	if err := syscall.Mount("proc", "/proc", "proc", syscall.MS_RDONLY|syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, "hidepid=2,subset=pid"); err != nil {
		return fmt.Errorf("private process view: %w", err)
	}
	if err := syscall.Setgroups([]int{}); err != nil {
		return err
	}
	if err := syscall.Setresgid(candidateUID, candidateUID, candidateUID); err != nil {
		return err
	}
	if err := syscall.Setresuid(candidateUID, candidateUID, candidateUID); err != nil {
		return err
	}
	if os.Getuid() != candidateUID || os.Geteuid() != candidateUID {
		return errors.New("candidate privilege drop failed")
	}
	argv := append([]string{"/usr/local/bin/sl-candidate-sandbox"}, args...)
	return syscall.Exec(argv[0], argv, os.Environ())
}
