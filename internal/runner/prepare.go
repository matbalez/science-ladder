package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func (r *Runtime) Prepare(ctx context.Context, envelope protocol.Envelope, upload UploadObject) (protocol.Envelope, error) {
	if err := r.Config.CheckHost(r.Keys); err != nil {
		return protocol.Envelope{}, err
	}
	if r.Signer == nil {
		return protocol.Envelope{}, errors.New("certified quarantine signer required")
	}
	payload, err := protocol.Verify(envelope, r.Keys)
	if err != nil {
		return protocol.Envelope{}, err
	}
	var job protocol.RunnerJob
	if err := protocol.DecodeStrict(payload, &job); err != nil {
		return protocol.Envelope{}, err
	}
	if job.APIVersion != protocol.APIVersion || job.Kind != "ValidationJob" || job.Purpose != "preflight" && job.Purpose != "artifact_prepare" || job.SourceSnapshot == nil || !job.ExpiresAt.After(time.Now()) || job.FencingToken < 1 || job.ExecutionProfileDigest != r.Config.ExecutionProfileDigest || job.RunnerEpoch != r.Config.RunnerEpoch {
		return protocol.Envelope{}, errors.New("invalid quarantine job binding")
	}
	if job.RequiredHostGroup != "" && job.RequiredHostGroup != r.Config.HostGroup {
		return protocol.Envelope{}, errors.New("quarantine host group mismatch")
	}
	if !protocol.ValidVerificationPolicy(protocol.JobVerificationPolicy(job)) {
		return protocol.Envelope{}, errors.New("quarantine job must bind verification policy")
	}
	if err := protocol.ValidateManifest(job.Manifest); err != nil {
		return protocol.Envelope{}, err
	}
	if job.Manifest.Validator.RuntimeImageDigest != r.Config.RuntimeImageDigest {
		return protocol.Envelope{}, errors.New("quarantine runtime image differs from job")
	}
	if job.Purpose == "preflight" && (job.AdvisorySnapshotDigest != r.Config.AdvisorySnapshot.Digest || job.RuntimeInventoryDigest != r.Config.RuntimeInventory.Digest || !protocol.ValidDigest(job.AdvisorySnapshotDigest) || !protocol.ValidDigest(job.RuntimeInventoryDigest)) {
		return protocol.Envelope{}, errors.New("quarantine advisory policy differs from signed job")
	}
	ctx, cancel := context.WithDeadline(ctx, job.ExpiresAt)
	defer cancel()
	workspace, err := os.MkdirTemp(r.Config.WorkRoot, "quarantine-")
	if err != nil {
		return protocol.Envelope{}, err
	}
	defer os.RemoveAll(workspace)
	sourcePath := filepath.Join(workspace, "source.json")
	if job.SourceSnapshot.Size > 96<<20 {
		return protocol.Envelope{}, errors.New("source object exceeds quarantine limit")
	}
	if err := r.fetch(ctx, *job.SourceSnapshot, sourcePath); err != nil {
		return protocol.Envelope{}, err
	}
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return protocol.Envelope{}, err
	}
	builder := Builder{Runtime: r, MakeSquashFS: r.Config.MakeSquashFS, Upload: upload}
	start := time.Now()
	var report protocol.BuildReport
	if job.Purpose == "preflight" {
		snapshot, parseErr := ReadSourceSnapshot(sourceData)
		if parseErr != nil {
			return protocol.Envelope{}, parseErr
		}
		report, err = builder.Preflight(ctx, job, snapshot, workspace)
	} else {
		report, err = builder.PrepareArtifact(ctx, job, sourceData, workspace)
	}
	if err != nil {
		report.Passed = false
		report.Findings = append(report.Findings, err.Error())
	}
	jobDigest, _ := protocol.Digest(job)
	receipt := protocol.RunReceipt{APIVersion: protocol.APIVersion, Kind: "ValidationRunReceipt", ID: job.ID + "-build", CreatedAt: time.Now().UTC(), Producer: r.Config.HostID, JobID: job.ID, JobDigest: jobDigest, ChallengeLockDigest: job.ChallengeLockDigest, ArtifactDigest: job.ArtifactDigest, SuiteDigest: job.SuiteDigest, ExecutionProfileDigest: job.ExecutionProfileDigest, RunnerEpoch: job.RunnerEpoch, FencingToken: job.FencingToken, HostID: r.Config.HostID, HostGroup: r.Config.HostGroup, Official: true, OfficialAcceptance: job.OfficialAcceptance, DeploymentMode: job.DeploymentMode, Outcome: "valid", DurationMillis: time.Since(start).Milliseconds(), BuildReport: &report}
	if !report.Passed {
		receipt.Outcome = "invalid_output"
		// Failed preparation may have produced local files before a later check
		// failed. Only successful, uploaded objects are granted CAS references.
		report.ValidatorDisk = nil
		report.ChallengeDisk = nil
		report.SuiteDisk = nil
		report.SubmissionDisk = nil
		report.SBOM = nil
	}
	if err := os.RemoveAll(workspace); err != nil {
		return protocol.Envelope{}, errors.New("quarantine teardown failed; host requires intervention")
	}
	receipt.CleanupAttested = true
	receipt.VerificationPolicy = job.VerificationPolicy
	return protocol.Sign(r.KeyID, r.Signer, receipt)
}
