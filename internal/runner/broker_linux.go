//go:build linux

package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

const candidateUID = 65533
const checkerUID = 65534

// candidateBroker runs only as root inside a disposable Firecracker guest.
// Candidate source never runs with the checker UID or sees checker/suite mounts.
// The outer VMM still isolates both domains from the physical host and app.
type candidateBroker struct {
	assets                    []protocol.EvaluationAsset
	program                   protocol.CandidateProgram
	baseline                  *candidateBroker
	measurement               *protocol.MeasurementPolicy
	pairs                     []protocol.TimingTrial
	pending, pairOutputsValid bool
	root                      string
	listener                  *net.UnixListener
	built                     bool
	ready                     bool
	runs                      int
	fault                     error
	done                      chan struct{}
}

func startCandidateBroker(ctx context.Context, m protocol.Manifest) (*candidateBroker, error) {
	if os.Getpid() != 1 || os.Geteuid() != 0 || m.Evaluation == nil || m.Evaluation.Program == nil {
		return nil, errors.New("candidate broker requires guest PID 1 and a frozen program contract")
	}
	b := &candidateBroker{assets: m.Evaluation.Assets, program: *m.Evaluation.Program, root: "/sl/candidate", done: make(chan struct{})}
	if err := b.prepareRootFrom(m.Submission, "/sl/submission"); err != nil {
		return nil, err
	}
	if err := b.prepareBaseline(m); err != nil {
		return nil, err
	}
	if err := os.MkdirAll("/sl/broker", 0711); err != nil {
		return nil, err
	}
	if err := syscall.Mount("tmpfs", "/sl/broker", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, "size=1m,mode=0711"); err != nil {
		return nil, err
	}
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: "/sl/broker/control.sock", Net: "unix"})
	if err != nil {
		return nil, err
	}
	if err = os.Chown("/sl/broker/control.sock", checkerUID, checkerUID); err != nil {
		l.Close()
		return nil, err
	}
	if err = os.Chmod("/sl/broker/control.sock", 0600); err != nil {
		l.Close()
		return nil, err
	}
	b.listener = l
	go func() { <-ctx.Done(); _ = l.Close() }()
	go b.serve(ctx)
	return b, nil
}

func bindReadOnly(source, target string) error {
	if err := syscall.Mount(source, target, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return err
	}
	return syscall.Mount("", target, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_NOSUID|syscall.MS_NODEV, "")
}

func (b *candidateBroker) prepareRootFrom(contract protocol.SubmissionContract, source string) error {
	if err := os.MkdirAll(b.root, 0700); err != nil {
		return err
	}
	if err := syscall.Mount("tmpfs", b.root, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "size="+strconv.Itoa(b.program.ScratchMB)+"m,mode=0755"); err != nil {
		return err
	}
	for _, dir := range []string{"usr", "work", "tmp", "etc", "dev", "assets"} {
		if err := os.Mkdir(filepath.Join(b.root, dir), 0755); err != nil {
			return err
		}
	}
	for _, asset := range b.assets {
		if asset.Visibility != "public" {
			continue
		}
		target := filepath.Join(b.root, "assets", asset.Name)
		if err := os.Mkdir(target, 0755); err != nil {
			return err
		}
		if err := bindReadOnly(filepath.Join("/sl/assets", asset.Name), target); err != nil {
			return err
		}
	}
	// No /proc, /sys, /sl, broker socket, answer keys or checker descriptors.
	if err := bindReadOnly("/opt/sl-private/toolchain/usr", filepath.Join(b.root, "usr")); err != nil {
		return err
	}
	for _, link := range []string{"bin", "lib", "lib64", "sbin"} {
		if _, err := os.Stat("/usr/" + link); err == nil {
			if err = os.Symlink("usr/"+link, filepath.Join(b.root, link)); err != nil {
				return err
			}
		}
	}
	// Debian tool names such as cc resolve through this immutable symlink
	// directory. It contains runtime alternatives, never host configuration.
	alternatives := filepath.Join(b.root, "etc/alternatives")
	if err := os.Mkdir(alternatives, 0755); err != nil {
		return err
	}
	if err := bindReadOnly("/etc/alternatives", alternatives); err != nil {
		return err
	}
	for _, device := range []string{"null", "zero", "urandom"} {
		target := filepath.Join(b.root, "dev", device)
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		f.Close()
		if err = syscall.Mount("/dev/"+device, target, "", syscall.MS_BIND, ""); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(b.root, "etc/passwd"), []byte("candidate:x:65533:65533:Candidate:/tmp:/bin/false\n"), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(b.root, "etc/group"), []byte("candidate:x:65533:\n"), 0644); err != nil {
		return err
	}
	if err := os.Chown(filepath.Join(b.root, "tmp"), candidateUID, candidateUID); err != nil {
		return err
	}
	work := filepath.Join(b.root, "work")
	// A separate bind permits sealing the entire compiled work tree read-only.
	if err := syscall.Mount(work, work, "", syscall.MS_BIND, ""); err != nil {
		return err
	}
	var total int64
	files := 0
	if err := filepath.WalkDir(source, func(name string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if err := protocol.ValidatePath(filepath.ToSlash(relative)); err != nil {
			return err
		}
		target := filepath.Join(work, relative)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("candidate source contains a link or special file")
		}
		files++
		total += info.Size()
		if files > contract.MaxFiles || total > contract.MaxBytes {
			return errors.New("candidate source exceeds immutable contract")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return copyFile(name, target, 0644)
	}); err != nil {
		return err
	}
	return filepath.Walk(work, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(name, candidateUID, candidateUID)
	})
}

