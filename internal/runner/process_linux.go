//go:build linux

package runner

import (
	"os/exec"
	"syscall"
)

// Spawn the Jailer directly as PID 1 in its own namespace; its exec into the VMM
// preserves the process handle. Killing that handle also destroys the namespace,
// avoiding orphan VMMs from a separate daemonizing/new-pid-ns wrapper process.
func isolateJailerProcess(command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWIPC | syscall.CLONE_NEWUTS, Pdeathsig: syscall.SIGKILL}
	return nil
}
