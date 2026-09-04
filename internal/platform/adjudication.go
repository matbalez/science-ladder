package platform

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"math/big"
	"time"
)

func better(a, b, direction string) bool {
	x, ok := new(big.Int).SetString(a, 10)
	if !ok {
		return false
	}
	y, ok := new(big.Int).SetString(b, 10)
	if !ok {
		return false
	}
	if direction == "maximize" {
		return x.Cmp(y) > 0
	}
	return x.Cmp(y) < 0
}
func crosses(score, threshold, direction string) bool {
	x, ok := new(big.Int).SetString(score, 10)
	if !ok {
		return false
	}
	y, ok := new(big.Int).SetString(threshold, 10)
	if !ok {
		return false
	}
	if direction == "maximize" {
		return x.Cmp(y) >= 0
	}
	return x.Cmp(y) <= 0
}
func meaningful(score, prior string, m protocol.Metric) bool {
	x, ok := new(big.Int).SetString(score, 10)
	if !ok {
		return false
	}
	y, ok := new(big.Int).SetString(prior, 10)
	if !ok {
		return false
	}
	d, ok := new(big.Int).SetString(m.MinimumDeltaTicks, 10)
	if !ok {
		return false
	}
	x.Sub(x, y)
	if m.Direction == "minimize" {
		x.Neg(x)
	}
	return x.Cmp(d) >= 0
}

