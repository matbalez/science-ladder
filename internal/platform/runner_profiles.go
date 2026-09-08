package platform

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"time"
)

// The authenticated certificate establishes the host; a requested profile only
// selects a separately commissioned, enabled configuration on that same host.
func (s *Server) selectRunnerProfile(ctx context.Context, host runnerIdentity, profile string) (runnerIdentity, error) {
	if profile == "" || profile == host.ExecutionProfile {
		return host, nil
	}
	if !protocol.ValidDigest(profile) {
		return host, fail(400, "runner_profile_invalid", "A pinned execution profile is required")
	}
	var capabilities, template []byte
	err := s.DB.QueryRow(ctx, `SELECT p.capabilities,p.advisory_snapshot_digest,p.runtime_inventory_digest,e.template
 FROM runner_profiles p JOIN runner_authorization_enrollments e ON e.host_id=p.host_id AND e.config_digest=p.config_digest
 WHERE p.host_id=$1 AND p.execution_profile_digest=$2 AND p.enabled AND e.enabled`, host.ID, profile).Scan(&capabilities, &host.AdvisorySnapshotDigest, &host.RuntimeInventoryDigest, &template)
	if err != nil {
		return host, fail(403, "runner_profile_unenrolled", "This host has no active enrollment for the requested profile")
	}
	var a struct {
		ExecutionProfileDigest string                         `json:"executionProfileDigest"`
		Capabilities           *protocol.ExecutorCapabilities `json:"capabilities"`
	}
	var c protocol.ExecutorCapabilities
	if json.Unmarshal(template, &a) != nil || json.Unmarshal(capabilities, &c) != nil || a.ExecutionProfileDigest != profile || a.Capabilities == nil {
		return host, fail(403, "runner_profile_mismatch", "The profile does not match its approved host template")
	}
	one, _ := protocol.Digest(a.Capabilities)
	two, _ := protocol.Digest(c)
	if one != two {
		return host, fail(403, "runner_profile_mismatch", "Capabilities differ from the approved template")
	}
	host.ExecutionProfile = profile
	host.Capabilities = &c
	return host, nil
}

// Run before reserving creator/submission capacity. Missing GPU/Metal hardware
// is an explicit admission error, never a queued job that silently falls back.
func (s *Server) requireExecutor(ctx context.Context, tx pgx.Tx, m protocol.Manifest, lockedProfile string) error {
	if m.Evaluation == nil {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT h.id,h.host_group,h.public_key,p.execution_profile_digest,p.capabilities,e.template
 FROM runner_profiles p JOIN runner_hosts h ON h.id=p.host_id
 JOIN runner_authorization_enrollments e ON e.host_id=p.host_id AND e.config_digest=p.config_digest
 WHERE p.enabled AND h.enabled AND e.enabled AND ($1='' OR p.execution_profile_digest=$1)
 AND 'submission'=ANY(h.purposes) AND 'confirmation'=ANY(h.purposes) AND ($2 OR h.encryption_public_key<>'')`, lockedProfile, m.Suite.Visibility != "hidden")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var h runnerIdentity
		var cb, tb []byte
		if err := rows.Scan(&h.ID, &h.Group, &h.PublicKey, &h.ExecutionProfile, &cb, &tb); err != nil {
			return err
		}
		var c protocol.ExecutorCapabilities
		var a struct {
			ExecutionProfileDigest string                         `json:"executionProfileDigest"`
			Capabilities           *protocol.ExecutorCapabilities `json:"capabilities"`
		}
		if json.Unmarshal(cb, &c) != nil || json.Unmarshal(tb, &a) != nil || a.Capabilities == nil || a.ExecutionProfileDigest != h.ExecutionProfile {
			continue
		}
		cd, _ := protocol.Digest(c)
		ad, _ := protocol.Digest(a.Capabilities)
		if cd == ad && protocol.MatchExecutor(*m.Evaluation, m.Resources, m.Validator.RuntimeImageDigest, c) == nil && s.verifyHostDelegation(h, time.Now()) == nil {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return fail(503, "executor_unavailable", "No commissioned executor matches this challenge's operating system, hardware, runtime and resource requirements")
}

func nullableCapabilities(c *protocol.ExecutorCapabilities) any {
	if c == nil {
		return nil
	}
	return raw(c)
}
