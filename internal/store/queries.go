package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

func (r *SQLiteRepository) Get(ctx context.Context, id string) (domain.ConservationCase, error) {
	var data []byte
	err := r.db.QueryRowContext(ctx, `SELECT aggregate_json FROM cases WHERE id=?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ConservationCase{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ConservationCase{}, fmt.Errorf("读取处置案: %w", err)
	}
	return unmarshalCase(data)
}

func (r *SQLiteRepository) List(ctx context.Context) ([]domain.ConservationCase, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT aggregate_json FROM cases ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("查询处置案: %w", err)
	}
	defer rows.Close()
	result := make([]domain.ConservationCase, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		c, err := unmarshalCase(data)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *SQLiteRepository) Audit(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT sequence,event_type,actor,summary,occurred_at,case_version,previous_hash,event_hash,details_json FROM audit_events WHERE case_id=? ORDER BY sequence`, caseID)
	if err != nil {
		return nil, fmt.Errorf("查询审计时间线: %w", err)
	}
	defer rows.Close()
	result := make([]domain.AuditEvent, 0)
	for rows.Next() {
		var event domain.AuditEvent
		var occurred, details string
		event.CaseID = caseID
		if err := rows.Scan(&event.Sequence, &event.EventType, &event.Actor, &event.Summary, &occurred, &event.CaseVersion, &event.PreviousHash, &event.EventHash, &details); err != nil {
			return nil, err
		}
		event.OccurredAt, _ = time.Parse(time.RFC3339Nano, occurred)
		if err := json.Unmarshal([]byte(details), &event.Details); err != nil {
			return nil, fmt.Errorf("解码审计详情: %w", err)
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		if _, err := r.Get(ctx, caseID); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *SQLiteRepository) EvidenceRevisions(ctx context.Context, caseID, zoneCode string) ([]domain.EvidenceRevision, error) {
	query := `SELECT evidence_json FROM evidence_revisions WHERE case_id=? ORDER BY zone_code,revision`
	args := []any{caseID}
	if zoneCode != "" {
		query = `SELECT evidence_json FROM evidence_revisions WHERE case_id=? AND zone_code=? ORDER BY revision`
		args = append(args, zoneCode)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询证据修订: %w", err)
	}
	defer rows.Close()
	result := make([]domain.EvidenceRevision, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var item domain.EvidenceRevision
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("解码证据修订: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if zoneCode != "" && len(result) == 0 {
		if _, err := r.Get(ctx, caseID); err != nil {
			return nil, err
		}
		return nil, domain.ErrNotFound
	}
	return result, nil
}

func (r *SQLiteRepository) FindCredential(ctx context.Context, number string) (domain.SafetyCredential, domain.FrozenManifest, error) {
	var credentialData, manifestData []byte
	err := r.db.QueryRowContext(ctx, `SELECT c.credential_json,m.manifest_json FROM credentials c JOIN frozen_manifests m ON m.case_id=c.case_id WHERE c.credential_no=?`, number).Scan(&credentialData, &manifestData)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SafetyCredential{}, domain.FrozenManifest{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.SafetyCredential{}, domain.FrozenManifest{}, fmt.Errorf("读取凭据: %w", err)
	}
	credential, err := unmarshalCredential(credentialData)
	if err != nil {
		return credential, domain.FrozenManifest{}, err
	}
	manifest, err := unmarshalManifest(manifestData)
	return credential, manifest, err
}
