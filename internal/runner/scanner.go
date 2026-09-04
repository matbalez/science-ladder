package runner

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

const AdvisoryPolicyVersion = "offline-advisory-v1"

type PackageCoordinate struct {
	Ecosystem     string `json:"ecosystem"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Digest        string `json:"digest,omitempty"`
	SourceName    string `json:"sourceName,omitempty"`
	SourceVersion string `json:"sourceVersion,omitempty"`
}
type RuntimeInventory struct {
	APIVersion               string              `json:"apiVersion"`
	RuntimeImageDigest       string              `json:"runtimeImageDigest"`
	ComponentInventoryDigest string              `json:"componentInventoryDigest,omitempty"`
	Packages                 []PackageCoordinate `json:"packages"`
}
type AdvisorySource struct {
	URL           string    `json:"url"`
	FetchedAt     time.Time `json:"fetchedAt"`
	ContentDigest string    `json:"contentDigest"`
}
type AdvisoryCoverage struct {
	Package    PackageCoordinate `json:"package"`
	Status     string            `json:"status"`
	SourceURL  string            `json:"sourceUrl"`
	Advisories []Advisory        `json:"advisories"`
}
type Advisory struct {
	ID        string `json:"id"`
	Severity  string `json:"severity"`
	SourceURL string `json:"sourceUrl"`
}
type AdvisorySnapshot struct {
	APIVersion  string             `json:"apiVersion"`
	Kind        string             `json:"kind"`
	ID          string             `json:"id"`
	GeneratedAt time.Time          `json:"generatedAt"`
	ExpiresAt   time.Time          `json:"expiresAt"`
	Sources     []AdvisorySource   `json:"sources"`
	Coverage    []AdvisoryCoverage `json:"coverage"`
}

func normalizePythonName(name string) (string, error) {
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`).MatchString(name) {
		return "", errors.New("invalid Python package name")
	}
	return regexp.MustCompile(`[-_.]+`).ReplaceAllString(strings.ToLower(name), "-"), nil
}
func normalizePythonVersion(version string) (string, error) {
	match := regexp.MustCompile(`(?i)^v?([0-9]+(?:\.[0-9]+)*)(?:(a|b|rc)([0-9]+))?(?:\.post([0-9]+))?(?:\.dev([0-9]+))?$`).FindStringSubmatch(version)
	if match == nil {
		return "", errors.New("dependency version is outside the canonical platform PEP 440 subset")
	}
	integer := func(s string) string {
		s = strings.TrimLeft(s, "0")
		if s == "" {
			return "0"
		}
		return s
	}
	parts := strings.Split(match[1], ".")
	for i := range parts {
		parts[i] = integer(parts[i])
	}
	out := strings.Join(parts, ".")
	if match[2] != "" {
		out += strings.ToLower(match[2]) + integer(match[3])
	}
	if match[4] != "" {
		out += ".post" + integer(match[4])
	}
	if match[5] != "" {
		out += ".dev" + integer(match[5])
	}
	return out, nil
}
func normalizePackage(p PackageCoordinate) (PackageCoordinate, error) {
	if len(p.Name) > 256 || len(p.Version) > 256 || p.Name == "" || p.Version == "" {
		return p, errors.New("invalid package coordinate")
	}
	switch p.Ecosystem {
	case "PyPI":
		name, err := normalizePythonName(p.Name)
		if err != nil {
			return p, err
		}
		version, err := normalizePythonVersion(p.Version)
		if err != nil {
			return p, err
		}
		p.Name = name
		p.Version = version
	case "Debian":
		if strings.ContainsAny(p.Name+p.Version, "\n\r\x00 ") || !regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]*$`).MatchString(p.Name) {
			return p, errors.New("invalid Debian package coordinate")
		}
	case "CPython":
		if p.Name != "cpython" {
			return p, errors.New("invalid interpreter coordinate")
		}
		version, err := normalizePythonVersion(p.Version)
		if err != nil {
			return p, err
		}
		p.Version = version
	default:
		return p, errors.New("unsupported advisory ecosystem")
	}
	return p, nil
}
func packageKey(p PackageCoordinate) string { return p.Ecosystem + ":" + p.Name + "@" + p.Version }

// DependencyInventory includes every runtime package and every locked wheel,
// including transitive vendored dependencies. Metadata cannot add unpinned code.
func DependencyInventory(files map[string][]byte, lockPath string, base RuntimeInventory) ([]PackageCoordinate, error) {
	if base.APIVersion != "science-ladder-runtime-inventory/v1" || !protocol.ValidDigest(base.RuntimeImageDigest) || len(base.Packages) == 0 {
		return nil, errors.New("complete platform runtime inventory required")
	}
	if _, err := lockedWheelFiles(files, lockPath); err != nil {
		return nil, err
	}
	packages := map[string]PackageCoordinate{}
	names := map[string]bool{}
	requirements := map[string][]string{}
	for _, p := range base.Packages {
		normalized, err := normalizePackage(p)
		if err != nil {
			return nil, err
		}
		key := packageKey(normalized)
		if _, ok := packages[key]; ok {
			return nil, errors.New("duplicate runtime inventory entry")
		}
		packages[key] = normalized
		if p.Ecosystem == "PyPI" {
			names[normalized.Name] = true
		}
	}
	for filename, data := range files {
		if !strings.HasPrefix(filename, "wheels/") {
			continue
		}
		reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		var metadata textproto.MIMEHeader
		count := 0
		for _, file := range reader.File {
			if strings.HasSuffix(file.Name, ".dist-info/METADATA") {
				count++
				if file.UncompressedSize64 > 128<<10 {
					return nil, errors.New("wheel metadata exceeds limit")
				}
				entry, err := file.Open()
				if err != nil {
					return nil, err
				}
				contents, err := io.ReadAll(io.LimitReader(entry, 128<<10))
				_ = entry.Close()
				if err != nil {
					return nil, err
				}
				metadata, err = textproto.NewReader(bufio.NewReader(bytes.NewReader(contents))).ReadMIMEHeader()
				if err != nil {
					return nil, errors.New("invalid wheel package metadata")
				}
			}
		}
		if count != 1 {
			return nil, errors.New("wheel must have exactly one METADATA identity")
		}
		coordinate, err := normalizePackage(PackageCoordinate{Ecosystem: "PyPI", Name: metadata.Get("Name"), Version: metadata.Get("Version"), Digest: protocol.DigestBytes(data)})
		if err != nil {
			return nil, err
		}
		parts := strings.Split(filepathBase(filename), "-")
		expectedName, err := normalizePythonName(parts[0])
		if err != nil {
			return nil, err
		}
		expectedVersion, err := normalizePythonVersion(parts[1])
		if err != nil || coordinate.Name != expectedName || coordinate.Version != expectedVersion {
			return nil, errors.New("wheel filename and metadata identity disagree")
		}
		key := packageKey(coordinate)
		if existing, ok := packages[key]; ok && existing.Digest != "" && existing.Digest != coordinate.Digest {
			return nil, errors.New("same dependency version has conflicting content hashes")
		}
		packages[key] = coordinate
		names[coordinate.Name] = true
		requirements[coordinate.Name] = metadata.Values("Requires-Dist")
	}
	dependencyPattern := regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)`)
	optionalOnly := regexp.MustCompile(`^extra\s*==\s*['"][A-Za-z0-9._-]+['"]$`)
	for owner, required := range requirements {
		for _, requirement := range required {
			parts := strings.SplitN(requirement, ";", 2)
			if len(parts) == 2 && optionalOnly.MatchString(strings.TrimSpace(parts[1])) {
				continue
			}
			match := dependencyPattern.FindStringSubmatch(strings.TrimSpace(parts[0]))
			if match == nil {
				return nil, errors.New("unrecognized transitive dependency metadata")
			}
			name, err := normalizePythonName(match[1])
			if err != nil || !names[name] {
				return nil, fmt.Errorf("unaccounted transitive dependency %s required by %s", match[1], owner)
			}
		}
	}
	out := make([]PackageCoordinate, 0, len(packages))
	for _, p := range packages {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return packageKey(out[i]) < packageKey(out[j]) })
	return out, nil
}
func filepathBase(filename string) string {
	parts := strings.Split(filename, "/")
	return parts[len(parts)-1]
}

