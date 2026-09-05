-- Operator enrollment permits automatic authorization of this exact host config.
-- The template is existing commissioned evidence, not a fresh security finding.
CREATE TABLE runner_authorization_enrollments (
 host_id text NOT NULL REFERENCES runner_hosts(id),
 config_digest text NOT NULL CHECK(config_digest ~ '^sha256:[0-9a-f]{64}$'),
 template jsonb NOT NULL CHECK(jsonb_typeof(template)='object'),
 enabled boolean NOT NULL DEFAULT false,
 approval_reason text NOT NULL CHECK(length(approval_reason) BETWEEN 20 AND 2000),
 approved_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(host_id,config_digest),
 CHECK(template->>'hostId'=host_id AND template->>'configDigest'=config_digest)
);
CREATE FUNCTION protect_runner_authorization_enrollment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' OR (to_jsonb(NEW)-'enabled') IS DISTINCT FROM (to_jsonb(OLD)-'enabled') THEN
  RAISE EXCEPTION 'Approved runner authorization template is immutable; disable it and enroll a new config';
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER immutable_runner_authorization_enrollment BEFORE UPDATE OR DELETE ON runner_authorization_enrollments FOR EACH ROW EXECUTE FUNCTION protect_runner_authorization_enrollment();

CREATE TABLE runner_authorization_renewals (
 id uuid PRIMARY KEY,
 host_id text NOT NULL,
 config_digest text NOT NULL,
 issued_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL CHECK(expires_at>issued_at AND expires_at<=issued_at+interval '24 hours'),
 envelope jsonb NOT NULL,
 digest text NOT NULL UNIQUE CHECK(digest ~ '^sha256:[0-9a-f]{64}$'),
 FOREIGN KEY(host_id,config_digest) REFERENCES runner_authorization_enrollments(host_id,config_digest)
);
CREATE INDEX runner_authorization_recent ON runner_authorization_renewals(host_id,issued_at DESC);
CREATE TRIGGER immutable_runner_authorization_renewal BEFORE UPDATE OR DELETE ON runner_authorization_renewals FOR EACH ROW EXECUTE FUNCTION deny_audit_mutation();
