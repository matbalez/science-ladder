package platform

import (
	"context"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSecurityMigrationRequiresSignedEvidenceAndDrainedPredecessor(t *testing.T) {
	s := testDB(t)
	u, prior := seed(t, s)
	ctx := context.Background()
	key, _ := testKey(t)
	s.ReceiptSigner = key
	s.Config.ReceiptKeyID = "migration-platform"
	var challenge string
	if err := s.DB.QueryRow(ctx, `SELECT challenge_id FROM challenge_versions WHERE id=$1`, prior).Scan(&challenge); err != nil {
		t.Fatal(err)
	}
	m := protocol.Manifest{APIVersion: protocol.APIVersion, Kind: "Challenge", Deadline: time.Now().Add(time.Hour), Milestones: []protocol.Milestone{{ID: "m1", Title: "Transferred", ThresholdTicks: "11"}, {ID: "m3", Title: "New", ThresholdTicks: "30"}}}
	successor := ID()
	commands := []struct {
		sql  string
		args []any
	}{
		{`UPDATE challenge_versions SET intake_status='paused' WHERE id=$1`, []any{prior}},
		{`INSERT INTO challenge_versions(id,challenge_id,repository,repository_id,source_commit,source_digest,manifest,status,intake_status,deadline,lock_digest,predecessor_id,transition_kind,prior_frontier_digest) VALUES($1,$2,'test/repo',1,$3,'next-source',$4,'locked','unavailable',$5,'next-lock',$6,'security_migration','')`, []any{successor, challenge, strings.Repeat("b", 40), raw(m), m.Deadline, prior}},
		{`INSERT INTO milestone_tiers(id,version_id,title,threshold_ticks) VALUES('m3',$1,'New',30)`, []any{successor}},
		{`INSERT INTO preflights(id,version_id,status,machine_receipt_digest) VALUES($1,$2,'pass','conformance')`, []any{ID(), successor}},
	}
	for _, c := range commands {
		if _, err := s.DB.Exec(ctx, c.sql, c.args...); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/publish", nil)
	receipt, err := s.stageMigration(r, tx, u, successor, m)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.completeMigration(ctx, successor); err == nil {
		t.Fatal("unsigned migration opened intake")
	}
	if err = s.signReceipt(ctx, receipt); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(ctx, `UPDATE challenge_versions SET next_sequence=1 WHERE id=$1`, prior); err != nil {
		t.Fatal(err)
	}
	if err = s.completeMigration(ctx, successor); err == nil {
		t.Fatal("migration skipped unresolved predecessor receipt")
	}
	var count int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM milestone_version_mappings WHERE version_id=$1 AND milestone_id='m1'`, successor).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed migration partially transferred milestone")
	}
	if _, err = s.DB.Exec(ctx, `UPDATE challenge_versions SET watermark=1 WHERE id=$1`, prior); err != nil {
		t.Fatal(err)
	}
	if err = s.completeMigration(ctx, successor); err == nil {
		t.Fatal("changed drained watermark no longer matches signed evidence")
	}
	// Restore the immutable snapshot signed into this synthetic migration receipt.
	if _, err = s.DB.Exec(ctx, `UPDATE challenge_versions SET next_sequence=0,watermark=0 WHERE id=$1`, prior); err != nil {
		t.Fatal(err)
	}
	if err = s.completeMigration(ctx, successor); err != nil {
		t.Fatal(err)
	}
	if err = s.completeMigration(ctx, successor); err != nil {
		t.Fatal("retry was not idempotent", err)
	}
	var oldStatus, newIntake string
	if err = s.DB.QueryRow(ctx, `SELECT status FROM challenge_versions WHERE id=$1`, prior).Scan(&oldStatus); err != nil {
		t.Fatal(err)
	}
	if err = s.DB.QueryRow(ctx, `SELECT intake_status FROM challenge_versions WHERE id=$1`, successor).Scan(&newIntake); err != nil {
		t.Fatal(err)
	}
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM milestone_version_mappings WHERE version_id=$1`, successor).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if oldStatus != "superseded" || newIntake != "open" || count != 2 {
		t.Fatalf("non-atomic migration %s %s %d", oldStatus, newIntake, count)
	}
}

func TestSecurityMigrationFixtureBindsExactPublicArtifact(t *testing.T) {
	contract := protocol.SubmissionContract{AllowedPaths: []string{"certificate.json"}, AllowedExtensions: []string{".json"}, MaxBytes: 1024, MaxFiles: 1, License: "MIT"}
	files := map[string][]byte{"certificate.json": []byte(`{"x":1}`)}
	_, digest, err := protocol.ArtifactFromFiles(files, contract)
	if err != nil {
		t.Fatal(err)
	}
	manifest := protocol.Manifest{Submission: contract, Fixtures: []protocol.Fixture{{Name: "previous_frontier", Path: "fixtures/frontier", ExpectedOutcome: "valid", ExpectedTicks: "1"}}}
	source := map[string][]byte{"fixtures/frontier/certificate.json": files["certificate.json"]}
	if err = verifyFrontierFixture(source, manifest, digest); err != nil {
		t.Fatal(err)
	}
	source["fixtures/frontier/certificate.json"] = []byte(`{"x":2}`)
	if err = verifyFrontierFixture(source, manifest, digest); err == nil {
		t.Fatal("creator substituted different frontier fixture")
	}
}
