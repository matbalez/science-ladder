CREATE TABLE preparation_budgets(owner_id uuid NOT NULL REFERENCES users(id),day date NOT NULL DEFAULT current_date,used integer NOT NULL CHECK(used>=0),PRIMARY KEY(owner_id,day));
CREATE UNIQUE INDEX submission_snapshot_intent_dedup ON submission_intents(owner_id,version_id,repository,ref);