// ConfirmedScore rounds disagreements within declared tolerance against the solver.
func ConfirmedScore(a, b string, m protocol.Metric) (string, error) {
	x, ok := new(big.Int).SetString(a, 10)
	if !ok {
		return "", errors.New("invalid primary ticks")
	}
	y, ok := new(big.Int).SetString(b, 10)
	if !ok {
		return "", errors.New("invalid confirmation ticks")
	}
	t, ok := new(big.Int).SetString(m.ToleranceTicks, 10)
	if !ok {
		return "", errors.New("invalid tolerance ticks")
	}
	diff := new(big.Int).Sub(x, y)
	diff.Abs(diff)
	if diff.Cmp(t) > 0 {
		return "", errors.New("independent runs disagree beyond tolerance")
	}
	if better(a, b, m.Direction) {
		return b, nil
	}
	return a, nil
}
func (s *Server) adjudicate(ctx context.Context, version string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var watermark int64
	var manifest []byte
	var bestID, frontierID *string
	err = tx.QueryRow(ctx, `SELECT watermark,manifest,verified_best_id,public_frontier_id FROM challenge_versions WHERE id=$1 FOR UPDATE`, version).Scan(&watermark, &manifest, &bestID, &frontierID)
	if err != nil {
		return err
	}
	var m protocol.Manifest
	if err = json.Unmarshal(manifest, &m); err != nil {
		return err
	}
	best, frontier := m.Metric.BaselineTicks, m.Metric.BaselineTicks
	if bestID != nil {
		if err = tx.QueryRow(ctx, `SELECT score_ticks::text FROM submissions WHERE id=$1`, *bestID).Scan(&best); err != nil {
			return err
		}
	}
	if frontierID != nil {
		if err = tx.QueryRow(ctx, `SELECT score_ticks::text FROM submissions WHERE id=$1`, *frontierID).Scan(&frontier); err != nil {
			return err
		}
	}
	for {
		var id, owner, artifact, outcome, commitment, status string
		var score *string
		var publish bool
		err = tx.QueryRow(ctx, `SELECT id,owner_id,artifact_digest,outcome,score_ticks::text,publish_requested,commitment,status FROM submissions WHERE version_id=$1 AND sequence=$2 FOR UPDATE`, version, watermark+1).Scan(&id, &owner, &artifact, &outcome, &score, &publish, &commitment, &status)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				break
			}
			return err
		}
		if status != "validated" {
			break
		}
		claims := []string{}
		record, advance := false, false
		if outcome == "valid" && score != nil {
			record = better(*score, best, m.Metric.Direction)
			rows, err := tx.Query(ctx, `SELECT m.id,mv.threshold_ticks::text FROM milestone_tiers m JOIN milestone_version_mappings mv ON mv.milestone_id=m.id LEFT JOIN milestone_claims c ON c.milestone_id=m.id WHERE mv.version_id=$1 AND c.id IS NULL ORDER BY m.id`, version)
			if err != nil {
				return err
			}
			for rows.Next() {
				var mid, threshold string
				if err = rows.Scan(&mid, &threshold); err != nil {
					rows.Close()
					return err
				}
				if crosses(*score, threshold, m.Metric.Direction) {
					claims = append(claims, mid)
				}
			}
			rows.Close()
			publish = publish || len(claims) > 0
			advance = publish && meaningful(*score, frontier, m.Metric)
			if record {
				best = *score
				bestID = &id
			}
			if advance {
				frontier = *score
				frontierID = &id
			}
		}
		watermark++
		receipt := protocol.Receipt{DeploymentMode: s.Config.DeploymentMode, OfficialAcceptance: false, APIVersion: protocol.APIVersion, Kind: "AdjudicationReceipt", ID: ID(), CreatedAt: time.Now().UTC(), Producer: "science-ladder", SubjectDigest: artifact, EconomicMode: "none", Data: map[string]any{"versionId": version, "submissionId": id, "sequence": formatInt(watermark), "outcome": outcome, "scoreTicks": score, "verifiedBestAdvanced": record, "publicFrontierAdvanced": advance, "milestoneIds": claims}}
		digest, err := protocol.Digest(receipt)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO receipts(digest,payload,owner_id,public) VALUES($1,$2,$3,$4)`, digest, raw(receipt), owner, publish); err != nil {
			return err
		}
		if err = enqueue(ctx, tx, "sign_receipt", digest); err != nil {
			return err
		}
		for _, mid := range claims {
			claimID := ID()
			if _, err = tx.Exec(ctx, `INSERT INTO milestone_claims(id,milestone_id,submission_id,receipt_digest) VALUES($1,$2,$3,$4)`, claimID, mid, id, digest); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(id,kind,resource_id,payload) VALUES($1,'milestone.claimed.v1',$2,$3)`, ID(), claimID, raw(map[string]any{"versionId": version, "milestoneId": mid, "submissionId": id, "economicMode": "none"})); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE submissions SET status='finalized',public=$2,adjudication_digest=$3 WHERE id=$1`, id, publish, digest); err != nil {
			return err
		}
		if publish {
			if _, err = tx.Exec(ctx, `UPDATE artifacts SET public_at=COALESCE(public_at,now()) WHERE digest=$1 OR digest=(SELECT disk_digest FROM submissions WHERE id=$2)`, artifact, id); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE receipts SET public=true WHERE digest=(SELECT receipt_digest FROM submissions WHERE id=$1)`, id); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE receipts SET public=true WHERE digest IN(SELECT rr.digest FROM runner_results rr JOIN runner_jobs j ON j.id=rr.job_id WHERE j.submission_id=$1)`, id); err != nil {
				return err
			}
		}
		tag, err := tx.Exec(ctx, `UPDATE capacity_reservations SET released_at=now() WHERE submission_id=$1 AND released_at IS NULL`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 1 {
			if _, err = tx.Exec(ctx, `UPDATE capacity SET reserved_units=reserved_units-2 WHERE id=1`); err != nil {
				return err
			}
		}
		event := map[string]any{"sequence": formatInt(watermark), "outcome": outcome, "commitment": commitment}
		if publish {
			event["submissionId"] = id
			event["scoreTicks"] = score
			event["milestones"] = claims
			event["receiptDigest"] = digest
		}
		if err = audit(ctx, tx, version, "submission.adjudicated", event); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE challenge_versions SET watermark=$2,verified_best_id=$3,public_frontier_id=$4 WHERE id=$1`, version, watermark, bestID, frontierID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