func BuildSBOM(runtimeDigest string, packages []PackageCoordinate) ([]byte, error) {
	components := []any{map[string]any{"type": "container", "name": "science-ladder-python-runtime", "version": runtimeDigest, "hashes": []any{map[string]any{"alg": "SHA-256", "content": strings.TrimPrefix(runtimeDigest, "sha256:")}}}}
	for _, p := range packages {
		kind := map[string]string{"Debian": "deb/debian", "PyPI": "pypi", "CPython": "generic"}[p.Ecosystem]
		if kind == "" {
			return nil, errors.New("unsupported SBOM ecosystem")
		}
		component := map[string]any{"type": "library", "name": p.Name, "version": p.Version, "purl": "pkg:" + kind + "/" + url.PathEscape(p.Name) + "@" + url.PathEscape(p.Version)}
		if p.Digest != "" {
			component["hashes"] = []any{map[string]any{"alg": "SHA-256", "content": strings.TrimPrefix(p.Digest, "sha256:")}}
		}
		components = append(components, component)
	}
	raw, err := json.Marshal(map[string]any{"bomFormat": "CycloneDX", "specVersion": "1.6", "version": 1, "components": components})
	if err != nil {
		return nil, err
	}
	return protocol.CanonicalJSON(raw)
}

func ReadRuntimeInventory(file PinnedFile) (RuntimeInventory, error) {
	var inventory RuntimeInventory
	if err := verifyPinned(file); err != nil {
		return inventory, err
	}
	data, err := os.ReadFile(file.Path)
	if err != nil {
		return inventory, err
	}
	if err := protocol.DecodeStrict(data, &inventory); err != nil {
		return inventory, err
	}
	return inventory, nil
}

