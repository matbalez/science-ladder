-- Operator-managed approvals; creators cannot choose the vulnerability policy.
ALTER TABLE runner_hosts ADD COLUMN advisory_snapshot_digest text NOT NULL DEFAULT '';
ALTER TABLE runner_hosts ADD COLUMN runtime_inventory_digest text NOT NULL DEFAULT '';
