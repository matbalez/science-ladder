package runner

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

// SourceSnapshot is the exact bounded JSON envelope independently fetched by the
// control plane from GitHub. []byte values encode as standard base64 in JSON.
type SourceSnapshot struct {
	RepositoryID int64             `json:"repositoryId"`
	SourceCommit string            `json:"sourceCommit"`
	Files        map[string][]byte `json:"files"`
}
type ArtifactSnapshot struct {
	Tree  protocol.ArtifactTree `json:"tree"`
	Files map[string][]byte     `json:"files"`
}

type UploadObject func(context.Context, string, string, protocol.ObjectRef) (protocol.ObjectRef, error)

type Builder struct {
	Runtime      *Runtime
	MakeSquashFS PinnedFile
	UnsafeLocal  bool
	Upload       UploadObject
}

func ReadSourceSnapshot(data []byte) (SourceSnapshot, error) {
	var snapshot SourceSnapshot
	if err := protocol.DecodeStrictBounded(data, &snapshot, 96<<20); err != nil {
		return snapshot, err
	}
	if snapshot.RepositoryID < 1 || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(snapshot.SourceCommit) {
		return snapshot, errors.New("exact independently fetched GitHub source identity required")
	}
	if err := validateSourceFiles(snapshot.Files); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func validateSourceFiles(files map[string][]byte) error {
	if len(files) == 0 || len(files) > 4096 {
		return errors.New("source file count exceeds bounds")
	}
	seen := map[string]bool{}
	var total int64
	secretPattern := regexp.MustCompile(`(?m)(-----BEGIN (?:EC |RSA |OPENSSH )?PRIVATE KEY-----|gh[pousr]_[A-Za-z0-9]{30,}|sk-proj-[A-Za-z0-9_-]{20,})`)
	for name, data := range files {
		if err := protocol.ValidatePath(name); err != nil {
			return err
		}
		fold := strings.ToLower(name)
		if seen[fold] {
			return errors.New("source paths collide")
		}
		seen[fold] = true
		total += int64(len(data))
		if total > 64<<20 {
			return errors.New("source exceeds 64 MiB")
		}
		base := path.Base(name)
		if strings.HasPrefix(base, ".env") && base != ".env.example" {
			return errors.New("private environment files forbidden")
		}
		if base == "Dockerfile" || strings.HasSuffix(base, ".sh") {
			return errors.New("creator Dockerfiles and shell build scripts are outside this profile")
		}
		if secretPattern.Match(data) {
			return errors.New("credential/private-key indicator detected in source")
		}
		if bytes.HasPrefix(data, []byte("version https://git-lfs.github.com/spec/v1")) {
			return errors.New("unresolved LFS pointer")
		}
	}
	return nil
}

func writeTree(directory string, files map[string][]byte) error {
	names := make([]string, 0, len(files))
	for name := range files {
		if err := protocol.ValidatePath(name); err != nil {
			return err
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		filename := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filename, files[name], 0644); err != nil {
			return err
		}
		if err := os.Chtimes(filename, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
			return err
		}
	}
	return nil
}

func subset(files map[string][]byte, prefix string) map[string][]byte {
	out := map[string][]byte{}
	prefix = strings.TrimSuffix(prefix, "/") + "/"
	for name, data := range files {
		if strings.HasPrefix(name, prefix) {
			out[strings.TrimPrefix(name, prefix)] = data
		}
	}
	return out
}

func (b *Builder) disk(ctx context.Context, root, destination string) (protocol.ObjectRef, error) {
	if err := verifyPinned(b.MakeSquashFS); err != nil {
		return protocol.ObjectRef{}, err
	}
	args := []string{root, destination, "-noappend", "-all-root", "-no-xattrs", "-no-exports", "-processors", "1", "-mkfs-time", "0", "-all-time", "0", "-comp", "gzip", "-b", "131072", "-no-progress"}
	cmd := exec.CommandContext(ctx, b.MakeSquashFS.Path, args...)
	output := &boundedBuffer{max: 65536}
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		return protocol.ObjectRef{}, fmt.Errorf("deterministic filesystem construction failed: %w", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		return protocol.ObjectRef{}, err
	}
	return protocol.ObjectRef{Digest: protocol.DigestBytes(data), Size: int64(len(data))}, nil
}

// lockedWheelFiles supports an offline platform-owned build recipe, never a
// creator command. The initial dependency profile accepts hash-locked pure Python
// wheels only; native wheels require a separately reviewed execution profile.
func lockedWheelFiles(files map[string][]byte, lockPath string) (map[string][]byte, error) {
	lock, ok := files[lockPath]
	if !ok {
		return nil, errors.New("dependency lock missing")
	}
	if len(lock) > 128*1024 {
		return nil, errors.New("dependency lock too large")
	}
	text := strings.ReplaceAll(string(lock), "\\\n", " ")
	linePattern := regexp.MustCompile(`^([A-Za-z0-9_.-]+)==([A-Za-z0-9_.+-]+)\s+--hash=sha256:([a-f0-9]{64})$`)
	pins := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		match := linePattern.FindStringSubmatch(line)
		if match == nil {
			return nil, errors.New("lock requires exact package==version and one SHA256 hash; no URLs or build options")
		}
		name := strings.ToLower(strings.ReplaceAll(match[1], "-", "_")) + "-" + match[2]
		if _, exists := pins[name]; exists {
			return nil, errors.New("duplicate dependency pin")
		}
		pins[name] = match[3]
	}
	out := map[string][]byte{}
	used := map[string]bool{}
	var total int64
	for name, data := range files {
		if !strings.HasPrefix(name, "wheels/") {
			continue
		}
		if !strings.HasSuffix(name, "-py3-none-any.whl") {
			return nil, errors.New("initial build profile supports pure Python py3-none-any wheels only")
		}
		base := path.Base(name)
		parts := strings.Split(base, "-")
		if len(parts) != 5 {
			return nil, errors.New("unsupported wheel filename")
		}
		pin := strings.ToLower(parts[0]) + "-" + parts[1]
		digest, ok := pins[pin]
		if !ok || protocol.DigestBytes(data) != "sha256:"+digest || used[pin] {
			return nil, errors.New("wheel is unpinned, duplicated or hash-mismatched")
		}
		used[pin] = true
		reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		if len(reader.File) > 4096 {
			return nil, errors.New("wheel entry count exceeds limit")
		}
		for _, file := range reader.File {
			if file.FileInfo().IsDir() {
				continue
			}
			if file.Mode()&os.ModeSymlink != 0 || !file.Mode().IsRegular() {
				return nil, errors.New("wheel links/special files forbidden")
			}
			relative := file.Name
			if err := protocol.ValidatePath(relative); err != nil {
				return nil, err
			}
			if strings.Contains(relative, ".data/") {
				return nil, errors.New("wheel data installers are outside the initial profile")
			}
			if strings.HasSuffix(relative, ".pth") || strings.HasSuffix(relative, ".so") {
				return nil, errors.New("wheel startup hooks/native code require a new platform profile")
			}
			if file.UncompressedSize64 > 32<<20 || file.UncompressedSize64 > 100*max(file.CompressedSize64, 1) {
				return nil, errors.New("wheel expansion limit exceeded")
			}
			total += int64(file.UncompressedSize64)
			if total > 128<<20 {
				return nil, errors.New("dependency tree exceeds limit")
			}
			entry, err := file.Open()
			if err != nil {
				return nil, err
			}
			contents, err := io.ReadAll(io.LimitReader(entry, int64(file.UncompressedSize64)+1))
			_ = entry.Close()
			if err != nil || uint64(len(contents)) != file.UncompressedSize64 {
				return nil, errors.New("invalid wheel member length")
			}
			target := "site-packages/" + relative
			if _, exists := out[target]; exists {
				return nil, errors.New("dependency paths collide")
			}
			out[target] = contents
		}
	}
	if len(used) != len(pins) {
		return nil, errors.New("all pinned wheels must be vendored for offline build")
	}
	return out, nil
}

