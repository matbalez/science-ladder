package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

const reviewEvidencePolicy = "pinned-text-evidence-v1"
const reviewEvidenceLimit = 192 << 10

type reviewEvidenceFile struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Text   string `json:"text"`
}
type reviewSourceEvidence struct {
	Policy         string               `json:"policy"`
	SourceCommit   string               `json:"sourceCommit"`
	SourceDigest   string               `json:"sourceSnapshotDigest"`
	ManifestDigest string               `json:"manifestDigest"`
	Files          []reviewEvidenceFile `json:"files"`
	Omitted        int                  `json:"omittedSelectedFiles"`
	Limitations    []string             `json:"limitations"`
}

// Only bounded, explicitly selected public scientific files reach the model.
// No walking of the filesystem, repository instructions, hidden suite object,
// solver uploads, credentials, or execution is involved.
func selectReviewEvidence(document []byte, sourceDigest, commit string, repositoryID int64, manifest protocol.Manifest) (reviewSourceEvidence, error) {
	result := reviewSourceEvidence{Policy: reviewEvidencePolicy, SourceCommit: commit, SourceDigest: sourceDigest, Files: []reviewEvidenceFile{}, Limitations: []string{"Repository text is creator-authored untrusted evidence; included tests and reproduction claims are not proof that they ran.", "Only selected bounded textual files are included. Hidden suites, private solver artifacts, credentials and repository instruction files are excluded.", "Primary-source retrieval checks establish observed text and metadata, not truth, licensing authority, completeness of literature, or the absence of metadata errors."}}
	if len(document) > 64<<20 || protocol.DigestBytes(document) != sourceDigest {
		return result, errors.New("scientific evidence snapshot digest mismatch")
	}
	var snapshot struct {
		RepositoryID int64             `json:"repositoryId"`
		SourceCommit string            `json:"sourceCommit"`
		Files        map[string][]byte `json:"files"`
	}
	if err := protocol.DecodeStrictBounded(document, &snapshot, 64<<20); err != nil {
		return result, errors.New("scientific evidence snapshot is invalid")
	}
	if snapshot.SourceCommit != commit || snapshot.RepositoryID != repositoryID {
		return result, errors.New("scientific evidence source binding mismatch")
	}
	sourceManifest, err := protocol.ParseManifest(snapshot.Files["science-ladder.yaml"])
	if err != nil {
		return result, errors.New("scientific evidence manifest is invalid")
	}
	expected, err := protocol.Digest(manifest)
	if err != nil {
		return result, err
	}
	actual, err := protocol.Digest(sourceManifest)
	if err != nil || expected != actual {
		return result, errors.New("scientific evidence manifest binding mismatch")
	}
	result.ManifestDigest = expected
	selected := []string{"challenge-brief.md", "THIRD_PARTY_NOTICES.md", "LICENSE", "DATA_LICENSE.md", "literature/reference.json", "docs/source-evidence.json", "tests/test_checker.py", "tools/reproduce.py"}
	for _, arg := range manifest.Validator.Entrypoint {
		if strings.HasPrefix(arg, "/sl/challenge/") {
			selected = append(selected, strings.TrimPrefix(arg, "/sl/challenge/"))
		}
	}
	if manifest.Suite.Visibility == "public" {
		selected = append(selected, path.Join(manifest.Suite.Path, "contract.json"))
	}
	for _, f := range manifest.Fixtures {
		if f.Name == "baseline" {
			for name := range snapshot.Files {
				if strings.HasPrefix(name, f.Path+"/") {
					selected = append(selected, name)
				}
			}
		}
	}
	sort.Strings(selected)
	seen := map[string]bool{}
	for _, name := range selected {
		if seen[name] {
			continue
		}
		seen[name] = true
		data, ok := snapshot.Files[name]
		if !ok {
			continue
		}
		if !safeReviewPath(name, manifest) || len(data) > 32<<10 || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 || reviewSecretMarker.Match(data) || len(result.Files) >= 24 {
			result.Omitted++
			continue
		}
		item := reviewEvidenceFile{Path: name, Digest: protocol.DigestBytes(data), Text: string(data)}
		result.Files = append(result.Files, item)
		if len(raw(result)) > reviewEvidenceLimit {
			result.Files = result.Files[:len(result.Files)-1]
			result.Omitted++
		}
	}
	return result, nil
}

var reviewSecretMarker = regexp.MustCompile(`(?i)-----BEGIN [A-Z ]*PRIVATE KEY-----|\bsk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{16,}|\b(?:ghp_|gho_|github_pat_)[A-Za-z0-9_]{16,}|\bAKIA[A-Z0-9]{16}\b|(?:password|api[_-]?key|access[_-]?token|client[_-]?secret)\s*[=:]\s*["']?[^\s"'<>]{8,}`)

func safeReviewPath(name string, m protocol.Manifest) bool {
	if protocol.ValidatePath(name) != nil {
		return false
	}
	lower := strings.ToLower(name)
	for _, component := range strings.Split(lower, "/") {
		if strings.HasPrefix(component, ".") || component == "private" || component == "secrets" || component == "credentials" || component == "agents.md" || component == "skill.md" {
			return false
		}
	}
	if m.Suite.Visibility == "hidden" && (name == m.Suite.Path || strings.HasPrefix(name, m.Suite.Path+"/")) {
		return false
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".md", ".txt", ".json", ".py":
		return true
	case "":
		return path.Base(name) == "LICENSE"
	}
	return false
}

