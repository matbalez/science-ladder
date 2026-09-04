package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matbalez/science-ladder/internal/runner"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"github.com/matbalez/science-ladder/prompts"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "sl:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Print(help)
		return nil
	}
	switch args[0] {
	case "version":
		fmt.Println("Science Ladder protocol v1 · CLI 0.1.0 · MIT")
		return nil
	case "scout-prompt":
		f := flag.NewFlagSet("scout-prompt", flag.ContinueOnError)
		topic := f.String("topic", "", "scientific topic")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		fmt.Print(strings.ReplaceAll(prompts.Scout, "{{FIELD_OR_TOPIC}}", *topic))
		return nil
	case "candidate":
		if len(args) != 3 || args[1] != "lint" {
			return errors.New("usage: sl candidate lint FILE")
		}
		data, err := os.ReadFile(args[2])
		if err != nil {
			return err
		}
		candidate, err := protocol.ParseCandidate(data)
		if err != nil {
			return err
		}
		digest, _ := protocol.Digest(candidate)
		return runner.WriteJSON(os.Stdout, map[string]any{"valid": true, "digest": digest, "disposition": candidate.Disposition, "official": false})
	case "challenge":
		if len(args) < 2 {
			return errors.New("challenge requires lint, init, or test")
		}
		switch args[1] {
		case "lint":
			if len(args) != 3 {
				return errors.New("usage: sl challenge lint FILE")
			}
			manifest, err := readManifest(args[2])
			if err != nil {
				return err
			}
			digest, _ := protocol.Digest(manifest)
			return runner.WriteJSON(os.Stdout, map[string]any{"valid": true, "manifestDigest": digest, "official": false})
		case "init":
			return scaffold(args[2:])
		case "test":
			return conformance(args[2:])
		}
		return errors.New("unknown challenge command")
	case "validate":
		f := flag.NewFlagSet("validate", flag.ContinueOnError)
		local := f.Bool("local", false, "local development only")
		unsafe := f.Bool("unsafe-local", false, "acknowledge local container isolation differs from production")
		manifestPath := f.String("manifest", "science-ladder.yaml", "manifest path")
		artifactPath := f.String("artifact", "", "artifact directory")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if !*local || !*unsafe || *artifactPath == "" {
			return errors.New("use --local --unsafe-local --artifact DIRECTORY")
		}
		manifest, err := readManifest(*manifestPath)
		if err != nil {
			return err
		}
		report, err := runner.LocalValidate(context.Background(), manifest, filepath.Dir(*manifestPath), *artifactPath, *unsafe)
		_ = runner.WriteJSON(os.Stdout, report)
		return err
	case "artifact":
		if len(args) < 2 || args[1] != "digest" {
			return errors.New("usage: sl artifact digest --manifest FILE --artifact DIRECTORY")
		}
		f := flag.NewFlagSet("artifact digest", flag.ContinueOnError)
		manifestPath := f.String("manifest", "science-ladder.yaml", "manifest")
		artifactPath := f.String("artifact", "", "artifact directory")
		if err := f.Parse(args[2:]); err != nil {
			return err
		}
		m, err := readManifest(*manifestPath)
		if err != nil {
			return err
		}
		tree, digest, err := protocol.CanonicalArtifact(*artifactPath, m.Submission)
		if err != nil {
			return err
		}
		return runner.WriteJSON(os.Stdout, map[string]any{"artifactDigest": digest, "tree": tree, "official": false})
	case "milestone-simulate":
		f := flag.NewFlagSet("milestone-simulate", flag.ContinueOnError)
		manifestPath := f.String("manifest", "science-ladder.yaml", "manifest")
		score := f.String("score", "", "decimal score")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		m, err := readManifest(*manifestPath)
		if err != nil {
			return err
		}
		ticks, err := protocol.NormalizeScore(*score, m.Metric)
		if err != nil {
			return err
		}
		crossed := []string{}
		for _, milestone := range m.Milestones {
			ok, err := protocol.Qualifies(ticks, milestone.ThresholdTicks, m.Metric.Direction)
			if err != nil {
				return err
			}
			if ok {
				crossed = append(crossed, milestone.ID)
			}
		}
		return runner.WriteJSON(os.Stdout, map[string]any{"scoreTicks": ticks, "crossedMilestones": crossed, "official": false, "note": "Simulation assumes all hard gates pass; official claims require independent confirmation and acceptance order."})
	case "receipt":
		if len(args) < 2 || args[1] != "verify" {
			return errors.New("usage: sl receipt verify --receipt FILE --keys FILE")
		}
		f := flag.NewFlagSet("receipt verify", flag.ContinueOnError)
		receiptPath := f.String("receipt", "", "DSSE receipt file")
		keysPath := f.String("keys", "", "trusted key ID to PEM JSON file")
		if err := f.Parse(args[2:]); err != nil {
			return err
		}
		data, err := os.ReadFile(*receiptPath)
		if err != nil {
			return err
		}
		var envelope protocol.Envelope
		if err := protocol.DecodeStrict(data, &envelope); err != nil {
			return err
		}
		keys, err := runner.ReadPublicKeys(*keysPath)
		if err != nil {
			return err
		}
		payload, err := protocol.Verify(envelope, keys)
		if err != nil {
			return err
		}
		return runner.WriteJSON(os.Stdout, map[string]any{"signatureValid": true, "payload": json.RawMessage(payload), "note": "Verify key delegation/validity and audit checkpoint history separately before granting official authority."})
	case "auth", "submit", "status", "export":
		return remoteCommand(args)
	}
	return fmt.Errorf("unknown command %q; run sl --help", args[0])
}