func (b *candidateBroker) serve(ctx context.Context) {
	defer close(b.done)
	for {
		conn, err := b.listener.AcceptUnix()
		if err != nil {
			if ctx.Err() == nil {
				b.fault = err
			}
			return
		}
		func() {
			defer conn.Close()
			stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
			defer stopClose()
			if deadline, ok := ctx.Deadline(); ok {
				_ = conn.SetDeadline(deadline)
			}
			if !checkerPeer(conn) {
				return
			}
			data, err := io.ReadAll(io.LimitReader(conn, (maxBrokerInput*4/3)+4097))
			var request BrokerRequest
			if err != nil || protocol.DecodeStrictBounded(data, &request, (maxBrokerInput*4/3)+4096) != nil {
				return
			}
			if b.measurement != nil && request.Action != "build" {
				response := b.timingRequest(ctx, request)
				_ = json.NewEncoder(conn).Encode(response)
				return
			}
			response := BrokerResponse{Outcome: "invalid_request", ExitCode: -1}
			if err := validateBrokerRequest(request, b.built, b.runs, b.program); err == nil && (request.Action == "build" || b.ready) {
				argv, budget := b.program.Run, b.program.RunBudget
				if request.Action == "build" {
					b.built = true
					argv, budget = b.program.Build, b.program.BuildBudget
				} else {
					b.runs++
				}
				response, err = b.execute(ctx, argv, budget, request.Input)
				if err != nil {
					b.fault = err
					response = BrokerResponse{Outcome: "infrastructure_fault", ExitCode: -1}
				}
				if request.Action == "build" && response.Outcome == "valid" && b.baseline != nil {
					base := b.baseline
					var e error
					baseResult, e := base.execute(ctx, base.program.Build, base.program.BuildBudget, nil)
					if e != nil {
						b.fault = e
						response.Outcome = "infrastructure_fault"
					} else if baseResult.Outcome != "valid" {
						b.fault = errors.New("frozen baseline failed to build")
						response.Outcome = "infrastructure_fault"
					} else {
						if e := base.sealWork(); e != nil {
							b.fault = e
							response.Outcome = "infrastructure_fault"
						} else {
							base.ready = true
						}
					}
				}
				if request.Action == "build" && response.Outcome == "valid" {
					if err := b.sealWork(); err != nil {
						b.fault = err
						response.Outcome = "infrastructure_fault"
					} else {
						b.ready = true
					}
				}
			}
			_ = json.NewEncoder(conn).Encode(response)
		}()
		if b.fault != nil {
			return
		}
	}
}

