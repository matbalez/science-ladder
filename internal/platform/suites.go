package platform

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"io"
	"net/http"
	"strings"
	"time"
)

type SecretSealer interface {
	Seal(context.Context, []byte, []byte) ([]byte, error)
	Open(context.Context, []byte, []byte) ([]byte, error)
}

func suiteAAD(id string) []byte { return []byte("science-ladder-suite-material-v1\x00" + id) }
func validateSuiteDocument(document []byte) error {
	var source struct {
		Files map[string]string `json:"files"`
	}
	if err := protocol.DecodeStrictBounded(document, &source, 2<<20); err != nil {
		return err
	}
	files := map[string][]byte{}
	defer func() {
		for _, b := range files {
			protocol.ZeroBytes(b)
		}
	}()
	for name, encoded := range source.Files {
		data, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil {
			return errors.New("hidden file content must be canonical base64")
		}
		files[name] = data
	}
	_, _, err := protocol.ArtifactFromFiles(files, protocol.SubmissionContract{AllowedPaths: []string{"*"}, AllowedExtensions: []string{".json", ".txt", ".csv", ".tsv", ".dat", ".bin", ".npy"}, MaxBytes: 1 << 20, MaxFiles: 1000, License: "MIT"})
	return err
}

func (s *Server) createSuite(w http.ResponseWriter, r *http.Request, u *User) error {
	if s.Store == nil || s.SuiteSealer == nil {
		return fail(503, "hidden_suite_storage_unconfigured", "Encrypted hidden suite storage must be configured before upload")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		return err
	}
	defer protocol.ZeroBytes(body)
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	var in struct {
		Document   string `json:"document"`
		License    string `json:"license"`
		Provenance string `json:"provenance"`
	}
	if err = readJSON(r, &in); err != nil {
		return err
	}
	if len(in.License) < 2 || len(in.Provenance) < 20 {
		return fail(422, "suite_rights_required", "Provide the suite license and a provenance/rights statement of at least 20 characters")
	}
	plain := []byte(in.Document)
	defer protocol.ZeroBytes(plain)
	if err = validateSuiteDocument(plain); err != nil {
		return fail(422, "suite_document_invalid", err.Error())
	}
	if replayed, e := s.replayBeforeFetch(w, r, u, body); e != nil {
		return e
	} else if replayed {
		return nil
	}
	if err = s.reserveFetch(r.Context(), u); err != nil {
		return err
	}
	ciphertext, material, err := protocol.SealHiddenSuite(plain)
	if err != nil {
		return err
	}
	defer protocol.ZeroBytes(material.Key)
	defer protocol.ZeroBytes(material.Salt)
	id := ID()
	private := raw(material)
	defer protocol.ZeroBytes(private)
	sealed, err := s.SuiteSealer.Seal(r.Context(), private, suiteAAD(id))
	if err != nil {
		return err
	}
	digest, err := s.Store.Put(r.Context(), ciphertext, "application/vnd.science-ladder.encrypted-suite")
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		if _, err := tx.Exec(r.Context(), `INSERT INTO artifacts(digest,blob_digest,size,media_type,owner_id) VALUES($1,$1,$2,'application/vnd.science-ladder.encrypted-suite',$3) ON CONFLICT DO NOTHING`, digest, len(ciphertext), u.ID); err != nil {
			return 0, nil, err
		}
		if _, err := tx.Exec(r.Context(), `INSERT INTO hidden_suites(id,owner_id,commitment,ciphertext_digest,encrypted_material,provenance,license) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, u.ID, material.Commitment, digest, sealed, in.Provenance, in.License); err != nil {
			return 0, nil, err
		}
		return 201, map[string]any{"id": id, "commitment": material.Commitment, "ciphertextDigest": digest, "status": "sealed"}, nil
	})
}
func (s *Server) suiteMaterial(ctx context.Context, commitment string) (protocol.SuiteKeyMaterial, string, error) {
	var material protocol.SuiteKeyMaterial
	if s.SuiteSealer == nil {
		return material, "", errors.New("hidden suite envelope encryption unavailable")
	}
	var id, digest string
	var encrypted []byte
	if err := s.DB.QueryRow(ctx, `SELECT id,ciphertext_digest,encrypted_material FROM hidden_suites WHERE commitment=$1`, commitment).Scan(&id, &digest, &encrypted); err != nil {
		return material, "", err
	}
	plain, err := s.SuiteSealer.Open(ctx, encrypted, suiteAAD(id))
	if err != nil {
		return material, "", err
	}
	defer protocol.ZeroBytes(plain)
	if err = protocol.DecodeStrict(plain, &material); err != nil {
		return material, "", err
	}
	if material.Commitment != commitment {
		return material, "", errors.New("hidden suite commitment and encrypted material mismatch")
	}
	return material, digest, nil
}
func (s *Server) grantHiddenSuite(ctx context.Context, job *protocol.RunnerJob, host runnerIdentity) error {
	if job.Manifest.Suite.Visibility != "hidden" || job.Purpose == "artifact_prepare" {
		return nil
	}
	public, err := base64.StdEncoding.Strict().DecodeString(host.EncryptionKey)
	if err != nil || len(public) != 32 {
		return fail(503, "host_encryption_key_missing", "Hidden suites require an enrolled X25519 host encryption key")
	}
	material, digest, err := s.suiteMaterial(ctx, job.Manifest.Suite.Commitment)
	if err != nil {
		return err
	}
	defer protocol.ZeroBytes(material.Key)
	defer protocol.ZeroBytes(material.Salt)
	capsule, err := protocol.WrapSuiteKey(public, material, protocol.HiddenSuiteContext(job.ID, material.Commitment))
	if err != nil {
		return err
	}
	var size int64
	if err = s.DB.QueryRow(ctx, `SELECT size FROM artifacts WHERE digest=$1`, digest).Scan(&size); err != nil {
		return err
	}
	url, err := s.Store.SignedRead(ctx, digest, 15*time.Minute)
	if err != nil {
		return err
	}
	job.HiddenSuite = &protocol.HiddenSuiteGrant{Source: protocol.ObjectRef{Digest: digest, Size: size, URL: url}, Commitment: material.Commitment, KeyCapsule: capsule}
	job.SuiteDigest = material.Commitment
	return nil
}
func (s *Server) requireSuiteOwner(ctx context.Context, m protocol.Manifest, u *User) error {
	if m.Suite.Visibility != "hidden" {
		return nil
	}
	if s.SuiteSealer == nil {
		return fail(503, "hidden_suite_storage_unconfigured", "Hidden suite key custody is not configured")
	}
	var exists bool
	if err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hidden_suites WHERE commitment=$1 AND owner_id=$2 AND revealed_at IS NULL)`, m.Suite.Commitment, u.ID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fail(422, "suite_commitment_unknown", "Upload and seal this hidden suite under your account before referencing its commitment")
	}
	return nil
}