func readManifest(filename string) (protocol.Manifest, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return protocol.Manifest{}, err
	}
	return protocol.ParseManifest(data)
}

func conformance(args []string) error {
	f := flag.NewFlagSet("challenge test", flag.ContinueOnError)
	manifestPath := f.String("manifest", "science-ladder.yaml", "manifest")
	unsafe := f.Bool("unsafe-local", false, "acknowledge local execution is nonofficial")
	if err := f.Parse(args); err != nil {
		return err
	}
	if !*unsafe {
		return errors.New("challenge test requires --unsafe-local")
	}
	m, err := readManifest(*manifestPath)
	if err != nil {
		return err
	}
	root := filepath.Dir(*manifestPath)
	reports := []map[string]any{}
	failed := false
	for _, fixture := range m.Fixtures {
		first, firstErr := runner.LocalValidate(context.Background(), m, root, filepath.Join(root, fixture.Path), true)
		passed := first.Outcome == fixture.ExpectedOutcome && (fixture.ExpectedTicks == "" || first.ScoreTicks == fixture.ExpectedTicks)
		if firstErr != nil && first.Outcome != "invalid_output" {
			passed = false
		}
		if passed && first.Outcome == "valid" {
			second, secondErr := runner.LocalValidate(context.Background(), m, root, filepath.Join(root, fixture.Path), true)
			if secondErr != nil || second.Outcome != first.Outcome || second.ScoreTicks != first.ScoreTicks {
				passed = false
			}
		}
		if fixture.Name == "baseline" && first.ScoreTicks != m.Metric.BaselineTicks {
			passed = false
		}
		if !passed {
			failed = true
		}
		report := map[string]any{"fixture": fixture.Name, "passed": passed, "report": first}
		if firstErr != nil {
			report["error"] = firstErr.Error()
		}
		reports = append(reports, report)
	}
	_ = runner.WriteJSON(os.Stdout, map[string]any{"kind": "LocalConformanceReport", "official": false, "passed": !failed, "fixtures": reports, "warning": "Local conformance does not certify remote build reproducibility, host isolation or scientific validity."})
	if failed {
		return errors.New("conformance fixture failed")
	}
	return nil
}

const help = `Science Ladder · payment-free scientific challenge protocol

  sl scout-prompt --topic "scientific field"
  sl candidate lint science-ladder-candidate.yaml
  sl challenge init --candidate science-ladder-candidate.yaml --out challenge
  sl challenge lint science-ladder.yaml
  sl challenge test --manifest science-ladder.yaml --unsafe-local
  sl validate --local --unsafe-local --manifest science-ladder.yaml --artifact submission
  sl artifact digest --manifest science-ladder.yaml --artifact submission
  sl milestone-simulate --manifest science-ladder.yaml --score 1.25
  sl receipt verify --receipt receipt.json --keys trusted-keys.json
  sl auth login --api https://YOUR-API
  sl submit --api URL --version ID --repository OWNER/REPO --commit FULL_SHA --license MIT
  sl status --api URL --submission ID
  sl export --api URL --version ID --out challenge-export.json

Local container runs require Docker and are always nonofficial.
SL_API_TOKEN provides a scoped API token without placing it in command history.
`
