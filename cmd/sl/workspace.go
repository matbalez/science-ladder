package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const defaultAPI = "https://scienceladder.org"
const workspaceFile = ".sl/workspace.json"

type LocalRecipe struct {
	Version      int        `json:"version"`
	Setup        [][]string `json:"setup"`
	Baseline     []string   `json:"baseline"`
	Check        []string   `json:"check"`
	CheckedGates []string   `json:"checkedGates,omitempty"`
}
type Workspace struct {
	API            string `json:"api"`
	Slug           string `json:"slug"`
	VersionID      string `json:"versionId"`
	Repository     string `json:"repository"`
	SourceCommit   string `json:"sourceCommit"`
	ManifestDigest string `json:"manifestDigest"`
}

var repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var commitPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

func workspaceCommand(command string, args []string) error {
	if command == "clone" {
		return cloneWorkspace(args)
	}
	root, w, err := findWorkspace()
	if err != nil {
		return err
	}
	if command == "doctor" {
		return doctorWorkspace(root, w)
	}
	if err = checkWorkspace(root, w); err != nil {
		return err
	}
	recipe, err := workspaceRecipe(root, w)
	if err != nil {
		return err
	}
	switch command {
	case "setup":
		if len(args) > 0 {
			return errors.New("usage: sl setup")
		}
		for _, argv := range recipe.Setup {
			if err = executeLocal(root, argv); err != nil {
				return err
			}
		}
		fmt.Println("Setup complete. Run sl run --baseline before changing the candidate.")
		return nil
	case "run":
		f := flag.NewFlagSet("run", flag.ContinueOnError)
		baseline := f.Bool("baseline", false, "reproduce the reference")
		if err = f.Parse(args); err != nil {
			return err
		}
		if len(f.Args()) > 0 {
			return errors.New("usage: sl run [--baseline]")
		}
		dir, err := newRunDirectory(root)
		if err != nil {
			return err
		}
		report := filepath.Join(dir, "result.json")
		argv := recipe.Check
		if *baseline {
			argv = recipe.Baseline
		}
		err = executeLocal(root, expandRecipe(argv, root, report))
		fmt.Println("Local run directory:", dir)
		if err == nil {
			if _, e := os.Stat(report); e != nil {
				return errors.New("checker did not write {report}; inspect the recipe")
			}
		}
		return err
	case "submit":
		return submitWorkspace(root, w, recipe, args)
	}
	return errors.New("unknown workspace command")
}
func cloneWorkspace(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: sl clone SLUG [--api URL] [--version ID] [--out DIRECTORY]")
	}
	slug := args[0]
	f := flag.NewFlagSet("clone", flag.ContinueOnError)
	api := f.String("api", defaultAPI, "platform origin")
	version := f.String("version", "", "require this published version")
	out := f.String("out", slug, "new workspace directory")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if len(f.Args()) > 0 || !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`).MatchString(slug) {
		return errors.New("use a challenge slug and named flags")
	}
	client, err := newClient(*api)
	if err != nil {
		return err
	}
	var c struct {
		VersionID    string            `json:"versionId"`
		Repository   string            `json:"repository"`
		SourceCommit string            `json:"sourceCommit"`
		Manifest     protocol.Manifest `json:"manifest"`
	}
	if err = client.request("GET", "/v1/challenges/"+url.PathEscape(slug), nil, &c); err != nil {
		return err
	}
	if *version != "" && *version != c.VersionID {
		return errors.New("published version changed; inspect the challenge before cloning")
	}
	if c.VersionID == "" || !repositoryPattern.MatchString(c.Repository) || !commitPattern.MatchString(c.SourceCommit) {
		return errors.New("platform returned incomplete source identity")
	}
	root, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	if err = os.Mkdir(root, 0755); err != nil {
		return fmt.Errorf("destination must not exist: %w", err)
	}
	source := filepath.Join(root, "challenge")
	if err = commandAt(root, "git", "clone", "--no-checkout", "https://github.com/"+c.Repository+".git", source); err != nil {
		return err
	}
	if err = commandAt(source, "git", "-c", "core.hooksPath=/dev/null", "checkout", "--detach", c.SourceCommit); err != nil {
		return err
	}
	m, err := readManifest(filepath.Join(source, "science-ladder.yaml"))
	if err != nil {
		return err
	}
	digest, err := protocol.Digest(m)
	if err != nil {
		return err
	}
	expected, err := protocol.Digest(c.Manifest)
	if err != nil {
		return err
	}
	if digest != expected {
		return errors.New("checkout manifest differs from published contract")
	}
	baseline := ""
	for _, fixture := range m.Fixtures {
		if fixture.Name == "baseline" {
			baseline = fixture.Path
		}
	}
	if baseline == "" {
		return errors.New("manifest has no baseline fixture")
	}
	if err = copyBaseline(filepath.Join(source, filepath.FromSlash(baseline)), filepath.Join(root, "artifact")); err != nil {
		return err
	}
	w := Workspace{strings.TrimRight(*api, "/"), slug, c.VersionID, c.Repository, c.SourceCommit, digest}
	if err = os.MkdirAll(filepath.Join(root, ".sl"), 0700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(w, "", "  ")
	if err = os.WriteFile(filepath.Join(root, workspaceFile), append(data, '\n'), 0600); err != nil {
		return err
	}
	fmt.Printf("Created %s\ncd %q\nRead challenge/README.md and the scientific brief, then run sl doctor, sl setup, and sl run --baseline. Edit artifact/ only. Clone has not executed challenge code.\n", root, root)
	return nil
}
func copyBaseline(source, destination string) error {
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("baseline must not contain symlinks")
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > 64<<20 {
			return errors.New("unsupported or oversized baseline file")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}
func findWorkspace() (string, Workspace, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", Workspace{}, err
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, workspaceFile))
		if err == nil {
			var w Workspace
			err = protocol.DecodeStrict(data, &w)
			return dir, w, err
		}
		if !os.IsNotExist(err) {
			return "", Workspace{}, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", Workspace{}, errors.New("no Science Ladder workspace; run sl clone CHALLENGE, then enter its directory")
}
func gitOutput(dir string, args ...string) (string, error) {
	c := exec.Command("git", args...)
	c.Dir = dir
	b, err := c.Output()
	return strings.TrimSpace(string(b)), err
}
func checkWorkspace(root string, w Workspace) error {
	source := filepath.Join(root, "challenge")
	head, err := gitOutput(source, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != w.SourceCommit {
		return errors.New("challenge checkout changed; restore the pinned source")
	}
	dirty, err := gitOutput(source, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if dirty != "" {
		return errors.New("challenge source has tracked edits; keep candidate changes in artifact/")
	}
	m, err := readManifest(filepath.Join(source, "science-ladder.yaml"))
	if err != nil {
		return err
	}
	digest, err := protocol.Digest(m)
	if err != nil {
		return err
	}
	if digest != w.ManifestDigest {
		return errors.New("manifest changed from the cloned version")
	}
	return nil
}
func workspaceRecipe(root string, w Workspace) (LocalRecipe, error) {
	// Only a recipe tracked in the pinned source may select executable commands.
	c := exec.Command("git", "show", w.SourceCommit+":science-ladder-local.json")
	c.Dir = filepath.Join(root, "challenge")
	data, err := c.Output()
	if err == nil {
		if len(data) > 65536 {
			return LocalRecipe{}, errors.New("recipe exceeds 64 KiB")
		}
		var r LocalRecipe
		if err = protocol.DecodeStrict(data, &r); err != nil {
			return r, err
		}
		return r, validateRecipe(r)
	}
	// Compatibility adapters preserve the existing immutable scientific versions.
	var r LocalRecipe
	switch w.Repository + "@" + w.SourceCommit {
	case "matbalez/science-ladder-smallest-triangle@b34fc3b226798d67286e5af1d2cd72201510dd57":
		r = programRecipe([]string{"unit-square", "all-triples", "exact-arithmetic"})
	case "matbalez/science-ladder-one-less-multiply@fa8ba6a6fa7ef0c5161198de0e79ef78ecce38cf":
		r = programRecipe([]string{"exact-tensor-identity", "ordered-bilinear-products", "bounded-certificate"})
	case "matbalez/science-ladder-quiet-echoes@f42f527e97563b1c068a1835732c6da44f21223f":
		r = LocalRecipe{Version: 1, Setup: [][]string{{"python3", "tools/reproduce.py", "--check"}, {"python3", "-m", "unittest", "discover", "-s", "tests", "-v"}}, Baseline: []string{"python3", "checker.py", "--submission", "fixtures/baseline", "--suite", "suite", "--output", "{report}"}, Check: []string{"python3", "checker.py", "--submission", "{artifact}", "--suite", "suite", "--output", "{report}"}}
	default:
		return r, errors.New("this source has no science-ladder-local.json recipe; use its documented native commands and explicit sl claim/submit flags")
	}
	return r, validateRecipe(r)
}
func programRecipe(gates []string) LocalRecipe {
	return LocalRecipe{Version: 1, Setup: [][]string{{"python3", "tools/reproduce.py"}, {"python3", "-m", "unittest", "discover", "-s", "validator", "-v"}}, Baseline: []string{"python3", "local.py", "--output", "{report}"}, Check: []string{"python3", "local.py", "--solver", "{artifact}/solver.py", "--output", "{report}"}, CheckedGates: gates}
}
func validateRecipe(r LocalRecipe) error {
	if r.Version != 1 || len(r.Check) == 0 || len(r.Baseline) == 0 || len(r.Setup) > 32 {
		return errors.New("recipe requires version 1, baseline and check commands, and at most 32 setup steps")
	}
	for _, argv := range append(append([][]string{}, r.Setup...), r.Baseline, r.Check) {
		if len(argv) == 0 || len(argv) > 128 || argv[0] == "" {
			return errors.New("commands must be nonempty argv arrays")
		}
		for _, arg := range argv {
			if strings.ContainsRune(arg, 0) || len(arg) > 8192 {
				return errors.New("invalid command argument")
			}
		}
	}
	if !strings.Contains(strings.Join(r.Check, " "), "{artifact}") {
		return errors.New("check must reference {artifact}")
	}
	for _, argv := range [][]string{r.Check, r.Baseline} {
		if !strings.Contains(strings.Join(argv, " "), "{report}") {
			return errors.New("baseline and check must write {report}")
		}
	}
	return nil
}
func expandRecipe(argv []string, root, report string) []string {
	out := make([]string, len(argv))
	for i, arg := range argv {
		out[i] = strings.NewReplacer("{artifact}", filepath.Join(root, "artifact"), "{report}", report).Replace(arg)
	}
	return out
}
func commandAt(dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}
func executeLocal(root string, argv []string) error {
	fmt.Fprintf(os.Stderr, "Local command (runs with your account permissions): %q\n", argv)
	return commandAt(filepath.Join(root, "challenge"), argv[0], argv[1:]...)
}
func newRunDirectory(root string) (string, error) {
	parent := filepath.Join(root, ".sl", "runs")
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	return os.MkdirTemp(parent, "run-")
}
func doctorWorkspace(root string, w Workspace) error {
	problems := 0
	check := func(name string, err error) {
		if err != nil {
			problems++
			fmt.Printf("FAIL %s: %v\n", name, err)
		} else {
			fmt.Println("OK", name)
		}
	}
	check("frozen source", checkWorkspace(root, w))
	r, err := workspaceRecipe(root, w)
	check("local recipe", err)
	if err == nil {
		seen := map[string]bool{}
		for _, argv := range append(append([][]string{}, r.Setup...), r.Baseline, r.Check) {
			name := argv[0]
			if seen[name] {
				continue
			}
			seen[name] = true
			if strings.Contains(name, "/") {
				_, err = os.Stat(filepath.Join(root, "challenge", name))
			} else {
				_, err = exec.LookPath(name)
			}
			check("runtime "+name, err)
		}
	}
	client, err := newClient(w.API)
	check("API configuration", err)
	if err == nil {
		var policy map[string]any
		check("published admission policy", client.request("GET", "/v1/challenge-versions/"+url.PathEscape(w.VersionID)+"/admission", nil, &policy))
		if client.token == "" {
			fmt.Println("INFO Sign-in can wait until submission: sl auth login")
		} else {
			var me map[string]any
			check("saved sign-in", client.request("GET", "/v1/me", nil, &me))
		}
	}
	if _, err = gitOutput(filepath.Join(root, "artifact"), "remote", "get-url", "origin"); err != nil {
		fmt.Println("INFO Before submission, commit and push artifact/ to your own GitHub repository and configure its origin. GitHub CLI can create it through the API.")
	}
	fmt.Println("Docker is needed only if the recipe uses it. Hidden tests and hardware measurements still require hosted qualification.")
	if problems > 0 {
		return fmt.Errorf("doctor found %d problem(s)", problems)
	}
	return nil
}
func githubRepository(remote string) (string, error) {
	remote = strings.TrimSuffix(remote, ".git")
	repo := ""
	for _, prefix := range []string{"git@github.com:", "https://github.com/", "ssh://git@github.com/"} {
		if strings.HasPrefix(remote, prefix) {
			repo = strings.TrimPrefix(remote, prefix)
		}
	}
	if !repositoryPattern.MatchString(repo) {
		return "", errors.New("artifact origin must be a GitHub HTTPS or SSH URL without credentials")
	}
	return repo, nil
}
func submitWorkspace(root string, w Workspace, r LocalRecipe, args []string) error {
	f := flag.NewFlagSet("submit", flag.ContinueOnError)
	model := f.String("model", "", "actual model")
	harness := f.String("harness", "", "actual harness")
	estimate := f.String("estimated-score-ticks", "", "hidden/hardware estimate")
	reason := f.String("measurement-reason", "", "qualification evidence")
	if err := f.Parse(args); err != nil {
		return err
	}
	if len(f.Args()) > 0 {
		return errors.New("unexpected submit arguments")
	}
	artifact := filepath.Join(root, "artifact")
	top, err := gitOutput(artifact, "rev-parse", "--show-toplevel")
	realArtifact, realErr := filepath.EvalSymlinks(artifact)
	realTop, topErr := filepath.EvalSymlinks(top)
	if err != nil || realErr != nil || topErr != nil || realTop != realArtifact {
		return errors.New("artifact/ must be its own Git repository, committed and pushed before submission")
	}
	dirty, err := gitOutput(artifact, "status", "--porcelain")
	if err != nil {
		return err
	}
	if dirty != "" {
		return errors.New("artifact repository has uncommitted files; commit and push the final candidate first")
	}
	remote, err := gitOutput(artifact, "remote", "get-url", "origin")
	if err != nil {
		return errors.New("set the artifact repository's GitHub origin and push before submission")
	}
	repository, err := githubRepository(remote)
	if err != nil {
		return err
	}
	commit, err := gitOutput(artifact, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	dir, err := newRunDirectory(root)
	if err != nil {
		return err
	}
	report := filepath.Join(dir, "result.json")
	claim := filepath.Join(dir, "claim.json")
	manifest := filepath.Join(root, "challenge", "science-ladder.yaml")
	argv := []string{"--api", w.API, "--version", w.VersionID, "--manifest", manifest, "--artifact", artifact, "--out", claim, "--report", report}
	if len(r.CheckedGates) > 0 {
		argv = append(argv, "--checked-gates", strings.Join(r.CheckedGates, ","))
	}
	if *estimate != "" {
		argv = append(argv, "--estimated-score-ticks", *estimate, "--measurement-reason", *reason)
	}
	argv = append(argv, "--")
	argv = append(argv, expandRecipe(r.Check, root, report)...)
	old, err := os.Getwd()
	if err != nil {
		return err
	}
	if err = os.Chdir(filepath.Join(root, "challenge")); err != nil {
		return err
	}
	defer os.Chdir(old)
	if err = claimCommand(argv); err != nil {
		return err
	}
	m, err := readManifest(manifest)
	if err != nil {
		return err
	}
	return remoteCommand([]string{"submit", "--api", w.API, "--version", w.VersionID, "--manifest", manifest, "--artifact", artifact, "--claim", claim, "--repository", repository, "--commit", commit, "--license", m.Submission.License, "--model", *model, "--harness", *harness})
}
