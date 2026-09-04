//go:build !linux

package runner

import (
	"errors"
	"os/exec"
)

func isolateJailerProcess(command *exec.Cmd) error {
	return errors.New("Jailer PID/IPC/UTS namespaces require Linux")
}
