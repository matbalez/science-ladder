package platform

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/http"
	"time"
)

func (s *Server) signer() (crypto.Signer, error) {
	if s.ReceiptSigner == nil {
		return nil, errors.New("platform receipt signer is not configured")
	}
	return s.ReceiptSigner, nil
}
func (s *Server) signReceipt(ctx context.Context, digest string) error {
	key, err := s.signer()
	if err != nil {
		return err
	}
	var payload []byte
	err = s.DB.QueryRow(ctx, `SELECT payload FROM receipts WHERE digest=$1 AND envelope IS NULL`, digest).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if e := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM receipts WHERE digest=$1 AND envelope IS NOT NULL)`, digest).Scan(&exists); e != nil {
			return e
		}
		if exists {
			return nil
		}
	}
	if err != nil {
		return err
	}
	var v any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err = decoder.Decode(&v); err != nil {
		return err
	}
	actual, err := protocol.Digest(v)
	if err != nil {
		return err
	}
	if actual != digest {
		return errors.New("persisted receipt payload differs from its immutable digest")
	}
	envelope, err := protocol.Sign(s.Config.ReceiptKeyID, key, v)
	if err != nil {
		return err
	}
	var hostBytes []byte
	hostErr := s.DB.QueryRow(ctx, `SELECT envelope FROM runner_results WHERE digest=$1`, digest).Scan(&hostBytes)
	if hostErr != nil && !errors.Is(hostErr, pgx.ErrNoRows) {
		return hostErr
	}
	if hostErr == nil {
		var hostEnvelope protocol.Envelope
		if err = json.Unmarshal(hostBytes, &hostEnvelope); err != nil {
			return err
		}
		if hostEnvelope.Payload != envelope.Payload || hostEnvelope.PayloadType != envelope.PayloadType {
			return errors.New("runner receipt countersignature payload mismatch")
		}
		for _, sig := range hostEnvelope.Signatures {
			if sig.KeyID == s.Config.ReceiptKeyID {
				return errors.New("runner and platform signing identities must be distinct")
			}
			envelope.Signatures = append(envelope.Signatures, sig)
		}
	}
	_, err = s.DB.Exec(ctx, `UPDATE receipts SET envelope=$2 WHERE digest=$1 AND envelope IS NULL`, digest, raw(envelope))
	return err
}
func (s *Server) keys(w http.ResponseWriter, r *http.Request, u *User) error {
	key, err := s.signer()
	if err != nil {
		respond(w, 200, map[string]any{"apiVersion": protocol.APIVersion, "keys": []any{}, "deploymentMode": s.Config.DeploymentMode, "keyHistory": s.KeyHistory, "witnessQuorum": false, "officialAcceptance": false})
		return nil
	}
	der, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return err
	}
	custody := "configured-online-key"
	if s.Config.ReceiptKMSKeyID != "" {
		custody = "managed-kms"
	}
	keys := []any{map[string]any{"id": s.Config.ReceiptKeyID, "algorithm": "ECDSA-P256-SHA256", "publicKeyPem": string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), "roles": []string{"control-plane-receipt", "audit-checkpoint"}, "custody": custody}}
	rows, err := s.DB.Query(r.Context(), `SELECT id,host_group,public_key,enabled FROM runner_hosts ORDER BY id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, group, pub string
		var enabled bool
		if err = rows.Scan(&id, &group, &pub, &enabled); err != nil {
			rows.Close()
			return err
		}
		if _, err = parseRunnerKey(pub); err != nil {
			continue
		}
		keys = append(keys, map[string]any{"id": id, "hostId": id, "hostGroup": group, "algorithm": "ECDSA-P256-SHA256", "publicKeyPem": pub, "roles": []string{"validation-run"}, "enabled": enabled, "custody": "operator-enrolled-host-key"})
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	var witnessed *time.Time
	if err = s.DB.QueryRow(r.Context(), `SELECT max(issued_at) FROM audit_checkpoints WHERE quorum_verified_at IS NOT NULL`).Scan(&witnessed); err != nil {
		return err
	}
	quorum := witnessed != nil && time.Since(*witnessed) <= time.Hour
	official := false
	limitations := []string{"Root delegation, external witness quorum, and independent security review are required before production competition."}
	if s.Config.DeploymentMode == "production" {
		tx, err := s.DB.Begin(r.Context())
		if err != nil {
			return err
		}
		gateErr := s.checkProductionAdmission(r.Context(), tx)
		tx.Rollback(r.Context())
		official = gateErr == nil
		if official {
			limitations = []string{}
		} else {
			limitations = []string{"Production admission is paused because a required signed release, custody, delegation, or witness freshness gate is unavailable."}
		}
	}
	respond(w, 200, map[string]any{"apiVersion": protocol.APIVersion, "keys": keys, "deploymentMode": s.Config.DeploymentMode, "keyHistory": s.KeyHistory, "witnessQuorum": quorum, "officialAcceptance": official, "limitations": limitations})

	return nil
}
