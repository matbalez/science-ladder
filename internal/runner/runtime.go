package runner

import (
	"bufio"
	"bytes"
	"context"
	"crypto"
	"crypto/ecdh"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

type PinnedFile struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}
type HostAttestation struct {
	HostID                 string    `json:"hostId"`
	PhysicalHostID         string    `json:"physicalHostId"`
	HostGroup              string    `json:"hostGroup"`
	ExpiresAt              time.Time `json:"expiresAt"`
	ExclusivePhysicalHost  bool      `json:"exclusivePhysicalHost"`
	EgressPolicyVerified   bool      `json:"egressPolicyVerified"`
	ExecutionProfileDigest string    `json:"executionProfileDigest"`
	RunnerEpoch            string    `json:"runnerEpoch"`
	ConfigDigest           string    `json:"configDigest"`
}

type Config struct {
	HostID                 string            `json:"hostId"`
	HostGroup              string            `json:"hostGroup"`
	RunnerEpoch            string            `json:"runnerEpoch"`
	ExecutionProfileDigest string            `json:"executionProfileDigest"`
	Kernel                 PinnedFile        `json:"kernel"`
	RootFS                 PinnedFile        `json:"rootFs"`
	Firecracker            PinnedFile        `json:"firecracker"`
	Jailer                 PinnedFile        `json:"jailer"`
	MakeSquashFS           PinnedFile        `json:"makeSquashFs"`
	CPUConfig              PinnedFile        `json:"cpuConfig"`
	RuntimeImageDigest     string            `json:"runtimeImageDigest"`
	EncryptionPublicKey    []byte            `json:"encryptionPublicKey"`
	WorkRoot               string            `json:"workRoot"`
	ResultSpool            string            `json:"resultSpool"`
	NetworkNamespace       string            `json:"networkNamespace"`
	CPUSet                 string            `json:"cpuSet"`
	UID                    int               `json:"uid"`
	GID                    int               `json:"gid"`
	Attestation            protocol.Envelope `json:"attestation"`
}

// ConfigBindingDigest binds the enrolled host to exact pinned runtime components,
// namespace and filesystem policy. The attestation is excluded to avoid recursion.
func ConfigBindingDigest(c Config) (string, error) {
	c.Attestation = protocol.Envelope{}
	return protocol.Digest(c)
}

type Runtime struct {
	Config       Config
	Keys         map[string]crypto.PublicKey
	Signer       crypto.Signer
	KeyID        string
	HTTPClient   *http.Client
	localObjects map[string]string
	EncryptionPrivateKey []byte
	suiteMaterial *protocol.SuiteKeyMaterial
}

func(r *Runtime)hiddenMaterial(job protocol.RunnerJob)(protocol.SuiteKeyMaterial,error){
	if r.suiteMaterial!=nil{material:=*r.suiteMaterial;material.Key=append([]byte{},material.Key...);material.Salt=append([]byte{},material.Salt...);if material.Commitment!=job.Manifest.Suite.Commitment{return protocol.SuiteKeyMaterial{},errors.New("private suite cache does not match lock")};return material,nil}
	if job.HiddenSuite==nil||job.HiddenSuite.Commitment!=job.Manifest.Suite.Commitment{return protocol.SuiteKeyMaterial{},errors.New("hidden suite grant does not match manifest")};private,err:=ecdh.X25519().NewPrivateKey(r.EncryptionPrivateKey);if err!=nil{return protocol.SuiteKeyMaterial{},errors.New("host hidden-suite decryption key unavailable")};if !bytes.Equal(private.PublicKey().Bytes(),r.Config.EncryptionPublicKey){return protocol.SuiteKeyMaterial{},errors.New("host encryption key differs from signed enrollment")};material,err:=protocol.UnwrapSuiteKey(r.EncryptionPrivateKey,job.HiddenSuite.KeyCapsule,protocol.HiddenSuiteContext(job.ID,job.HiddenSuite.Commitment));if err!=nil{return material,err};if material.Commitment!=job.HiddenSuite.Commitment{return protocol.SuiteKeyMaterial{},errors.New("decrypted suite key has wrong commitment")};return material,nil
}

func ReadPublicKeys(filename string) (map[string]crypto.PublicKey, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var values map[string]string
	if err := protocol.DecodeStrict(data, &values); err != nil {
		return nil, err
	}
	keys := map[string]crypto.PublicKey{}
	for id, value := range values {
		block, rest := pem.Decode([]byte(value))
		if block == nil || len(bytes.TrimSpace(rest)) != 0 {
			return nil, errors.New("public keys must be PEM PKIX values")
		}
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		keys[id] = key
	}
	return keys, nil
}

