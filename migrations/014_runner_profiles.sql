-- Multiple immutable runtime profiles may be commissioned on one physical host.
-- This does not invent independent replicas or grant hardware by self-report.
CREATE TABLE runner_profiles (
 host_id text NOT NULL REFERENCES runner_hosts(id),
 execution_profile_digest text NOT NULL CHECK(execution_profile_digest ~ '^sha256:[0-9a-f]{64}$'),
 config_digest text NOT NULL,
 capabilities jsonb NOT NULL CHECK(jsonb_typeof(capabilities)='object'),
 advisory_snapshot_digest text NOT NULL CHECK(advisory_snapshot_digest ~ '^sha256:[0-9a-f]{64}$'),
 runtime_inventory_digest text NOT NULL CHECK(runtime_inventory_digest ~ '^sha256:[0-9a-f]{64}$'),
 enabled boolean NOT NULL DEFAULT false,
 PRIMARY KEY(host_id,execution_profile_digest),
 UNIQUE(host_id,config_digest),
 FOREIGN KEY(host_id,config_digest) REFERENCES runner_authorization_enrollments(host_id,config_digest)
);
CREATE FUNCTION protect_runner_profile() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' OR (to_jsonb(NEW)-'enabled') IS DISTINCT FROM (to_jsonb(OLD)-'enabled') THEN
  RAISE EXCEPTION 'Approved runner profile is immutable; disable it and enroll a new profile';
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER immutable_runner_profile BEFORE UPDATE OR DELETE ON runner_profiles FOR EACH ROW EXECUTE FUNCTION protect_runner_profile();