func checkerPeer(c *net.UnixConn) bool {
	raw, err := c.SyscallConn()
	if err != nil {
		return false
	}
	allowed := false
	err = raw.Control(func(fd uintptr) {
		cred, err := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		allowed = err == nil && cred.Uid == checkerUID
	})
	return err == nil && allowed
}

func (b *candidateBroker) execute(parent context.Context, argv []string, budget protocol.StageBudget, input []byte) (BrokerResponse, error) {
	result := BrokerResponse{Outcome: "candidate_error", ExitCode: -1}
	cg := fmt.Sprintf("/sys/fs/cgroup/candidate-%d", time.Now().UnixNano())
	if err := os.Mkdir(cg, 0700); err != nil {
		return result, err
	}
	defer os.Remove(cg)
	for name, value := range map[string]string{"pids.max": strconv.Itoa(budget.MaxProcesses), "memory.max": strconv.FormatInt(int64(budget.MemoryMB)<<20, 10), "memory.swap.max": "0"} {
		if err := os.WriteFile(filepath.Join(cg, name), []byte(value), 0600); err != nil {
			return result, err
		}
	}
	group, err := os.Open(cg)
	if err != nil {
		return result, err
	}
	defer group.Close()
	ctx, cancel := context.WithTimeout(parent, time.Duration(budget.TimeoutSeconds)*time.Second)
	defer cancel()
	args := append([]string{strconv.FormatInt(budget.MaxOutputBytes, 10)}, argv...)
	command := exec.CommandContext(ctx, "/usr/local/bin/sl-candidate-sandbox", args...)
	command.Dir = "/work"
	command.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/tmp", "TMPDIR=/tmp", "TZ=UTC", "LC_ALL=C.UTF-8", "PYTHONHASHSEED=0", "PYTHONDONTWRITEBYTECODE=1", "SOURCE_DATE_EPOCH=0", "CARGO_HOME=/tmp/cargo", "OPENBLAS_NUM_THREADS=1", "OMP_NUM_THREADS=1"}
	command.SysProcAttr = &syscall.SysProcAttr{Chroot: b.root, Credential: &syscall.Credential{Uid: candidateUID, Gid: candidateUID}, Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC | syscall.CLONE_NEWUTS, UseCgroupFD: true, CgroupFD: int(group.Fd()), Pdeathsig: syscall.SIGKILL}
	command.Cancel = func() error { return killGuestValidatorCgroup(cg) }
	command.WaitDelay = 2 * time.Second
	stdout, stderr := &boundedBuffer{max: int(budget.MaxOutputBytes)}, &boundedBuffer{max: 65536}
	command.Stdin = bytes.NewReader(input)
	command.Stdout = stdout
	command.Stderr = stderr
	start := time.Now()
	runErr := command.Run()
	if err := killGuestValidatorCgroup(cg); err != nil {
		return result, err
	}
	result.DurationNanos = time.Since(start).Nanoseconds()
	limited, err := guestResourceLimitExceeded(cg, runErr)
	if err != nil {
		return result, err
	}
	result.Stdout = append([]byte(nil), stdout.b.Bytes()...)
	result.Stderr = append([]byte(nil), stderr.b.Bytes()...)
	if command.ProcessState != nil {
		result.ExitCode = command.ProcessState.ExitCode()
	}
	switch {
	case ctx.Err() != nil || limited:
		result.Outcome = "resource_limit"
	case stdout.overflow || stderr.overflow:
		result.Outcome = "output_limit"
	case runErr == nil:
		result.Outcome = "valid"
	}
	// All writers are gone before removing scratch. Build artifacts live under
	// /work; /tmp never survives between independent cases.
	tmp := filepath.Join(b.root, "tmp")
	if err := os.RemoveAll(tmp); err != nil {
		return result, err
	}
	if err := os.Mkdir(tmp, 0700); err != nil {
		return result, err
	}
	if err := os.Chown(tmp, candidateUID, candidateUID); err != nil {
		return result, err
	}
	return result, nil
}

func (b *candidateBroker) sealWork() error {
	return syscall.Mount("", filepath.Join(b.root, "work"), "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_NOSUID|syscall.MS_NODEV, "")
}