func lockSuiteOwner(ctx context.Context, tx pgx.Tx, m protocol.Manifest, u *User) error {
	if m.Suite.Visibility != "hidden" {
		return nil
	}
	var id string
	if err := tx.QueryRow(ctx, `SELECT id FROM hidden_suites WHERE commitment=$1 AND owner_id=$2 AND revealed_at IS NULL FOR UPDATE`, m.Suite.Commitment, u.ID).Scan(&id); err != nil {
		return fail(409, "suite_not_available", "Hidden suite was revealed or is not owned by this creator")
	}
	return nil
}
func (s *Server) revealSuite(w http.ResponseWriter, r *http.Request, u *User) error {
	var commitment string
	err := s.DB.QueryRow(r.Context(), `SELECT commitment FROM hidden_suites WHERE id=$1 AND owner_id=$2`, r.PathValue("id"), u.ID).Scan(&commitment)
	if err != nil {
		return err
	}
	var safe bool
	if err = s.DB.QueryRow(r.Context(), `SELECT count(*)>0 AND bool_and((intake_status='closed' OR deadline<now()) AND manifest->'suite'->>'revealAt' IS NOT NULL AND (manifest->'suite'->>'revealAt')::timestamptz<=now()) FROM challenge_versions WHERE manifest->'suite'->>'commitment'=$1`, commitment).Scan(&safe); err != nil {
		return err
	}
	if !safe {
		return fail(409, "suite_reveal_not_due", "All bound seasons must be closed and their immutable suite reveal times must have passed")
	}
	material, digest, err := s.suiteMaterial(r.Context(), commitment)
	if err != nil {
		return err
	}
	defer protocol.ZeroBytes(material.Key)
	defer protocol.ZeroBytes(material.Salt)
	ciphertext, err := s.Store.Get(r.Context(), digest, 3<<20)
	if err != nil {
		return err
	}
	plaintext, err := protocol.DecryptSuiteObject(material, ciphertext, "source")
	if err != nil {
		return err
	}
	defer protocol.ZeroBytes(plaintext)
	// Keep source bytes encrypted in object storage even after disclosure.
	// Only the transaction below can declassify the encrypted objects and key.
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var exists bool
		if err := tx.QueryRow(r.Context(), `SELECT revealed_at IS NOT NULL FROM hidden_suites WHERE id=$1 AND owner_id=$2 FOR UPDATE`, r.PathValue("id"), u.ID).Scan(&exists); err != nil {
			return 0, nil, err
		}
		var stillSafe bool
		if err := tx.QueryRow(r.Context(), `SELECT count(*)>0 AND bool_and((intake_status='closed' OR deadline<now()) AND manifest->'suite'->>'revealAt' IS NOT NULL AND (manifest->'suite'->>'revealAt')::timestamptz<=now()) FROM challenge_versions WHERE manifest->'suite'->>'commitment'=$1`, commitment).Scan(&stillSafe); err != nil {
			return 0, nil, err
		}
		if !stillSafe {
			return 0, nil, fail(409, "suite_reveal_not_due", "A bound season is still active or its reveal time has not passed")
		}
		if exists {
			return 200, map[string]any{"id": r.PathValue("id"), "status": "revealed"}, nil
		}
		if _, err := tx.Exec(r.Context(), `UPDATE artifacts SET public_at=COALESCE(public_at,now()) WHERE digest=$1 OR digest IN(SELECT ru.digest FROM runner_uploads ru JOIN runner_jobs j ON j.id=ru.job_id JOIN challenge_versions v ON v.id=j.version_id WHERE v.manifest->'suite'->>'commitment'=$2 AND ru.role='suiteDisk' AND ru.verified)`, digest, commitment); err != nil {
			return 0, nil, err
		}
		receipt := protocol.Receipt{APIVersion: protocol.APIVersion, Kind: "SuiteRevealReceipt", ID: ID(), CreatedAt: time.Now().UTC(), Producer: "science-ladder", DeploymentMode: s.Config.DeploymentMode, EconomicMode: "none", SubjectDigest: commitment, Data: map[string]any{"suiteId": r.PathValue("id"), "commitment": commitment, "plaintextDigest": material.PlaintextDigest, "sourceDigest": digest, "sourceFormat": "encrypted-suite-object-v1", "encryptionRole": "source", "salt": base64.StdEncoding.EncodeToString(material.Salt), "suiteKey": base64.StdEncoding.EncodeToString(material.Key)}}
		rd, err := protocol.Digest(receipt)
		if err != nil {
			return 0, nil, err
		}
		if err = saveReceipt(r, tx, rd, receipt, u.ID, true); err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE hidden_suites SET revealed_at=now(),reveal_receipt_digest=$2 WHERE id=$1`, r.PathValue("id"), rd); err != nil {
			return 0, nil, err
		}
		return 200, map[string]any{"id": r.PathValue("id"), "status": "revealed", "receiptDigest": rd}, nil
	})
}
