-- Protocol ParseTicks accepts at most 160 characters. PostgreSQL must preserve
-- every protocol-valid integer rather than fail after an official run completes.
ALTER TABLE submissions ALTER COLUMN score_ticks TYPE numeric(160,0);
ALTER TABLE milestone_tiers ALTER COLUMN threshold_ticks TYPE numeric(160,0);
ALTER TABLE milestone_version_mappings ALTER COLUMN threshold_ticks TYPE numeric(160,0);
