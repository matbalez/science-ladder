package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func scaffold(args []string) error {
	f := flag.NewFlagSet("challenge init", flag.ContinueOnError)
	candidatePath := f.String("candidate", "", "adopted candidate")
	out := f.String("out", "", "new challenge directory")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *candidatePath == "" || *out == "" {
		return errors.New("--candidate and --out required")
	}
	data, err := os.ReadFile(*candidatePath)
	if err != nil {
		return err
	}
	candidate, err := protocol.ParseCandidate(data)
	if err != nil {
		return err
	}
	if candidate.Manifest == nil || candidate.Disposition != "viable" {
		return errors.New("scaffolding requires an adopted viable candidate with complete manifest")
	}
	if err := os.Mkdir(*out, 0755); err != nil {
		return fmt.Errorf("destination must not exist: %w", err)
	}
	m := candidate.Manifest
	m.VerificationPolicy = protocol.ManifestVerificationPolicy(*m)
	manifest, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	files := map[string][]byte{"science-ladder.yaml": append(manifest, '\n'), "science-ladder-candidate.yaml": data, "requirements.lock": []byte("# Add pinned packages with --hash=sha256:...; an empty file uses only the platform standard library.\n"), "README.md": []byte("# " + m.Title + "\n\nDRAFT scaffold imported from an adopted Science Ladder candidate.\n\nReplace the deliberately failing checker and supply real baseline, valid, invalid, malformed, numeric-boundary and resource fixtures. Inspect source evidence and data licenses. Do not publish until the local suite and isolated remote preflight actually pass.\n\nRun `sl challenge lint science-ladder.yaml` and `sl challenge test --manifest science-ladder.yaml --unsafe-local`.\n\nLocal reports are nonofficial. Publication locks the exact contract, source, validator, suite, execution profile and reviews.\n"), ".gitignore": []byte(".env*\n!.env.example\n__pycache__/\n*.pyc\n.sl-work/\n")}
	checkerPath := m.Validator.Entrypoint[1][len("/sl/challenge/"):]
	files[checkerPath] = []byte("\"\"\"Replace with a deterministic scientific validator; this draft always fails.\"\"\"\nimport json\nfrom pathlib import Path\nmanifest = json.loads(Path('/sl/config/manifest.json').read_text()) if Path('/sl/config/manifest.json').exists() else json.loads(Path('/sl/challenge/science-ladder.yaml').read_text())\nresult = {'apiVersion': 'science-ladder/v1', 'kind': 'ValidatorResult', 'score': '0', 'gates': {gate: False for gate in manifest['hardGates']}}\nPath('/sl/output/result.json').write_text(json.dumps(result))\n")
	for relative, contents := range files {
		filename := filepath.Join(*out, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filename, contents, 0644); err != nil {
			return err
		}
	}
	for _, fixture := range m.Fixtures {
		if err := os.MkdirAll(filepath.Join(*out, fixture.Path), 0755); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(*out, m.Suite.Path), 0755); err != nil {
		return err
	}
	fmt.Printf("Created draft in %s. Checker intentionally fails until you implement and test the scientific contract.\n", *out)
	return nil
}