// Preflight constructs immutable disks twice and executes the declared fixtures.
// The official path requires a certified dedicated host; local mode is explicitly
// nonofficial and its reports cannot satisfy the server's production trust checks.
func (b *Builder) Preflight(ctx context.Context, job protocol.RunnerJob, snapshot SourceSnapshot, outputRoot string) (protocol.BuildReport, error) {
	if job.SourceSnapshot == nil {
		return protocol.BuildReport{}, errors.New("source snapshot grant required")
	}
	m := job.Manifest
	report := protocol.BuildReport{SourceSnapshotDigest: job.SourceSnapshot.Digest, ValidatorImageDigest: m.Validator.RuntimeImageDigest, ExecutionProfileDigest: job.ExecutionProfileDigest, Fixtures: []protocol.FixtureReport{}, Findings: []string{}}
	manifestDigest, err := protocol.Digest(m)
	if err != nil {
		return report, err
	}
	report.ManifestDigest = manifestDigest
	if err := protocol.ValidateManifest(m); err != nil {
		return report, err
	}
	if err := validateSourceFiles(snapshot.Files); err != nil {
		return report, err
	}
	sourceManifest, err := protocol.ParseManifest(snapshot.Files["science-ladder.yaml"])
	if err != nil {
		return report, fmt.Errorf("source manifest: %w", err)
	}
	sourceManifestDigest, _ := protocol.Digest(sourceManifest)
	if sourceManifestDigest != manifestDigest {
		return report, errors.New("source manifest does not match signed job")
	}
	if !b.UnsafeLocal {
		if b.Runtime == nil {
			return report, errors.New("certified quarantine runtime required")
		}
		if err := b.Runtime.Config.CheckHost(b.Runtime.Keys); err != nil {
			return report, err
		}
	} else {
		report.Findings = append(report.Findings, "UNOFFICIAL local build and fixture execution; not a production conformance receipt")
	}
	if m.Suite.Visibility != "public" {
		return report, errors.New("hidden-suite preflight requires an encrypted suite input grant; do not substitute public source bytes")
	}
	dependencies, err := lockedWheelFiles(snapshot.Files, m.Validator.DependencyLock)
	if err != nil {
		return report, err
	}
	validatorFiles := map[string][]byte{}
	for name, data := range snapshot.Files {
		validatorFiles[name] = data
	}
	for name, data := range dependencies {
		if _, exists := validatorFiles[name]; exists {
			return report, errors.New("source collides with locked dependency tree")
		}
		validatorFiles[name] = data
	}
	report.ScansPassed = true
	if err := os.MkdirAll(outputRoot, 0700); err != nil {
		return report, err
	}
	workspace, err := os.MkdirTemp(outputRoot, "preflight-")
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(workspace)
	for pass := 0; pass < 2; pass++ {
		root := filepath.Join(workspace, fmt.Sprintf("validator-%d", pass))
		if err := os.Mkdir(root, 0755); err != nil {
			return report, err
		}
		if err := writeTree(root, validatorFiles); err != nil {
			return report, err
		}
		destination := filepath.Join(outputRoot, fmt.Sprintf("validator-%d.squashfs", pass))
		ref, err := b.disk(ctx, root, destination)
		if err != nil {
			return report, err
		}
		if pass == 0 {
			report.ValidatorDisk = &ref
			report.ValidatorDiskDigest = ref.Digest
		} else {
			report.RebuiltValidatorDiskDigest = ref.Digest
		}
	}
	report.OfflineRebuild = report.ValidatorDiskDigest == report.RebuiltValidatorDiskDigest
	if !report.OfflineRebuild {
		return report, errors.New("offline validator rebuild digest mismatch")
	}
	sourceRoot := filepath.Join(workspace, "source")
	if err := os.Mkdir(sourceRoot, 0755); err != nil {
		return report, err
	}
	if err := writeTree(sourceRoot, snapshot.Files); err != nil {
		return report, err
	}
	challengeDisk := *report.ValidatorDisk
	if err := copyFile(filepath.Join(outputRoot, "validator-0.squashfs"), filepath.Join(outputRoot, "challenge.squashfs"), 0400); err != nil {
		return report, err
	}
	report.ChallengeDisk = &challengeDisk
	suiteFiles := subset(snapshot.Files, m.Suite.Path)
	if len(suiteFiles) == 0 {
		return report, errors.New("declared public suite contains no files")
	}
	suiteRoot := filepath.Join(workspace, "suite")
	if err := os.Mkdir(suiteRoot, 0755); err != nil {
		return report, err
	}
	if err := writeTree(suiteRoot, suiteFiles); err != nil {
		return report, err
	}
	suiteDisk, err := b.disk(ctx, suiteRoot, filepath.Join(outputRoot, "suite.squashfs"))
	if err != nil {
		return report, err
	}
	report.SuiteDisk = &suiteDisk
	report.SuiteDigest = suiteDisk.Digest
	localObjects := map[string]string{report.ValidatorDisk.Digest: filepath.Join(outputRoot, "validator-0.squashfs"), challengeDisk.Digest: filepath.Join(outputRoot, "challenge.squashfs"), suiteDisk.Digest: filepath.Join(outputRoot, "suite.squashfs")}
	allPassed := true
	for index, fixture := range m.Fixtures {
		files := subset(snapshot.Files, fixture.Path)
		_, artifactDigest, artifactErr := protocol.ArtifactFromFiles(files, m.Submission)
		fixtureReport := protocol.FixtureReport{Name: fixture.Name, ExpectedOutcome: fixture.ExpectedOutcome}
		if artifactErr != nil {
			fixtureReport.Outcome = "invalid_output"
			fixtureReport.Passed = fixture.ExpectedOutcome == "invalid_output"
			report.Fixtures = append(report.Fixtures, fixtureReport)
			allPassed = allPassed && fixtureReport.Passed
			continue
		}
		artifactRoot := filepath.Join(workspace, fmt.Sprintf("artifact-%d", index))
		if err := os.Mkdir(artifactRoot, 0755); err != nil {
			return report, err
		}
		if err := writeTree(artifactRoot, files); err != nil {
			return report, err
		}
		diskPath := filepath.Join(workspace, fmt.Sprintf("submission-%d.squashfs", index))
		artifactDisk, err := b.disk(ctx, artifactRoot, diskPath)
		if err != nil {
			return report, err
		}
		localObjects[artifactDisk.Digest] = diskPath
		var runs []LocalReport
		for repeat := 0; repeat < 2; repeat++ {
			if b.UnsafeLocal {
				local, runErr := LocalValidate(ctx, m, sourceRoot, artifactRoot, true)
				if runErr != nil && local.Outcome == "infrastructure_fault" {
					return report, runErr
				}
				runs = append(runs, local)
			} else {
				child := job
				child.ID = fmt.Sprintf("preflight-%d-%d-%d", time.Now().UnixNano(), index, repeat)
				child.Purpose = "preflight"
				child.ArtifactDigest = artifactDigest
				child.ValidatorDisk = *report.ValidatorDisk
				child.SubmissionDisk = artifactDisk
				child.SuiteDisk = suiteDisk
				child.ChallengeDisk = challengeDisk
				child.SuiteDigest = suiteDisk.Digest
				runRuntime := *b.Runtime
				runRuntime.localObjects = localObjects
				envelope, err := runRuntime.runJob(ctx, child)
				if err != nil {
					return report, err
				}
				keys := map[string]crypto.PublicKey{runRuntime.KeyID: runRuntime.Signer.Public()}
				payload, err := protocol.Verify(envelope, keys)
				if err != nil {
					return report, err
				}
				var run protocol.RunReceipt
				if err := protocol.DecodeStrict(payload, &run); err != nil {
					return report, err
				}
				runs = append(runs, LocalReport{Outcome: run.Outcome, ScoreTicks: run.ScoreTicks, Gates: run.Gates})
			}
		}
		fixtureReport.Outcome = runs[0].Outcome
		fixtureReport.ScoreTicks = runs[0].ScoreTicks
		fixtureReport.Passed = runs[0].Outcome == fixture.ExpectedOutcome && runs[1].Outcome == runs[0].Outcome && runs[1].ScoreTicks == runs[0].ScoreTicks && (fixture.ExpectedTicks == "" || runs[0].ScoreTicks == fixture.ExpectedTicks)
		if fixture.Name == "baseline" && runs[0].ScoreTicks != m.Metric.BaselineTicks {
			fixtureReport.Passed = false
		}
		allPassed = allPassed && fixtureReport.Passed
		report.Fixtures = append(report.Fixtures, fixtureReport)
	}
	report.HostileCorpusPassed = hostileParserCorpus(m)
	if report.HostileCorpusPassed {
		report.HostileCorpusPassed, err = b.isolationCorpus(ctx, job, outputRoot)
		if err != nil {
			report.Findings = append(report.Findings, err.Error())
		}
	}
	report.Passed = allPassed && report.ScansPassed && report.OfflineRebuild && report.HostileCorpusPassed
	if b.Upload != nil && report.Passed {
		for _, item := range []struct {
			role, path string
			ref        **protocol.ObjectRef
		}{{"validatorDisk", filepath.Join(outputRoot, "validator-0.squashfs"), &report.ValidatorDisk}, {"challengeDisk", filepath.Join(outputRoot, "challenge.squashfs"), &report.ChallengeDisk}, {"suiteDisk", filepath.Join(outputRoot, "suite.squashfs"), &report.SuiteDisk}} {
			uploaded, err := b.Upload(ctx, item.role, item.path, **item.ref)
			if err != nil {
				return report, err
			}
			*item.ref = &uploaded
		}
	}
	return report, nil
}

