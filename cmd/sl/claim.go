package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/matbalez/science-ladder/internal/runner"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

type claimOutput struct {
	data     []byte
	overflow bool
}

func (b *claimOutput) Write(p []byte) (int, error) {
	n := len(p)
	remain := 65536 - len(b.data)
	if n > remain {
		b.overflow = true
		b.data = append(b.data, p[:remain]...)
	} else {
		b.data = append(b.data, p...)
	}
	return n, nil
}
func boundedClaimFile(path string) ([]byte, error) {
	info, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > 65536 {
		return nil, errors.New("claim/report must be a regular file of at most 64 KiB")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 65537))
	if len(b) > 65536 {
		return nil, errors.New("claim/report exceeds 64 KiB")
	}
	return b, e
}
func claimCommand(args []string) error {
	f := flag.NewFlagSet("claim", flag.ContinueOnError)
	api := f.String("api", os.Getenv("SL_API_URL"), "API origin")
	version := f.String("version", "", "challenge version ID")
	manifest := f.String("manifest", "science-ladder.yaml", "frozen manifest")
	artifact := f.String("artifact", "", "artifact-only directory")
	report := f.String("report", "", "new result file written by checker; default is checker stdout")
	out := f.String("out", "", "new claim file outside artifact directory")
	estimate := f.String("estimated-score-ticks", "", "frontier estimate for server-only measurement")
	reason := f.String("measurement-reason", "", "why local qualification predicts improvement on server-only measurements")
	if e := f.Parse(args); e != nil {
		return e
	}
	if *version == "" || *artifact == "" || *out == "" || len(f.Args()) == 0 {
		return errors.New("usage: sl claim --api URL --version ID --artifact DIR --out claim.json [--report NEW-FILE] -- CHECKER ARGUMENTS")
	}
	artifactRoot, e := filepath.EvalSymlinks(*artifact)
	if e != nil {
		return e
	}
	artifactRoot, e = filepath.Abs(artifactRoot)
	if e != nil {
		return e
	}
	for _, p := range []string{*out, *report} {
		if p != "" {
			parent, e := filepath.EvalSymlinks(filepath.Dir(p))
			if e != nil {
				return e
			}
			parent, e = filepath.Abs(parent)
			if e != nil {
				return e
			}
			relative, e := filepath.Rel(artifactRoot, parent)
			if e != nil {
				return e
			}
			if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return errors.New("claim and report files must be outside the artifact directory")
			}
			if _, e := os.Lstat(p); !os.IsNotExist(e) {
				return fmt.Errorf("use a new output path: %s", p)
			}
		}
	}
	m, e := readManifest(*manifest)
	if e != nil {
		return e
	}
	client, e := newClient(*api)
	if e != nil {
		return e
	}
	var policy struct {
		Manifest      protocol.Manifest `json:"manifest"`
		LockDigest    string            `json:"lockDigest"`
		FrontierTicks string            `json:"frontierTicks"`
		Mode          string            `json:"mode"`
	}
	if e = client.request("GET", "/v1/challenge-versions/"+url.PathEscape(*version)+"/admission", nil, &policy); e != nil {
		return e
	}
	a, _ := protocol.Digest(m)
	b, _ := protocol.Digest(policy.Manifest)
	if a != b {
		return errors.New("local manifest differs from the published version; use the pinned challenge source")
	}
	_, before, e := protocol.CanonicalArtifact(*artifact, m.Submission)
	if e != nil {
		return e
	}
	// The user's explicit command runs locally; neither the API nor this command chooses executable code.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, f.Args()[0], f.Args()[1:]...)
	var output claimOutput
	command.Stdout = &output
	command.Stderr = os.Stderr
	fmt.Fprintln(os.Stderr, "Running final local check against the candidate. This is not platform verification.")
	if e = command.Run(); e != nil {
		return fmt.Errorf("local checker failed: %w", e)
	}
	_, after, e := protocol.CanonicalArtifact(*artifact, m.Submission)
	if e != nil {
		return e
	}
	if before != after {
		return errors.New("artifact changed during local validation; rerun with a fixed candidate")
	}
	data := output.data
	if *report != "" {
		data, e = boundedClaimFile(*report)
		if e != nil {
			return e
		}
	} else if output.overflow {
		return errors.New("checker stdout exceeds 64 KiB; use --report for its result file")
	}
	c := protocol.FrontierClaim{APIVersion: protocol.FrontierClaimVersion, VersionID: *version, LockDigest: policy.LockDigest, ArtifactDigest: before, Mode: policy.Mode}
	if policy.Mode == "qualification" {
		if len(data) == 0 {
			return errors.New("local qualification must produce a report")
		}
		c.Qualification = &protocol.LocalQualification{ChecksPassed: true, ReportDigest: protocol.DigestBytes(data), EstimatedScoreTicks: *estimate, Reason: *reason}
	} else {
		var kind struct {
			Kind string `json:"kind"`
		}
		if e = json.Unmarshal(data, &kind); e != nil {
			return e
		}
		if kind.Kind == "LocalReport" || kind.Kind == "LocalValidationReport" {
			var local runner.LocalReport
			if e = json.Unmarshal(data, &local); e != nil {
				return e
			}
			if local.Outcome != "valid" || local.ArtifactDigest != before || local.Result == nil {
				return errors.New("local report must be valid, match the artifact and include its validator result; update the CLI")
			}
			c.Result = local.Result
		} else {
			var result protocol.ValidatorResult
			if e = protocol.DecodeStrict(data, &result); e != nil {
				return e
			}
			c.Result = &result
		}
	}
	ticks, e := c.Validate(m)
	if e != nil {
		return e
	}
	if !protocol.FrontierImprovement(ticks, policy.FrontierTicks, m.Metric) {
		return errors.New("local result does not improve the public frontier by the required delta; continue searching locally")
	}
	file, e := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if e != nil {
		return e
	}
	defer file.Close()
	if e = runner.WriteJSON(file, c); e != nil {
		return e
	}
	fmt.Fprintln(os.Stderr, "Unverified frontier claim saved. Submit the same artifact with --claim; the server will recheck admission.")
	return nil
}
