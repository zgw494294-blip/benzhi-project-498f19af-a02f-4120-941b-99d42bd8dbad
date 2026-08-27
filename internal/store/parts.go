package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

func syncImmutableParts(ctx context.Context, tx *sql.Tx, c domain.ConservationCase) error {
	for _, item := range c.Evidence {
		data, err := marshal(item)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO evidence_revisions(id,case_id,zone_code,revision,evidence_json,recorded_at) VALUES(?,?,?,?,?,?)`, item.ID, c.ID, item.ZoneCode, item.Revision, data, item.RecordedAt.Format(time.RFC3339Nano))
		if err != nil {
			return fmt.Errorf("保存证据修订: %w", err)
		}
	}
	for _, plan := range c.Plans {
		data, err := marshal(plan)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO treatment_plans(id,case_id,version,plan_json,submitted_at) VALUES(?,?,?,?,?)`, plan.ID, c.ID, plan.Version, data, plan.SubmittedAt.Format(time.RFC3339Nano))
		if err != nil {
			return fmt.Errorf("保存方案修订: %w", err)
		}
	}
	for _, trial := range c.Trials {
		data, err := marshal(trial)
		if err != nil {
			return err
		}
		createdAt := trial.StartedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO pilot_trials(id,case_id,revision,trial_json,created_at) VALUES(?,?,?,?,?)`, trial.ID, c.ID, trial.Revision, data, createdAt.Format(time.RFC3339Nano))
		if err != nil {
			return fmt.Errorf("保存试验修订: %w", err)
		}
	}
	if c.FrozenManifest != nil {
		data, err := marshal(c.FrozenManifest)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO frozen_manifests(case_id,digest,manifest_json,frozen_at) VALUES(?,?,?,?)`, c.ID, c.FrozenManifest.ManifestDigest, data, c.FrozenManifest.FrozenAt.Format(time.RFC3339Nano))
		if err != nil {
			return fmt.Errorf("保存冻结清单: %w", err)
		}
	}
	if c.Credential != nil {
		data, err := marshal(c.Credential)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO credentials(credential_no,case_id,credential_json,issued_at) VALUES(?,?,?,?) ON CONFLICT(credential_no) DO UPDATE SET credential_json=excluded.credential_json WHERE credentials.case_id=excluded.case_id`, c.Credential.CredentialNo, c.ID, data, c.Credential.IssuedAt.Format(time.RFC3339Nano))
		if err != nil {
			return fmt.Errorf("保存安全凭据: %w", err)
		}
	}
	return nil
}
