package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"time"

	"github.com/matbalez/science-ladder/internal/runner"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

func runtimeInventory(args []string) error {
	f := flag.NewFlagSet("runtime-inventory", flag.ContinueOnError)
	image := f.String("image", "", "approved digest-pinned Python runtime image")
	output := f.String("out", "", "new inventory JSON file")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *output == "" {
		return errors.New("--out is required")
	}
	inventory, err := runner.InventoryRuntime(context.Background(), *image)
	if err != nil {
		return err
	}
	return writeNewJSON(*output, inventory)
}

func dependencyInventory(args []string) error {
	f := flag.NewFlagSet("dependency-inventory", flag.ContinueOnError)
	basePath := f.String("runtime-inventory", "", "approved runtime inventory JSON")
	snapshotPath := f.String("snapshot", "", "source snapshot JSON")
	output := f.String("out", "", "new complete dependency inventory JSON")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *output == "" {
		return errors.New("--out is required")
	}
	data, err := os.ReadFile(*basePath)
	if err != nil {
		return err
	}
	var base runner.RuntimeInventory
	if err := protocol.DecodeStrict(data, &base); err != nil {
		return err
	}
	data, err = os.ReadFile(*snapshotPath)
	if err != nil {
		return err
	}
	snapshot, err := runner.ReadSourceSnapshot(data)
	if err != nil {
		return err
	}
	manifest, err := protocol.ParseManifest(snapshot.Files["science-ladder.yaml"])
	if err != nil {
		return err
	}
	if manifest.Validator.RuntimeImageDigest != base.RuntimeImageDigest {
		return errors.New("manifest/runtime inventory image mismatch")
	}
	base.Packages, err = runner.DependencyInventory(snapshot.Files, manifest.Validator.DependencyLock, base)
	if err != nil {
		return err
	}
	return writeNewJSON(*output, base)
}

func writeNewJSON(filename string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data, err = protocol.CanonicalJSON(data)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

// advisoryCheck evaluates an unsigned draft for operator review. Production
// Builder.Scan additionally requires the pinned platform signature and job pins.
func advisoryCheck(args []string) error {
	f := flag.NewFlagSet("advisory-check", flag.ContinueOnError)
	inventoryPath := f.String("runtime-inventory", "", "complete runtime/dependency inventory JSON")
	snapshotPath := f.String("snapshot", "", "unsigned advisory snapshot draft")
	output := f.String("out", "", "new review result JSON")
	if err := f.Parse(args); err != nil {
		return err
	}
	data, err := os.ReadFile(*inventoryPath)
	if err != nil {
		return err
	}
	var inventory runner.RuntimeInventory
	if err := protocol.DecodeStrict(data, &inventory); err != nil {
		return err
	}
	data, err = os.ReadFile(*snapshotPath)
	if err != nil {
		return err
	}
	var snapshot runner.AdvisorySnapshot
	if err := protocol.DecodeStrict(data, &snapshot); err != nil {
		return err
	}
	findings, status := runner.ScanAdvisories(inventory.Packages, snapshot, time.Now().UTC())
	result := map[string]any{"signatureVerified": false, "officialAcceptance": false, "status": status, "findings": findings, "packagesChecked": len(inventory.Packages)}
	if *output != "" {
		return writeNewJSON(*output, result)
	}
	return runner.WriteJSON(os.Stdout, result)
}