func ScanAdvisories(packages []PackageCoordinate, snapshot AdvisorySnapshot, now time.Time) ([]protocol.VulnerabilityFinding, string) {
	findings := []protocol.VulnerabilityFinding{}
	unknown := func(id string) {
		findings = append(findings, protocol.VulnerabilityFinding{Component: "platform advisory policy", ID: id, Severity: "unknown"})
	}
	if snapshot.APIVersion != "science-ladder-advisories/v1" || snapshot.Kind != "AdvisorySnapshot" || snapshot.ID == "" || !snapshot.ExpiresAt.After(now) || snapshot.GeneratedAt.After(now.Add(time.Minute)) || snapshot.GeneratedAt.Before(now.Add(-7*24*time.Hour)) || snapshot.ExpiresAt.Sub(snapshot.GeneratedAt) > 7*24*time.Hour {
		unknown("stale_or_invalid_advisory_snapshot")
		return findings, "unknown"
	}
	sources := map[string]bool{}
	for _, source := range snapshot.Sources {
		u, err := url.Parse(source.URL)
		if err != nil || u.Scheme != "https" || u.User != nil || !protocol.ValidDigest(source.ContentDigest) || source.FetchedAt.After(now.Add(time.Minute)) || source.FetchedAt.Before(snapshot.GeneratedAt.Add(-24*time.Hour)) {
			unknown("unverifiable_advisory_provenance")
			return findings, "unknown"
		}
		switch u.Hostname() {
		case "api.osv.dev", "osv.dev", "osv-vulnerabilities.storage.googleapis.com", "security-tracker.debian.org", "security-team.debian.org", "bugs.debian.org", "bugzilla.redhat.com", "api.github.com", "github.com", "raw.githubusercontent.com", "www.python.org", "pypi.org":
		default:
			unknown("unapproved_primary_advisory_source")
			return findings, "unknown"
		}
		sources[source.URL] = true
	}
	if len(sources) == 0 {
		unknown("missing_advisory_sources")
		return findings, "unknown"
	}
	coverage := map[string]AdvisoryCoverage{}
	for _, entry := range snapshot.Coverage {
		normalized, err := normalizePackage(entry.Package)
		if err != nil {
			unknown("invalid_advisory_coordinate")
			return findings, "unknown"
		}
		key := packageKey(normalized)
		if _, exists := coverage[key]; exists {
			unknown("duplicate_advisory_coverage")
			return findings, "unknown"
		}
		entry.Package = normalized
		coverage[key] = entry
	}
	status := "pass"
	for _, p := range packages {
		entry, ok := coverage[packageKey(p)]
		if !ok || entry.Package != p || entry.Status != "complete" || !sources[entry.SourceURL] {
			findings = append(findings, protocol.VulnerabilityFinding{Component: packageKey(p), ID: "coverage_missing_or_incomplete", Severity: "unknown", SourceURL: entry.SourceURL})
			status = "unknown"
			continue
		}
		for _, advisory := range entry.Advisories {
			severity := strings.ToLower(advisory.Severity)
			switch severity {
			case "none", "low", "moderate", "medium":
			case "high", "critical":
				if status != "unknown" {
					status = "fail"
				}
			default:
				status = "unknown"
				severity = "unknown"
			}
			if advisory.ID == "" || !sources[advisory.SourceURL] {
				status = "unknown"
				severity = "unknown"
			}
			findings = append(findings, protocol.VulnerabilityFinding{Component: packageKey(p), ID: advisory.ID, Severity: severity, SourceURL: advisory.SourceURL})
		}
	}
	return findings, status
}