func (s *Server) scientificEvidence(ctx context.Context, version string, m protocol.Manifest) (map[string]any, error) {
	if s.Store == nil {
		return nil, errors.New("scientific evidence storage unavailable")
	}
	var sourceDigest, commit string
	var repoID int64
	if err := s.DB.QueryRow(ctx, `SELECT source_digest,source_commit,repository_id FROM challenge_versions WHERE id=$1`, version).Scan(&sourceDigest, &commit, &repoID); err != nil {
		return nil, err
	}
	document, err := s.Store.Get(ctx, sourceDigest, 64<<20)
	if err != nil {
		return nil, errors.New("pinned scientific evidence could not be loaded")
	}
	source, err := selectReviewEvidence(document, sourceDigest, commit, repoID, m)
	if err != nil {
		return nil, err
	}
	// These records were accepted by the authenticated signed-run ingress. Include
	// the actual fixture envelopes, not user-authored claims of conformance.
	machine, err := queryObjects(ctx, s.DB, `SELECT jsonb_build_object('id',p.id,'status',p.status,'findings',p.findings,'freshVmRuns',p.reports->'freshVmRuns','scansPassed',p.reports->'scansPassed','hostileCorpusPassed',p.reports->'hostileCorpusPassed','fixtures',p.reports->'fixtures','machineReceiptDigest',p.machine_receipt_digest) FROM preflights p WHERE p.version_id=$1 ORDER BY p.created_at DESC LIMIT 2`, version)
	if err != nil {
		return nil, err
	}
	boundedMachine := []json.RawMessage{}
	size := 0
	for _, item := range machine {
		if len(item) <= 128<<10 && size+len(item) <= 192<<10 {
			boundedMachine = append(boundedMachine, item)
			size += len(item)
		}
	}
	prior, err := queryObjects(ctx, s.DB, `SELECT jsonb_build_object('id',id,'digest',digest,'status',status,'report',report) FROM review_runs WHERE version_id=$1 AND kind='scientific-legibility' ORDER BY created_at DESC LIMIT 3`, version)
	if err != nil {
		return nil, err
	}
	boundedPrior := []json.RawMessage{}
	size = 0
	for _, item := range prior {
		if len(item) <= 32<<10 && size+len(item) <= 64<<10 {
			boundedPrior = append(boundedPrior, item)
			size += len(item)
		}
	}
	return map[string]any{"source": source, "platformRecordedMachineEvidence": boundedMachine, "previousReviews": boundedPrior}, nil
}

func (s *Server) requestScientificReview(w http.ResponseWriter, r *http.Request, u *User) error {
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var in struct {
			Reason string `json:"reason"`
		}
		if err := readJSON(r, &in); err != nil {
			return 0, nil, err
		}
		in.Reason = strings.TrimSpace(in.Reason)
		if len(in.Reason) < 20 || len(in.Reason) > 2000 {
			return 0, nil, fail(422, "review_reason_required", "Explain the corrected evidence or request for re-review in 20–2,000 characters")
		}
		if s.Config.OpenAIKey == "" || s.Store == nil {
			return 0, nil, fail(503, "scientific_review_unavailable", "Scientific review and immutable storage must be configured")
		}
		version := r.PathValue("id")
		var locked bool
		if err := tx.QueryRow(r.Context(), `SELECT v.lock_digest IS NOT NULL FROM challenge_versions v JOIN challenges c ON c.id=v.challenge_id WHERE v.id=$1 AND c.owner_id=$2 FOR UPDATE OF v`, version, u.ID).Scan(&locked); err != nil {
			return 0, nil, err
		}
		if locked {
			return 0, nil, fail(409, "review_contract_locked", "A locked contract requires a new version for review changes")
		}
		var previous, queued bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM review_runs WHERE version_id=$1 AND kind='scientific-legibility'),EXISTS(SELECT 1 FROM scientific_review_requests WHERE version_id=$1 AND status='queued')`, version).Scan(&previous, &queued); err != nil {
			return 0, nil, err
		}
		if !previous {
			return 0, nil, fail(409, "initial_review_required", "Request preflight to perform the initial scientific review")
		}
		if queued {
			return 0, nil, fail(409, "review_already_pending", "A scientific re-review is already pending")
		}
		if err := s.reservePreparation(r.Context(), tx, u); err != nil {
			return 0, nil, err
		}
		id := ID()
		if _, err := tx.Exec(r.Context(), `INSERT INTO scientific_review_requests(id,version_id,requested_by,reason) VALUES($1,$2,$3,$4)`, id, version, u.ID, in.Reason); err != nil {
			return 0, nil, err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE challenge_versions SET review_status='pending' WHERE id=$1`, version); err != nil {
			return 0, nil, err
		}
		if err := enqueue(r.Context(), tx, "scientific_rereview", id); err != nil {
			return 0, nil, err
		}
		return 202, map[string]any{"id": id, "versionId": version, "status": "queued", "evidencePolicy": reviewEvidencePolicy}, nil
	})
}

func (s *Server) scientificRereview(ctx context.Context, id string) error {
	var version, status, reason string
	if err := s.DB.QueryRow(ctx, `SELECT version_id,status,reason FROM scientific_review_requests WHERE id=$1`, id).Scan(&version, &status, &reason); err != nil {
		return err
	}
	if status == "completed" {
		return nil
	}
	return s.scientificReviewAttempt(ctx, version, id, reason)
}
