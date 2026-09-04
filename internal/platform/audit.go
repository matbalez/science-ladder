package platform

import (
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	logaudit "github.com/matbalez/science-ladder/internal/audit"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/http"
	"os"
	"strconv"
	"time"
)

func (s *Server) loadTrust() error {
	if s.Config.KeyHistoryFile == "" && s.Config.RootPublicKeyFile == "" {
		return nil
	}
	rootBytes, err := os.ReadFile(s.Config.RootPublicKeyFile)
	if err != nil {
		return errors.New("cannot load pinned audit root public key")
	}
	root, err := logaudit.ParsePublicKey(string(rootBytes))
	if err != nil {
		return err
	}
	b, err := os.ReadFile(s.Config.KeyHistoryFile)
	if err != nil {
		return errors.New("cannot load signed key history")
	}
	var envelopes []protocol.Envelope
	if err = json.Unmarshal(b, &envelopes); err != nil {
		return errors.New("key history file must be an array of signed history envelopes")
	}
	if len(envelopes) == 0 || len(envelopes) > 100 {
		return errors.New("key history chain length invalid")
	}
	var previous *logaudit.History
	for _, envelope := range envelopes {
		h, e := logaudit.VerifyHistory(envelope, s.Config.RootKeyID, root, previous, time.Now().UTC())
		if e != nil {
			return e
		}
		previous = &h
	}
	s.TrustedRootKey = root
	if s.Config.ReleaseAttestationFile != "" {
		b, e := os.ReadFile(s.Config.ReleaseAttestationFile)
		if e != nil {
			return errors.New("cannot load root-signed release attestation")
		}
		var env protocol.Envelope
		if e = protocol.DecodeStrict(b, &env); e != nil {
			return e
		}
		s.ReleaseEnvelope = &env
	}
	s.TrustHistory = previous
	s.KeyHistory = envelopes
	return nil
}
func readAuditEvents(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, after int64, limit int) ([]logaudit.Event, error) {
	rows, err := q.Query(ctx, `SELECT sequence::text,kind,payload,previous_digest,digest FROM audit_events WHERE sequence>$1 ORDER BY sequence LIMIT $2`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []logaudit.Event{}
	for rows.Next() {
		var e logaudit.Event
		if err = rows.Scan(&e.Sequence, &e.Kind, &e.Payload, &e.PreviousDigest, &e.Digest); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
func (s *Server) publicAuditEvents(w http.ResponseWriter, r *http.Request, u *User) error {
	after, err := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if r.URL.Query().Get("after") == "" {
		after = 0
		err = nil
	}
	if err != nil || after < 0 {
		return fail(400, "cursor_invalid", "Audit cursor must be a nonnegative integer")
	}
	limit := 1000
	if n, e := strconv.Atoi(r.URL.Query().Get("limit")); e == nil && n > 0 && n <= 10000 {
		limit = n
	}
	events, err := readAuditEvents(r.Context(), s.DB, after, limit)
	if err != nil {
		return err
	}
	respond(w, 200, map[string]any{"events": events})
	return nil
}
func (s *Server) checkpointTick(ctx context.Context) error {
	key, err := s.signer()
	if err != nil {
		return nil
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(6842077293)`); err != nil {
		return err
	}
	var previous *logaudit.Checkpoint
	var lastPayload []byte
	var pending *string
	var issued time.Time
	err = tx.QueryRow(ctx, `SELECT payload,CASE WHEN envelope IS NULL THEN digest END,issued_at FROM audit_checkpoints ORDER BY id DESC LIMIT 1`).Scan(&lastPayload, &pending, &issued)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var checkpoint logaudit.Checkpoint
	var digest string
	if pending != nil {
		digest = *pending
		if err = json.Unmarshal(lastPayload, &checkpoint); err != nil {
			return err
		}
	} else {
		after := int64(0)
		if len(lastPayload) > 0 {
			previous = &logaudit.Checkpoint{}
			if err = json.Unmarshal(lastPayload, previous); err != nil {
				return err
			}
			after, err = strconv.ParseInt(previous.ToSequence, 10, 64)
			if err != nil {
				return err
			}
		}
		events, err := readAuditEvents(ctx, tx, after, 1000)
		if err != nil {
			return err
		}
		if previous != nil && time.Since(issued) < time.Minute && len(events) < 100 {
			return nil
		}
		checkpoint, err = logaudit.Build(s.Config.PublicOrigin, events, previous, time.Now().UTC())
		if err != nil {
			return err
		}
		digest, err = logaudit.CanonicalDigest(checkpoint)
		if err != nil {
			return err
		}
		from, _ := strconv.ParseInt(checkpoint.FromSequence, 10, 64)
		to, _ := strconv.ParseInt(checkpoint.ToSequence, 10, 64)
		if _, err = tx.Exec(ctx, `INSERT INTO audit_checkpoints(digest,payload,from_sequence,to_sequence,issued_at) VALUES($1,$2,$3,$4,$5)`, digest, raw(checkpoint), from, to, checkpoint.IssuedAt); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if s.Config.DeploymentMode == "production" {
		if s.TrustHistory == nil {
			return errors.New("production checkpoints require root-signed key delegation")
		}
		allowed := s.TrustHistory.KeysAt("audit-checkpoint", checkpoint.IssuedAt)
		configured, err := logaudit.Fingerprint(key.Public())
		if err != nil {
			return err
		}
		delegated, err := logaudit.Fingerprint(allowed[s.Config.ReceiptKeyID])
		if err != nil || configured != delegated {
			return errors.New("checkpoint signer has no valid root delegation")
		}
	}
	envelope, err := protocol.Sign(s.Config.ReceiptKeyID, key, checkpoint)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(ctx, `UPDATE audit_checkpoints SET envelope=$2 WHERE digest=$1 AND envelope IS NULL`, digest, raw(envelope))
	return err
}
func (s *Server) publicCheckpoints(w http.ResponseWriter, r *http.Request, u *User) error {
	after, parseErr := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if r.URL.Query().Get("after") != "" && (parseErr != nil || after < 0) {
		return fail(400, "cursor_invalid", "Checkpoint cursor must be a nonnegative integer")
	}
	if digest := r.URL.Query().Get("afterDigest"); digest != "" {
		if r.URL.Query().Get("after") != "" {
			return fail(400, "cursor_ambiguous", "Use afterDigest or after, not both")
		}
		if err := s.DB.QueryRow(r.Context(), `SELECT id FROM audit_checkpoints WHERE digest=$1 AND envelope IS NOT NULL`, digest).Scan(&after); err != nil {
			return err
		}
	}
	limit := 20
	if text := r.URL.Query().Get("limit"); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil || n < 1 || n > 20 {
			return fail(400, "limit_invalid", "Checkpoint limit must be between 1 and 20")
		}
		limit = n
	}
	rows, err := s.DB.Query(r.Context(), `SELECT id,digest,envelope,from_sequence,to_sequence,issued_at,quorum_verified_at FROM audit_checkpoints WHERE id>$1 AND envelope IS NOT NULL ORDER BY id LIMIT $2`, after, limit)
	if err != nil {
		return err
	}
	type record struct {
		id       int64
		digest   string
		envelope []byte
		from, to int64
		issued   time.Time
		quorum   *time.Time
	}
	records := []record{}
	for rows.Next() {
		var row record
		if err = rows.Scan(&row.id, &row.digest, &row.envelope, &row.from, &row.to, &row.issued, &row.quorum); err != nil {
			rows.Close()
			return err
		}
		records = append(records, row)
	}
	rows.Close()
	out := []any{}
	for _, row := range records {
		var envelope protocol.Envelope
		if err = json.Unmarshal(row.envelope, &envelope); err != nil {
			return err
		}
		events := []logaudit.Event{}
		if row.to >= row.from {
			events, err = readAuditEvents(r.Context(), s.DB, row.from-1, int(row.to-row.from+1))
			if err != nil {
				return err
			}
		}
		ws, err := queryObjects(r.Context(), s.DB, `SELECT envelope FROM witness_receipts WHERE checkpoint_digest=$1 ORDER BY key_id`, row.digest)
		if err != nil {
			return err
		}
		witnesses := []protocol.Envelope{}
		for _, w := range ws {
			var e protocol.Envelope
			if err = json.Unmarshal(w, &e); err != nil {
				return err
			}
			witnesses = append(witnesses, e)
		}
		out = append(out, map[string]any{"id": strconv.FormatInt(row.id, 10), "digest": row.digest, "bundle": logaudit.Bundle{Checkpoint: envelope, Witnesses: witnesses, Events: events}, "quorumVerified": row.quorum != nil, "issuedAt": row.issued})
	}
	respond(w, 200, map[string]any{"checkpoints": out, "deploymentMode": s.Config.DeploymentMode})
	return nil
}
func (s *Server) witnessReceipt(w http.ResponseWriter, r *http.Request, u *User) error {
	if s.TrustHistory == nil {
		return fail(503, "witness_trust_unconfigured", "An externally pinned root and signed witness identities must be configured")
	}
	var in struct {
		Envelope protocol.Envelope `json:"envelope"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	var payload []byte
	var envelopeBytes []byte
	var issued time.Time
	if err := s.DB.QueryRow(r.Context(), `SELECT payload,envelope,issued_at FROM audit_checkpoints WHERE digest=$1 AND envelope IS NOT NULL`, r.PathValue("digest")).Scan(&payload, &envelopeBytes, &issued); err != nil {
		return err
	}
	var platformEnvelope protocol.Envelope
	if err := json.Unmarshal(envelopeBytes, &platformEnvelope); err != nil {
		return err
	}
	if in.Envelope.Payload != platformEnvelope.Payload || in.Envelope.PayloadType != platformEnvelope.PayloadType || len(in.Envelope.Signatures) != 1 {
		return fail(422, "checkpoint_mismatch", "A witness must sign exactly one known committed checkpoint with one delegated identity")
	}
	keys := s.TrustHistory.KeysAt("audit-witness", issued)
	if _, err := protocol.Verify(in.Envelope, keys); err != nil {
		return fail(401, "witness_signature_invalid", "Witness signature or delegation is invalid")
	}
	id := in.Envelope.Signatures[0].KeyID
	if s.TrustHistory.WitnessOperatorsAt(issued)[id] == "" {
		return fail(403, "witness_not_registered", "Witness is not part of the configured independent quorum")
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `SELECT id FROM audit_checkpoints WHERE digest=$1 FOR UPDATE`, r.PathValue("digest")); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO witness_receipts(checkpoint_digest,key_id,envelope) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, r.PathValue("digest"), id, raw(in.Envelope)); err != nil {
		return err
	}
	existing, err := queryObjects(r.Context(), tx, `SELECT envelope FROM witness_receipts WHERE checkpoint_digest=$1`, r.PathValue("digest"))
	if err != nil {
		return err
	}
	witnesses := []protocol.Envelope{}
	for _, item := range existing {
		var e protocol.Envelope
		if err = json.Unmarshal(item, &e); err != nil {
			return err
		}
		witnesses = append(witnesses, e)
	}
	quorum := logaudit.VerifyQuorum(platformEnvelope, witnesses, keys, s.TrustHistory.WitnessOperatorsAt(issued), 2) == nil
	if quorum {
		if _, err = tx.Exec(r.Context(), `UPDATE audit_checkpoints SET quorum_verified_at=COALESCE(quorum_verified_at,now()) WHERE digest=$1`, r.PathValue("digest")); err != nil {
			return err
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	respond(w, 200, map[string]any{"accepted": true, "quorumVerified": quorum})
	return nil
}
func (s *Server) checkProductionAdmission(ctx context.Context, tx pgx.Tx) error {
	if s.Config.DeploymentMode != "production" {
		return nil
	}
	if s.Config.ReceiptKMSKeyID == "" || s.TrustHistory == nil {
		return fail(503, "production_trust_incomplete", "Production acceptance requires KMS receipt custody and externally pinned root-signed key history")
	}
	if s.ReleaseEnvelope == nil {
		return fail(503, "external_release_gates_incomplete", "Production acceptance requires root-signed evidence of the external security review, drills, and invitation pilot")
	}
	historyDigest, err := logaudit.CanonicalDigest(s.TrustHistory)
	if err != nil {
		return err
	}
	if _, err = logaudit.VerifyRelease(*s.ReleaseEnvelope, s.Config.RootKeyID, s.TrustedRootKey, historyDigest, s.Config.SourceCommit, time.Now()); err != nil {
		return fail(503, "release_attestation_invalid", "The root-signed external release evidence does not authorize this deployment")
	}
	keys := s.TrustHistory.KeysAt("control-plane-receipt", time.Now())
	var expected crypto.PublicKey = keys[s.Config.ReceiptKeyID]
	if expected == nil || s.ReceiptSigner == nil {
		return fail(503, "receipt_delegation_invalid", "Receipt signing delegation is missing or expired")
	}
	a, _ := logaudit.Fingerprint(expected)
	b, _ := logaudit.Fingerprint(s.ReceiptSigner.Public())
	if a != b {
		return fail(503, "receipt_delegation_invalid", "Receipt signer does not match its root-signed delegation")
	}
	var genesis string
	if err := tx.QueryRow(ctx, `SELECT digest FROM audit_checkpoints ORDER BY id LIMIT 1`).Scan(&genesis); err != nil {
		return fail(503, "audit_genesis_missing", "Audit history has not been bootstrapped")
	}
	if genesis != s.TrustHistory.GenesisCheckpointDigest {
		return fail(503, "audit_genesis_mismatch", "Audit genesis differs from the externally pinned history")
	}
	var last *time.Time
	if err := tx.QueryRow(ctx, `SELECT max(issued_at) FROM audit_checkpoints WHERE quorum_verified_at IS NOT NULL`).Scan(&last); err != nil {
		return err
	}
	if last == nil || !logaudit.IntakeAllowed(*last, time.Now()) {
		return fail(503, "witness_quorum_stale", "Independent witness quorum is unavailable beyond the one-hour grace period")
	}
	return nil
}
