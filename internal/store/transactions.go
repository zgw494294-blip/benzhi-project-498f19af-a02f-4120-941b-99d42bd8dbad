package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

func (r *SQLiteRepository) Create(ctx context.Context, c domain.ConservationCase, key string, mutation application.Mutation) (domain.ConservationCase, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return c, false, fmt.Errorf("开始建档事务: %w", err)
	}
	defer tx.Rollback()
	if cached, ok, err := cachedResult(ctx, tx, key); err != nil {
		return c, false, err
	} else if ok {
		return cached, true, nil
	}
	payload, err := marshal(c)
	if err != nil {
		return c, false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cases(id,cave_code,mural_zone,status,version,aggregate_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		c.ID, c.CaveCode, c.MuralZone, c.Status, c.Version, payload, c.CreatedAt.Format(time.RFC3339Nano), c.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return c, false, fmt.Errorf("保存处置案: %w", err)
	}
	event := domain.AuditEvent{CaseID: c.ID, Sequence: 1, EventType: mutation.EventType, Actor: mutation.Actor, Summary: mutation.Summary, OccurredAt: c.CreatedAt, CaseVersion: c.Version, Details: mutation.Details}
	event.EventHash = domain.AuditHash("", event)
	if err := insertAudit(ctx, tx, event); err != nil {
		return c, false, err
	}
	if err := insertIdempotency(ctx, tx, key, c.ID, mutation.EventType, payload, c.CreatedAt); err != nil {
		return c, false, err
	}
	if err := tx.Commit(); err != nil {
		return c, false, fmt.Errorf("提交建档事务: %w", err)
	}
	return c, false, nil
}

func (r *SQLiteRepository) Mutate(ctx context.Context, id string, expectedVersion int64, key string, mutation application.Mutation) (domain.ConservationCase, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ConservationCase{}, false, fmt.Errorf("开始更新事务: %w", err)
	}
	defer tx.Rollback()
	if cached, ok, err := cachedResult(ctx, tx, key); err != nil {
		return domain.ConservationCase{}, false, err
	} else if ok {
		if cached.ID != id {
			return domain.ConservationCase{}, false, domain.ErrConflict
		}
		return cached, true, nil
	}
	var data []byte
	var storedVersion int64
	err = tx.QueryRowContext(ctx, `SELECT aggregate_json,version FROM cases WHERE id=?`, id).Scan(&data, &storedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ConservationCase{}, false, domain.ErrNotFound
	}
	if err != nil {
		return domain.ConservationCase{}, false, fmt.Errorf("读取处置案: %w", err)
	}
	if storedVersion != expectedVersion {
		return domain.ConservationCase{}, false, domain.ErrConflict
	}
	c, err := unmarshalCase(data)
	if err != nil {
		return c, false, err
	}
	if c.FrozenManifest != nil && (mutation.EventType == "evidence_revised" || mutation.EventType == "plan_submitted" || mutation.EventType == "pilot_trial_recorded") {
		return c, false, domain.ErrAlreadyFrozen
	}
	if mutation.Apply == nil {
		return c, false, errors.New("缺少事务变更函数")
	}
	if err := mutation.Apply(&c); err != nil {
		return c, false, err
	}
	c.Version = storedVersion + 1
	c.UpdatedAt = time.Now().UTC()
	payload, err := marshal(c)
	if err != nil {
		return c, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE cases SET status=?,version=?,aggregate_json=?,updated_at=? WHERE id=? AND version=?`, c.Status, c.Version, payload, c.UpdatedAt.Format(time.RFC3339Nano), id, expectedVersion)
	if err != nil {
		return c, false, fmt.Errorf("条件更新处置案: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return c, false, err
	}
	if affected != 1 {
		return c, false, domain.ErrConflict
	}
	if err := r.syncImmutableParts(ctx, tx, c); err != nil {
		return c, false, err
	}
	sequence, previous, err := nextAuditSequence(ctx, tx, id)
	if err != nil {
		return c, false, err
	}
	event := domain.AuditEvent{CaseID: id, Sequence: sequence, EventType: mutation.EventType, Actor: mutation.Actor, Summary: mutation.Summary, OccurredAt: c.UpdatedAt, CaseVersion: c.Version, PreviousHash: previous, Details: mutation.Details}
	event.EventHash = domain.AuditHash(previous, event)
	if err := insertAudit(ctx, tx, event); err != nil {
		return c, false, err
	}
	if err := insertIdempotency(ctx, tx, key, id, mutation.EventType, payload, c.UpdatedAt); err != nil {
		return c, false, err
	}
	if err := tx.Commit(); err != nil {
		return c, false, fmt.Errorf("提交更新事务: %w", err)
	}
	return c, false, nil
}

func cachedResult(ctx context.Context, tx *sql.Tx, key string) (domain.ConservationCase, bool, error) {
	var data []byte
	err := tx.QueryRowContext(ctx, `SELECT response_json FROM idempotency_results WHERE idempotency_key=?`, key).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ConservationCase{}, false, nil
	}
	if err != nil {
		return domain.ConservationCase{}, false, fmt.Errorf("读取幂等结果: %w", err)
	}
	c, err := unmarshalCase(data)
	return c, err == nil, err
}

func insertIdempotency(ctx context.Context, tx *sql.Tx, key, caseID, operation string, payload []byte, at time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO idempotency_results(idempotency_key,case_id,operation,response_json,created_at) VALUES(?,?,?,?,?)`, key, caseID, operation, payload, at.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("保存幂等结果: %w", err)
	}
	return nil
}

func nextAuditSequence(ctx context.Context, tx *sql.Tx, caseID string) (int64, string, error) {
	var sequence int64
	var hash string
	err := tx.QueryRowContext(ctx, `SELECT sequence,event_hash FROM audit_events WHERE case_id=? ORDER BY sequence DESC LIMIT 1`, caseID).Scan(&sequence, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return 1, "", nil
	}
	if err != nil {
		return 0, "", fmt.Errorf("读取审计序列: %w", err)
	}
	return sequence + 1, hash, nil
}

func insertAudit(ctx context.Context, tx *sql.Tx, event domain.AuditEvent) error {
	details, err := json.Marshal(event.Details)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(case_id,sequence,event_type,actor,summary,occurred_at,case_version,previous_hash,event_hash,details_json) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		event.CaseID, event.Sequence, event.EventType, event.Actor, event.Summary, event.OccurredAt.Format(time.RFC3339Nano), event.CaseVersion, event.PreviousHash, event.EventHash, details)
	if err != nil {
		return fmt.Errorf("保存审计事件: %w", err)
	}
	return nil
}
