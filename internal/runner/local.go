// Package runner keeps checker execution outside the application control plane.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

type LocalReport struct {
	APIVersion     string          `json:"apiVersion"`
	Kind           string          `json:"kind"`
	Official       bool            `json:"official"`
	Warning        string          `json:"warning"`
	Outcome        string          `json:"outcome"`
	ScoreTicks     string          `json:"scoreTicks,omitempty"`
	ArtifactDigest string          `json:"artifactDigest,omitempty"`
	Gates          map[string]bool `json:"gates,omitempty"`
	DurationMillis int64           `json:"durationMillis"`
}

type boundedBuffer struct {
	mu       sync.Mutex
	b        bytes.Buffer
	max      int
	overflow bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := b.max - b.b.Len()
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.b.Write(p)
	return n, nil
}

// LocalValidate runs a pinned container, with explicit opt-in and no network or
// secrets. It is developer feedback and cannot issue an official run receipt.
func LocalValidate(ctx context.Context, m protocol.Manifest, challengeRoot, artifactRoot string, unsafeLocal bool) (LocalReport, error) {
	report := LocalReport{APIVersion: protocol.APIVersion, Kind: "LocalValidationReport", Official: false, Warning: "UNOFFICIAL local container run; not the production Firecracker execution profile", Outcome: "infrastructure_fault"}
	if !unsafeLocal {
		return report, errors.New("local execution requires --unsafe-local; no official result will be issued")
	}
	if err := protocol.ValidateManifest(m); err != nil {
		return report, err
	}
	_, digest, err := protocol.CanonicalArtifact(artifactRoot, m.Submission)
	if err != nil {
		report.Outcome = "invalid_output"
		return report, err
	}
	report.ArtifactDigest = digest
	challengeRoot, err = filepath.Abs(challengeRoot)
	if err != nil {
		return report, err
	}
	artifactRoot, err = filepath.Abs(artifactRoot)
	if err != nil {
		return report, err
	}
	suitePath := filepath.Join(challengeRoot, filepath.FromSlash(m.Suite.Path))
	info, err := os.Stat(suitePath)
	if err != nil || !info.IsDir() {
		return report, errors.New("public suite directory unavailable")
	}
	if m.Suite.Visibility != "public" {
		return report, errors.New("local command cannot materialize a hidden official suite")
	}
	dependencyRoot, err := os.MkdirTemp("", "science-ladder-dependencies-")
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(dependencyRoot)
	if err := os.Chmod(dependencyRoot, 0755); err != nil {
		return report, err
	}
	dependencyInputs := map[string][]byte{}
	lockBytes, err := os.ReadFile(filepath.Join(challengeRoot, m.Validator.DependencyLock))
	if err != nil {
		return report, err
	}
	dependencyInputs[m.Validator.DependencyLock] = lockBytes
	wheelRoot := filepath.Join(challengeRoot, "wheels")
	if entries, err := os.ReadDir(wheelRoot); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return report, errors.New("wheel directory must contain only regular wheel files")
			}
			info, err := entry.Info()
			if err != nil || info.Size() > 32<<20 {
				return report, errors.New("invalid local wheel size")
			}
			data, err := os.ReadFile(filepath.Join(wheelRoot, entry.Name()))
			if err != nil {
				return report, err
			}
			dependencyInputs["wheels/"+entry.Name()] = data
		}
	}
	dependencies, err := lockedWheelFiles(dependencyInputs, m.Validator.DependencyLock)
	if err != nil {
		return report, err
	}
	if err := writeTree(dependencyRoot, dependencies); err != nil {
		return report, err
	}
	output, err := os.MkdirTemp("", "science-ladder-local-")
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(output)
	if err := os.Chmod(output, 0777); err != nil {
		return report, err
	}
	for _, value := range []string{challengeRoot, artifactRoot, suitePath, output, dependencyRoot} {
		if strings.ContainsAny(value, ",\n\r") {
			return report, errors.New("unsupported mount path character")
		}
	}
	containerName := fmt.Sprintf("sl-local-%d", time.Now().UnixNano())
	args := []string{"run", "--rm", "--name", containerName, "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--pids-limit=64", "--user=65534:65534", "--platform=linux/amd64", "--memory", fmt.Sprintf("%dm", m.Resources.MemoryMB), "--cpus", fmt.Sprint(m.Resources.VCPU), "--ulimit", "nofile=128:128", "--ulimit", "fsize=65536:65536", "--tmpfs", "/sl/work:rw,noexec,nosuid,nodev,size=64m", "--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=16m", "--mount", "type=bind,src=" + challengeRoot + ",dst=/sl/challenge,readonly", "--mount", "type=bind,src=" + artifactRoot + ",dst=/sl/submission,readonly", "--mount", "type=bind,src=" + suitePath + ",dst=/sl/suite,readonly", "--mount", "type=bind,src=" + output + ",dst=/sl/output", "--env", "PYTHONHASHSEED=0", "--env", "TZ=UTC", "--env", "LC_ALL=C.UTF-8", "--env", "OPENBLAS_NUM_THREADS=1", "--env", "OMP_NUM_THREADS=1", "--env", "PYTHONDONTWRITEBYTECODE=1", "--entrypoint", m.Validator.Entrypoint[0], m.Validator.RuntimeImageDigest}
	// RuntimeImageDigest is an OCI digest. Docker requires an immutable repository
	// reference; the default platform runtime repository contains that exact digest.
	runtimeRepository := os.Getenv("SL_RUNTIME_REPOSITORY")
	if runtimeRepository == "" {
		runtimeRepository = "ghcr.io/matbalez/science-ladder-python"
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]*$`).MatchString(runtimeRepository) {
		return report, errors.New("invalid local runtime repository")
	}
	args[len(args)-1] = runtimeRepository + "@" + m.Validator.RuntimeImageDigest
	// Insert the immutable dependency mount before the image/entrypoint tail.
	imageIndex := len(args) - 3
	dependencyArgs := []string{"--mount", "type=bind,src=" + dependencyRoot + ",dst=/sl/validator,readonly", "--env", "PYTHONPATH=/sl/validator/site-packages"}
	args = append(append(append([]string{}, args[:imageIndex]...), dependencyArgs...), args[imageIndex:]...)
	args = append(args, m.Validator.Entrypoint[1:]...)
	deadline, cancel := context.WithTimeout(ctx, time.Duration(m.Resources.TimeoutSeconds)*time.Second)
	defer cancel()
	command := exec.CommandContext(deadline, "docker", args...)
	logs := &boundedBuffer{max: 65536}
	command.Stdout = logs
	command.Stderr = logs
	start := time.Now()
	err = command.Run()
	report.DurationMillis = time.Since(start).Milliseconds()
	if deadline.Err() != nil {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = exec.CommandContext(cleanup, "docker", "rm", "-f", containerName).Run()
		report.Outcome = "resource_limit"
		return report, nil
	}
	if err != nil {
		report.Outcome = "challenge_fault"
		return report, fmt.Errorf("local checker exited unsuccessfully: %w", err)
	}
	info, err = os.Lstat(filepath.Join(output, "result.json"))
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		report.Outcome = "invalid_output"
		return report, errors.New("checker did not produce one bounded regular result.json")
	}
	file, err := os.Open(filepath.Join(output, "result.json"))
	if err != nil {
		return report, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 65537))
	if err != nil {
		return report, err
	}
	result, ticks, err := protocol.ValidateResult(data, m)
	if err != nil {
		report.Outcome = "invalid_output"
		return report, err
	}
	report.Outcome = "valid"
	for _, passed := range result.Gates {
		if !passed {
			report.Outcome = "hard_gate_failed"
		}
	}
	report.ScoreTicks = ticks
	report.Gates = result.Gates
	return report, nil
}

func WriteJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
