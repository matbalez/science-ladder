package main

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"

	"github.com/matbalez/science-ladder/internal/runner"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

func suiteCommand(args []string) error {
	if len(args) == 0 || args[0] != "upload" {
		return errors.New("usage: sl suite upload --api URL --files PRIVATE_DIRECTORY --license LICENSE --provenance TEXT")
	}
	f := flag.NewFlagSet("suite upload", flag.ContinueOnError)
	api := f.String("api", os.Getenv("SL_API_URL"), "API origin")
	root := f.String("files", "", "private inert suite directory")
	license := f.String("license", "", "suite data license")
	provenance := f.String("provenance", "", "source provenance and rights")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if *root == "" || *license == "" || *provenance == "" {
		return errors.New("suite files, license and provenance are required")
	}
	client, err := newClient(*api)
	if err != nil {
		return err
	}
	if client.token == "" {
		return errors.New("authenticate before uploading private suite data")
	}
	document, err := privateSuiteDocument(*root, *license)
	if err != nil {
		return err
	}
	defer protocol.ZeroBytes(document)
	var result map[string]any
	if err := client.request("POST", "/v1/suites", map[string]any{"document": string(document), "license": *license, "provenance": *provenance}, &result); err != nil {
		return err
	}
	return runner.WriteJSON(os.Stdout, result)
}

func privateSuiteDocument(root, license string) ([]byte, error) {
	contract := protocol.SubmissionContract{AllowedPaths: []string{"*"}, AllowedExtensions: []string{".json", ".csv", ".tsv", ".txt", ".dat", ".npy", ".bin"}, MaxBytes: 1 << 20, MaxFiles: 256, License: license}
	tree, _, err := protocol.CanonicalArtifact(root, contract)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	defer func() {
		for _, data := range files {
			protocol.ZeroBytes(data)
		}
	}()
	for _, entry := range tree.Entries {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.Path)))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) != entry.Size || protocol.DigestBytes(data) != entry.Digest {
			protocol.ZeroBytes(data)
			return nil, errors.New("suite file changed while preparing upload")
		}
		files[entry.Path] = data
	}
	return json.Marshal(map[string]any{"files": files})
}