func ReadLocalSigner(filename string) (crypto.Signer, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("local key must have mode 0600")
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid private key PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, errors.New("key is not a signer")
	}
	return signer, nil
}

func (c Config) CheckHost(keys map[string]crypto.PublicKey) error {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || os.Geteuid() != 0 {
		return errors.New("official runtime requires root on dedicated Linux amd64")
	}
	if c.HostID == "" || c.HostGroup == "" || c.RunnerEpoch == "" || !protocol.ValidDigest(c.ExecutionProfileDigest) || c.UID < 10000 || c.GID < 10000 || c.CPUSet == "" {
		return errors.New("incomplete dedicated host profile")
	}
	if strings.ContainsAny(c.CPUSet, " \n\r;") || strings.ContainsAny(c.HostID, "/\\\n") {
		return errors.New("invalid host configuration")
	}
	payload, err := protocol.Verify(c.Attestation, keys)
	if err != nil {
		return fmt.Errorf("host attestation: %w", err)
	}
	var attestation HostAttestation
	if err := protocol.DecodeStrict(payload, &attestation); err != nil {
		return err
	}
	if attestation.HostID != c.HostID || attestation.HostGroup != c.HostGroup || attestation.PhysicalHostID == "" || !attestation.ExclusivePhysicalHost || !attestation.EgressPolicyVerified || !attestation.ExpiresAt.After(time.Now()) || attestation.ExecutionProfileDigest != c.ExecutionProfileDigest || attestation.RunnerEpoch != c.RunnerEpoch {
		return errors.New("host inventory attestation does not authorize this profile")
	}
	binding, err := ConfigBindingDigest(c)
	if err != nil || binding != attestation.ConfigDigest || !protocol.ValidDigest(c.RuntimeImageDigest) {
		return errors.New("host attestation does not bind the exact configured runtime")
	}
	if _, err := os.Stat("/dev/kvm"); err != nil {
		return errors.New("KVM device unavailable")
	}
	for _, check := range []struct{ path, want string }{{"/sys/devices/system/cpu/smt/active", "0"}, {"/sys/kernel/mm/ksm/run", "0"}} {
		data, err := os.ReadFile(check.path)
		if err != nil || strings.TrimSpace(string(data)) != check.want {
			return fmt.Errorf("required host control missing: %s", check.path)
		}
	}
	swaps, err := os.ReadFile("/proc/swaps")
	if err != nil || len(strings.Split(strings.TrimSpace(string(swaps)), "\n")) != 1 {
		return errors.New("host swap must be disabled")
	}
	if !tmpfsPath(c.WorkRoot) {
		return errors.New("runner work root must reside on tmpfs")
	}
	info, err := os.Stat(c.WorkRoot)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("private mode 0700 work root required")
	}
	if !filepath.IsAbs(c.ResultSpool) || strings.HasPrefix(c.ResultSpool, strings.TrimSuffix(c.WorkRoot, "/")+"/") || c.ResultSpool == c.WorkRoot {
		return errors.New("durable result spool must be outside ephemeral guest storage")
	}
	info, err = os.Stat(c.ResultSpool)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("private durable result spool required")
	}
	if !filepath.IsAbs(c.NetworkNamespace) {
		return errors.New("dedicated network namespace required")
	}
	if _, err := os.Stat(c.NetworkNamespace); err != nil {
		return errors.New("network namespace missing")
	}
	for _, file := range []PinnedFile{c.Kernel, c.RootFS, c.Firecracker, c.Jailer, c.MakeSquashFS, c.CPUConfig} {
		if err := verifyPinned(file); err != nil {
			return err
		}
		if err := rootOwnedHierarchy(file.Path); err != nil {
			return err
		}
	}
	for _, name := range []string{c.WorkRoot, c.ResultSpool, c.NetworkNamespace} {
		if err := rootOwnedHierarchy(name); err != nil {
			return err
		}
	}
	return nil
}

func rootOwnedHierarchy(filename string) error {
	for name := filepath.Clean(filename); ; name = filepath.Dir(name) {
		info, err := os.Lstat(name)
		if err != nil {
			return err
		}
		owner, ok := info.Sys().(*syscall.Stat_t)
		if !ok || owner.Uid != 0 || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
			return fmt.Errorf("runtime path must be root-owned and protected: %s", name)
		}
		if name == "/" {
			break
		}
	}
	return nil
}