func (b *Builder) PrepareArtifact(ctx context.Context, job protocol.RunnerJob, data []byte, outputRoot string) (protocol.BuildReport, error) {
	if job.SourceSnapshot==nil{return protocol.BuildReport{},errors.New("artifact source grant required")}
	manifestDigest,err:=protocol.Digest(job.Manifest);if err!=nil{return protocol.BuildReport{},err}
	report := protocol.BuildReport{ManifestDigest:manifestDigest,SourceSnapshotDigest:job.SourceSnapshot.Digest,ExecutionProfileDigest: job.ExecutionProfileDigest, Findings: []string{}}
	if !b.UnsafeLocal {
		if b.Runtime == nil {
			return report, errors.New("certified quarantine runtime required")
		}
		if err := b.Runtime.Config.CheckHost(b.Runtime.Keys); err != nil {
			return report, err
		}
	}
	var snapshot ArtifactSnapshot
	if err := protocol.DecodeStrictBounded(data, &snapshot, 96<<20); err != nil {
		return report, err
	}
	tree, digest, err := protocol.ArtifactFromFiles(snapshot.Files, job.Manifest.Submission)
	if err != nil {
		return report, err
	}
	expectedTree, _ := json.Marshal(snapshot.Tree)
	actualTree, _ := json.Marshal(tree)
	if !bytes.Equal(expectedTree, actualTree) || digest != job.ArtifactDigest {
		return report, errors.New("canonical artifact binding mismatch")
	}
	root, err := os.MkdirTemp(outputRoot, "artifact-")
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(root)
	if err := writeTree(root, snapshot.Files); err != nil {
		return report, err
	}
	destination := filepath.Join(outputRoot, "submission.squashfs")
	ref, err := b.disk(ctx, root, destination)
	if err != nil {
		return report, err
	}
	if b.Upload != nil {
		ref, err = b.Upload(ctx, "submissionDisk", destination, ref)
		if err != nil {
			return report, err
		}
	}
	report.SubmissionDisk = &ref
	report.ScansPassed = true
	report.Passed = true
	return report, nil
}

func hostileParserCorpus(m protocol.Manifest) bool {
	for _, name := range []string{"../escape.json", "/absolute.json", "data/../escape.json", "data/e\u0301.json", "data/link\\escape.json"} {
		if protocol.ValidatePath(name) == nil {
			return false
		}
	}
	for _, score := range []string{"NaN", "Infinity", "-0", "01", "1e9999", strings.Repeat("9", 101)} {
		if _, err := protocol.NormalizeScore(score, m.Metric); err == nil {
			return false
		}
	}
	for _, input := range []string{`{"x":1,"x":2}`, `{"x":1e9999}`, `{} {}`, strings.Repeat("[", 40) + "0" + strings.Repeat("]", 40)} {
		if _, err := protocol.CanonicalJSON([]byte(input)); err == nil {
			return false
		}
	}
	return true
}