func (b *Builder) Scan(files map[string][]byte, m protocol.Manifest, sbomPath string) (protocol.VulnerabilityScan, protocol.ObjectRef, error) {
	scan := protocol.VulnerabilityScan{PolicyVersion: AdvisoryPolicyVersion, ScannedAt: time.Now().UTC(), Status: "unknown", Findings: []protocol.VulnerabilityFinding{}}
	if b.Runtime == nil {
		return scan, protocol.ObjectRef{}, errors.New("platform-controlled signed advisory policy is not configured")
	}
	config := b.Runtime.Config
	inventory, err := ReadRuntimeInventory(config.RuntimeInventory)
	if err != nil {
		return scan, protocol.ObjectRef{}, err
	}
	if inventory.RuntimeImageDigest != m.Validator.RuntimeImageDigest {
		return scan, protocol.ObjectRef{}, errors.New("runtime inventory does not bind validator image")
	}
	packages, err := DependencyInventory(files, m.Validator.DependencyLock, inventory)
	if err != nil {
		return scan, protocol.ObjectRef{}, err
	}
	sbom, err := BuildSBOM(m.Validator.RuntimeImageDigest, packages)
	if err != nil {
		return scan, protocol.ObjectRef{}, err
	}
	if err := os.WriteFile(sbomPath, sbom, 0400); err != nil {
		return scan, protocol.ObjectRef{}, err
	}
	ref := protocol.ObjectRef{Digest: protocol.DigestBytes(sbom), Size: int64(len(sbom))}
	scan.SBOMDigest = ref.Digest
	scan.RuntimeInventoryDigest = config.RuntimeInventory.Digest
	scan.PackagesChecked = len(packages)
	if err := verifyPinned(config.AdvisorySnapshot); err != nil {
		return scan, ref, err
	}
	if err := verifyPinned(config.AdvisoryKeys); err != nil {
		return scan, ref, err
	}
	keys, err := ReadPublicKeys(config.AdvisoryKeys.Path)
	if err != nil {
		return scan, ref, err
	}
	data, err := os.ReadFile(config.AdvisorySnapshot.Path)
	if err != nil {
		return scan, ref, err
	}
	var envelope protocol.Envelope
	if err := protocol.DecodeStrict(data, &envelope); err != nil {
		return scan, ref, err
	}
	payload, err := protocol.Verify(envelope, keys)
	if err != nil {
		return scan, ref, errors.New("advisory snapshot lacks trusted platform signature")
	}
	var snapshot AdvisorySnapshot
	if err := protocol.DecodeStrict(payload, &snapshot); err != nil {
		return scan, ref, err
	}
	scan.AdvisorySnapshotDigest = config.AdvisorySnapshot.Digest
	scan.Findings, scan.Status = ScanAdvisories(packages, snapshot, scan.ScannedAt)
	if scan.Status != "pass" {
		return scan, ref, errors.New("vulnerability review has high/critical findings, stale data or incomplete coverage")
	}
	return scan, ref, nil
}
