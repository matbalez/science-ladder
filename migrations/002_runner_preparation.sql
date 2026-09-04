ALTER TABLE runner_hosts ADD COLUMN purposes text[] NOT NULL DEFAULT '{submission,confirmation}';
ALTER TABLE runner_hosts ADD COLUMN execution_profile_digest text NOT NULL DEFAULT '';
ALTER TABLE runner_jobs ADD COLUMN intent_id uuid REFERENCES submission_intents(id);
CREATE TABLE runner_uploads(job_id uuid NOT NULL REFERENCES runner_jobs(id),role text NOT NULL,digest text NOT NULL,size bigint NOT NULL CHECK(size>0 AND size<=1073741824),verified boolean NOT NULL DEFAULT false,PRIMARY KEY(job_id,role));
