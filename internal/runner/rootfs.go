package runner

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed assets/rootfs-build.sh
var rootfsScript []byte

//go:embed assets/native-rootfs-build.sh
var nativeRootfsScript []byte

// BuildRootFS composes a pinned guest filesystem from approved platform images.
// It runs no creator code. The filesystem-tools image must contain tar, coreutils
// and mke2fs; both OCI references must include immutable SHA-256 digests.
func BuildRootFS(ctx context.Context, pythonImage, toolsImage, guestInit, output string) error {
	return BuildRootFSForProfile(ctx, pythonImage, toolsImage, guestInit, output, "artifact-checker-v1")
}

func BuildRootFSForProfile(ctx context.Context, pythonImage, toolsImage, guestInit, output, profile string) error {
	script := rootfsScript
	maximum := int64(1 << 30)
	if profile == "native-evaluator-v2" {
		script = nativeRootfsScript
		maximum = 4 << 30
	} else if profile != "artifact-checker-v1" {
		return errors.New("unsupported rootfs build profile")
	}
	for _, reference := range []string{pythonImage, toolsImage} {
		parts := strings.Split(reference, "@")
		if len(parts) != 2 || !strings.HasPrefix(parts[1], "sha256:") || len(parts[1]) != 71 || strings.ContainsAny(reference, " \n\r") {
			return errors.New("rootfs image references require immutable OCI digests")
		}
	}
	info, err := os.Stat(guestInit)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("compiled Linux amd64 guest init required")
	}
	guestInit, err = filepath.Abs(guestInit)
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	if err := os.Mkdir(output, 0700); err != nil {
		return errors.New("rootfs output directory must be new")
	}
	work, err := os.MkdirTemp("", "sl-rootfs-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if err := os.WriteFile(filepath.Join(work, "build.sh"), script, 0400); err != nil {
		return err
	}
	initBytes, err := os.ReadFile(guestInit)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(work, "sl-init"), initBytes, 0500); err != nil {
		return err
	}
	name := fmt.Sprintf("sl-rootfs-source-%d", time.Now().UnixNano())
	logs := &boundedBuffer{max: 65536}
	run := func(args ...string) error {
		command := exec.CommandContext(ctx, "docker", args...)
		command.Stdout = logs
		command.Stderr = logs
		return command.Run()
	}
	if err := run("create", "--name", name, "--platform=linux/amd64", pythonImage); err != nil {
		return fmt.Errorf("create pinned runtime image: %w", err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = exec.CommandContext(cleanup, "docker", "rm", "-f", name).Run()
	}()
	if err := run("export", "--output", filepath.Join(work, "base.tar"), name); err != nil {
		return err
	}
	for _, value := range []string{work, guestInit, output} {
		if strings.ContainsAny(value, ",\n\r") {
			return errors.New("unsupported build mount path")
		}
	}
	args := []string{"run", "--rm", "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--pids-limit=128", "--memory=3g", "--cpus=2", "--platform=linux/amd64", "--tmpfs", "/work:rw,nosuid,nodev,size=2g", "--tmpfs", "/tmp:rw,nosuid,nodev,size=64m", "--mount", "type=bind,src=" + work + ",dst=/input,readonly", "--mount", "type=bind,src=" + output + ",dst=/output", "--entrypoint", "/bin/sh", toolsImage, "/input/build.sh"}
	if profile == "native-evaluator-v2" {
		// This step handles only platform-owned image bytes. Use a disposable
		// Linux volume: macOS shared-directory inode behavior breaks GNU tar's
		// directory integrity checks, and a full native tree need not live in RAM.
		buildWork := fmt.Sprintf("sl-native-rootfs-work-%d", time.Now().UnixNano())
		if err := run("volume", "create", buildWork); err != nil {
			return err
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = exec.CommandContext(cleanup, "docker", "volume", "rm", buildWork).Run()
		}()
		for i := range args {
			if args[i] == "/work:rw,nosuid,nodev,size=2g" {
				args[i-1] = "--mount"
				args[i] = "type=volume,src=" + buildWork + ",dst=/work"
			}
		}
	}
	if err := run(args...); err != nil {
		return fmt.Errorf("platform rootfs composition failed: %w: %s", err, logs.b.String())
	}
	image := filepath.Join(output, "rootfs.ext4")
	info, err = os.Lstat(image)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maximum {
		return errors.New("invalid composed rootfs")
	}
	return nil
}
