package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/internal/runner"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

const runnerAuthorizationLifetime = 24 * time.Hour

// An omitted purposes member preserves old workers. An explicit empty array
// pauses claims; requesting a purpose can only narrow the enrolled permissions.
func requestedRunnerPurposes(r *http.Request, enrolled []string) ([]string, error) {
	var in *struct {
		Purposes json.RawMessage `json:"purposes"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if in == nil {
		return nil, fail(400, "invalid_runner_purposes", "A runner claim request must be an object")
	}
	if len(in.Purposes) == 0 {
		return enrolled, nil
	}
	if bytes.Equal(bytes.TrimSpace(in.Purposes), []byte("null")) {
		return nil, fail(400, "invalid_runner_purposes", "purposes must be an array of runner job purposes")
	}
	var requested []string
	if err := json.Unmarshal(in.Purposes, &requested); err != nil {
		return nil, fail(400, "invalid_runner_purposes", "purposes must be an array of runner job purposes")
	}
	if len(requested) > 4 {
		return nil, fail(400, "invalid_runner_purposes", "At most four runner job purposes may be requested")
	}
	allowed := make(map[string]bool, len(enrolled))
	for _, purpose := range enrolled {
		allowed[purpose] = true
	}
	seen := map[string]bool{}
	filtered := []string{}
	for _, purpose := range requested {
		switch purpose {
		case "preflight", "artifact_prepare", "submission", "confirmation":
		default:
			return nil, fail(400, "invalid_runner_purposes", "Unknown runner job purpose")
		}
		if seen[purpose] {
			return nil, fail(400, "invalid_runner_purposes", "Runner job purposes must be unique")
		}
		seen[purpose] = true
		if allowed[purpose] {
			filtered = append(filtered, purpose)
		}
	}
	return filtered, nil
}

// Renewal grants continued permission to use an unchanged approved host. It is
// not a new advisory collection, physical inventory or isolation assessment.
// The template comes exclusively from operator enrollment, never the request.
func (s *Server) renewRunnerAuthorization(w http.ResponseWriter, r *http.Request, identity runnerIdentity) error {
	var in struct {
		ConfigDigest string `json:"configDigest"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if !protocol.ValidDigest(in.ConfigDigest) {
		return fail(400, "invalid_config_digest", "The exact enrolled configuration digest is required")
	}
	if err := s.verifyHostDelegation(identity, time.Now().UTC()); err != nil {
		return err
	}
	key, err := s.signer()
	if err != nil {
		return fail(503, "signing_unavailable", "Host authorization signer unavailable")
	}
	ctx := r.Context()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var templateJSON []byte
	var group, profile string
	// Lock enrollment through signing/commit so a concurrent revocation cannot
	// interleave with issuance. Existing issued leases do not override host revocation
	// at the authenticated claim endpoint.
	err = tx.QueryRow(ctx, `SELECT e.template,h.host_group,h.execution_profile_digest
		FROM runner_authorization_enrollments e JOIN runner_hosts h ON h.id=e.host_id
		WHERE e.host_id=$1 AND e.config_digest=$2 AND e.enabled AND h.enabled
		FOR SHARE OF e,h`, identity.ID, in.ConfigDigest).Scan(&templateJSON, &group, &profile)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(403, "runner_authorization_unenrolled", "This exact host configuration has no active renewal enrollment")
	}
	if err != nil {
		return err
	}
	var attestation runner.HostAttestation
	if err := protocol.DecodeStrict(templateJSON, &attestation); err != nil || attestation.HostID != identity.ID || attestation.HostGroup != group || attestation.ExecutionProfileDigest != profile || !protocol.ValidDigest(profile) || attestation.ConfigDigest != in.ConfigDigest || attestation.RunnerEpoch != "1" || attestation.PhysicalHostID == "" || !attestation.ExclusivePhysicalHost || !attestation.EgressPolicyVerified {
		return fail(403, "runner_authorization_mismatch", "The approved host template does not match the active enrolled execution profile")
	}
	issuedAt := time.Now().UTC()
	attestation.ExpiresAt = issuedAt.Add(runnerAuthorizationLifetime)
	envelope, err := protocol.Sign(s.Config.ReceiptKeyID, key, attestation)
	if err != nil {
		return err
	}
	digest, err := protocol.Digest(envelope)
	if err != nil {
		return err
	}
	id := ID()
	if _, err = tx.Exec(ctx, `INSERT INTO runner_authorization_renewals(id,host_id,config_digest,issued_at,expires_at,envelope,digest) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, identity.ID, in.ConfigDigest, issuedAt, attestation.ExpiresAt, raw(envelope), digest); err != nil {
		return err
	}
	if err := audit(ctx, tx, "", "runner.authorization_renewed", map[string]any{"renewalId": id, "hostId": identity.ID, "configDigest": in.ConfigDigest, "envelopeDigest": digest, "issuedAt": issuedAt, "expiresAt": attestation.ExpiresAt, "scope": "unchanged enrolled host authorization; no new advisory or hardware assessment"}); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	respond(w, 200, map[string]any{"attestation": envelope})
	return nil
}