func tmpfsPath(filename string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	best := ""
	kind := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mount := fields[1]
		if filename == mount || strings.HasPrefix(filename, strings.TrimSuffix(mount, "/")+"/") {
			if len(mount) > len(best) {
				best = mount
				kind = fields[2]
			}
		}
	}
	return kind == "tmpfs"
}

func verifyPinned(file PinnedFile) error {
	if !filepath.IsAbs(file.Path) || !protocol.ValidDigest(file.Digest) {
		return errors.New("all runtime components require absolute paths and digests")
	}
	info, err := os.Lstat(file.Path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 {
		return fmt.Errorf("unsafe runtime file %s", file.Path)
	}
	data, err := os.ReadFile(file.Path)
	if err != nil {
		return err
	}
	if protocol.DigestBytes(data) != file.Digest {
		return fmt.Errorf("runtime digest mismatch: %s", file.Path)
	}
	return nil
}

func (r *Runtime) Run(ctx context.Context, envelope protocol.Envelope) (protocol.Envelope, error) {
	if err := r.Config.CheckHost(r.Keys); err != nil {
		return protocol.Envelope{}, err
	}
	if r.Signer == nil {
		return protocol.Envelope{}, errors.New("certified host signer required")
	}
	payload, err := protocol.Verify(envelope, r.Keys)
	if err != nil {
		return protocol.Envelope{}, err
	}
	var job protocol.RunnerJob
	if err := protocol.DecodeStrict(payload, &job); err != nil {
		return protocol.Envelope{}, err
	}
	if err := ValidateJob(job, r.Config); err != nil {
		return protocol.Envelope{}, err
	}
	return r.runJob(ctx, job)
}

func (r *Runtime) runJob(ctx context.Context, job protocol.RunnerJob) (protocol.Envelope, error) {
	// Atomic directory creation provides per-host single-job tenancy, even across
	// daemon processes. A stale directory requires operator recovery, never reuse.
	lease := filepath.Join(r.Config.WorkRoot, "active")
	if err := os.Mkdir(lease, 0700); err != nil {
		return protocol.Envelope{}, errors.New("host is busy or requires stale-run recovery")
	}
	defer os.RemoveAll(lease)
	root := filepath.Join(lease, filepath.Base(r.Config.Firecracker.Path), job.ID, "root")
	if err := os.MkdirAll(root, 0700); err != nil {
		return protocol.Envelope{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(job.Manifest.Resources.TimeoutSeconds+60)*time.Second)
	defer cancel()
	for _, item := range []struct {
		name string
		ref  protocol.ObjectRef
	}{{"validator.squashfs", job.ValidatorDisk}, {"submission.squashfs", job.SubmissionDisk}, {"suite.squashfs", job.SuiteDisk}, {"challenge.squashfs", job.ChallengeDisk}} {
		if err := r.fetch(ctx, item.ref, filepath.Join(root, item.name)); err != nil {
			return protocol.Envelope{}, err
		}
	}
	if job.Manifest.Suite.Visibility=="hidden"{material,err:=r.hiddenMaterial(job);if err!=nil{return protocol.Envelope{},err};defer protocol.ZeroBytes(material.Key);defer protocol.ZeroBytes(material.Salt);filename:=filepath.Join(root,"suite.squashfs");ciphertext,err:=os.ReadFile(filename);if err!=nil{return protocol.Envelope{},err};plaintext,err:=protocol.DecryptSuiteObject(material,ciphertext,"disk");if err!=nil{return protocol.Envelope{},err};defer protocol.ZeroBytes(plaintext);if err:=os.WriteFile(filename,plaintext,0400);err!=nil{return protocol.Envelope{},err}}
	for _, item := range []struct {
		name string
		file PinnedFile
	}{{"vmlinux", r.Config.Kernel}, {"rootfs.ext4", r.Config.RootFS}} {
		if err := copyFile(item.file.Path, filepath.Join(root, item.name), 0400); err != nil {
			return protocol.Envelope{}, err
		}
	}
	configDir := filepath.Join(lease, "job-config")
	if err := os.Mkdir(configDir, 0700); err != nil {
		return protocol.Envelope{}, err
	}
	manifestJSON, err := json.Marshal(job.Manifest)
	if err != nil {
		return protocol.Envelope{}, err
	}
	if err := os.WriteFile(filepath.Join(configDir, "manifest.json"), manifestJSON, 0400); err != nil {
		return protocol.Envelope{}, err
	}
	build := exec.CommandContext(ctx, r.Config.MakeSquashFS.Path, configDir, filepath.Join(root, "config.squashfs"), "-noappend", "-all-root", "-no-xattrs", "-no-exports", "-processors", "1", "-mkfs-time", "0", "-all-time", "0")
	logs := &boundedBuffer{max: 65536}
	build.Stdout = logs
	build.Stderr = logs
	if err := build.Run(); err != nil {
		return protocol.Envelope{}, errors.New("trusted job configuration disk failed")
	}
	drives := []map[string]any{{"drive_id": "rootfs", "path_on_host": "/rootfs.ext4", "is_root_device": true, "is_read_only": true}}
	for _, name := range []string{"validator", "submission", "suite", "challenge", "config"} {
		drives = append(drives, map[string]any{"drive_id": name, "path_on_host": "/" + name + ".squashfs", "is_root_device": false, "is_read_only": true})
	}
	configuration := map[string]any{"boot-source": map[string]any{"kernel_image_path": "/vmlinux", "boot_args": "console=ttyS0 reboot=k panic=1 pci=off root=/dev/vda ro init=/sbin/sl-init"}, "drives": drives, "machine-config": map[string]any{"vcpu_count": job.Manifest.Resources.VCPU, "mem_size_mib": job.Manifest.Resources.MemoryMB, "smt": false}, "network-interfaces": []any{}}
	cpuBytes, err := os.ReadFile(r.Config.CPUConfig.Path)
	if err != nil {
		return protocol.Envelope{}, err
	}
	var cpuConfig map[string]any
	if err := protocol.DecodeStrict(cpuBytes, &cpuConfig); err != nil {
		return protocol.Envelope{}, err
	}
	configuration["cpu-config"] = cpuConfig
	encoded, err := json.Marshal(configuration)
	if err != nil {
		return protocol.Envelope{}, err
	}
	if err := os.WriteFile(filepath.Join(root, "firecracker.json"), encoded, 0400); err != nil {
		return protocol.Envelope{}, err
	}
	if err := filepath.Walk(root, func(name string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return os.Chown(name, r.Config.UID, r.Config.GID)
	}); err != nil {
		return protocol.Envelope{}, err
	}
	args := []string{"--id", job.ID, "--exec-file", r.Config.Firecracker.Path, "--uid", fmt.Sprint(r.Config.UID), "--gid", fmt.Sprint(r.Config.GID), "--chroot-base-dir", lease, "--netns", r.Config.NetworkNamespace, "--cgroup-version", "2", "--cgroup", "cpuset.cpus=" + r.Config.CPUSet, "--cgroup", "memory.max=" + fmt.Sprint(int64(job.Manifest.Resources.MemoryMB+256)*1024*1024), "--cgroup", "pids.max=64", "--", "--no-api", "--config-file", "/firecracker.json"}
	command := exec.CommandContext(ctx, r.Config.Jailer.Path, args...)
	output := &boundedBuffer{max: 128 * 1024}
	command.Stdout = output
	command.Stderr = output
	start := time.Now()
	runErr := command.Run()
	duration := time.Since(start).Milliseconds()
	jobDigest, _ := protocol.Digest(job)
	receipt := protocol.RunReceipt{AcceptanceReceiptDigest: job.AcceptanceReceiptDigest, DeploymentMode: job.DeploymentMode, OfficialAcceptance: job.OfficialAcceptance, APIVersion: protocol.APIVersion, Kind: "ValidationRunReceipt", ID: job.ID + "-run", CreatedAt: time.Now().UTC(), Producer: r.Config.HostID, JobID: job.ID, JobDigest: jobDigest, ChallengeLockDigest: job.ChallengeLockDigest, ArtifactDigest: job.ArtifactDigest, SuiteDigest: job.SuiteDigest, ExecutionProfileDigest: job.ExecutionProfileDigest, RunnerEpoch: job.RunnerEpoch, FencingToken: job.FencingToken, HostID: r.Config.HostID, HostGroup: r.Config.HostGroup, Official: true, Outcome: "infrastructure_fault", DurationMillis: duration}
	if ctx.Err() != nil {
		receipt.Outcome = "resource_limit"
	} else if runErr == nil && !output.overflow {
		resultBytes, guestOutcome, err := parseGuestOutput(output.b.Bytes())
		if err == nil && guestOutcome != "" {
			receipt.Outcome = guestOutcome
		} else if err == nil {
			result, ticks, err := protocol.ValidateResult(resultBytes, job.Manifest)
			if err == nil {
				receipt.Outcome = "valid"
				receipt.ScoreTicks = ticks
				receipt.Gates = result.Gates
				for _, pass := range result.Gates {
					if !pass {
						receipt.Outcome = "hard_gate_failed"
					}
				}
			} else {
				receipt.Outcome = "invalid_output"
			}
		} else {
			receipt.Outcome = "invalid_output"
		}
	}
	if err := os.RemoveAll(lease); err != nil {
		return protocol.Envelope{}, errors.New("ephemeral teardown failed; host must be quarantined")
	}
	receipt.CleanupAttested = true
	return protocol.Sign(r.KeyID, r.Signer, receipt)
}

func ValidateJob(job protocol.RunnerJob, c Config) error {
	if job.APIVersion != protocol.APIVersion || job.Kind != "ValidationJob" || len(job.ID) > 80 || job.ID == "" || strings.ContainsAny(job.ID, "./\\\n\r ") || !job.ExpiresAt.After(time.Now()) || job.CreatedAt.After(time.Now().Add(time.Minute)) || job.ExpiresAt.Sub(job.CreatedAt) > time.Hour {
		return errors.New("invalid or expired job")
	}
	if job.Purpose != "submission" && job.Purpose != "confirmation" {
		return errors.New("this runtime only executes prebuilt competitive disks; quarantine preparation is separate")
	}
	if job.ExecutionProfileDigest != c.ExecutionProfileDigest || job.RunnerEpoch != c.RunnerEpoch || job.RequiredHostGroup != "" && job.RequiredHostGroup != c.HostGroup || job.FencingToken < 1 {
		return errors.New("job profile/epoch/host/fencing mismatch")
	}
	if job.Manifest.Validator.RuntimeImageDigest != c.RuntimeImageDigest {
		return errors.New("job runtime image does not match enrolled rootfs profile")
	}
	for _, host := range job.ExcludedHostIDs {
		if host == c.HostID {
			return errors.New("confirmation anti-affinity violation")
		}
	}
	for _, digest := range []string{job.ChallengeLockDigest, job.ArtifactDigest, job.SuiteDigest, job.ExecutionProfileDigest, job.AcceptanceReceiptDigest} {
		if !protocol.ValidDigest(digest) {
			return errors.New("job has invalid immutable digest")
		}
	}
	return protocol.ValidateManifest(job.Manifest)
}

func (r *Runtime) fetch(ctx context.Context, ref protocol.ObjectRef, destination string) error {
	if filename, ok := r.localObjects[ref.Digest]; ok {
		data, err := os.ReadFile(filename)
		if err != nil || int64(len(data)) != ref.Size || protocol.DigestBytes(data) != ref.Digest {
			return errors.New("cached object binding mismatch")
		}
		return os.WriteFile(destination, data, 0400)
	}
	if !protocol.ValidDigest(ref.Digest) || ref.Size < 1 || ref.Size > 1<<30 {
		return errors.New("invalid exact-object grant")
	}
	u, err := url.Parse(ref.URL)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return errors.New("object grants require HTTPS")
	}
	client := r.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("object redirect forbidden") }}
	}
	request, err := http.NewRequestWithContext(ctx, "GET", ref.URL, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("object read rejected")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, ref.Size+1))
	if err != nil || int64(len(data)) != ref.Size || protocol.DigestBytes(data) != ref.Digest {
		return errors.New("object size/digest mismatch")
	}
	return os.WriteFile(destination, data, 0400)
}

func copyFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func parseFrame(output []byte) ([]byte, error) {
	data, _, err := parseGuestOutput(output)
	return data, err
}
func parseGuestOutput(output []byte) ([]byte, string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 4096), 100000)
	var result []byte
	outcome := ""
	frames := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "SL_RESULT ") {
			frames++
			data, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(line, "SL_RESULT "))
			if err != nil || len(data) > 65536 {
				return nil, "", errors.New("invalid guest frame")
			}
			result = data
		}
		if strings.HasPrefix(line, "SL_ERROR ") {
			frames++
			outcome = strings.TrimPrefix(line, "SL_ERROR ")
			switch outcome {
			case "resource_limit", "challenge_fault", "invalid_output":
			default:
				return nil, "", errors.New("unknown guest error category")
			}
		}
	}
	if scanner.Err() != nil || frames != 1 {
		return nil, "", errors.New("missing, multiple or oversized guest frames")
	}
	return result, outcome, nil
}
