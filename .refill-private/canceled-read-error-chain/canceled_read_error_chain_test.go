package canceled_read_error_chain

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

type canceledRepository struct {
	allowGet bool
}

func canceledRead(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("SQLite 读取已中断: %w", err)
	}
	return errors.New("测试仓储只应在 context 取消后调用")
}

func (r canceledRepository) Create(ctx context.Context, c domain.ConservationCase, _ string, _ application.Mutation) (domain.ConservationCase, bool, error) {
	return c, false, canceledRead(ctx)
}

func (r canceledRepository) Mutate(ctx context.Context, _ string, _ int64, _ string, _ application.Mutation) (domain.ConservationCase, bool, error) {
	return domain.ConservationCase{}, false, canceledRead(ctx)
}

func (r canceledRepository) Get(ctx context.Context, id string) (domain.ConservationCase, error) {
	if r.allowGet {
		return domain.ConservationCase{ID: id}, nil
	}
	return domain.ConservationCase{}, canceledRead(ctx)
}

func (r canceledRepository) List(ctx context.Context) ([]domain.ConservationCase, error) {
	return nil, canceledRead(ctx)
}

func (r canceledRepository) Audit(ctx context.Context, _ string) ([]domain.AuditEvent, error) {
	return nil, canceledRead(ctx)
}

func (r canceledRepository) EvidenceRevisions(ctx context.Context, _, _ string) ([]domain.EvidenceRevision, error) {
	return nil, canceledRead(ctx)
}

func (r canceledRepository) FindCredential(ctx context.Context, _ string) (domain.SafetyCredential, domain.FrozenManifest, error) {
	return domain.SafetyCredential{}, domain.FrozenManifest{}, canceledRead(ctx)
}

func (canceledRepository) Close() error { return nil }

func TestCanceledReadPreservesErrorChain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	service := application.NewService(canceledRepository{})
	evidenceService := application.NewService(canceledRepository{allowGet: true})
	tests := []struct {
		name string
		call func() error
	}{
		{name: "GetCase", call: func() error { _, err := service.GetCase(ctx, "case-1"); return err }},
		{name: "ListCases", call: func() error { _, err := service.ListCases(ctx); return err }},
		{name: "AuditTimeline", call: func() error { _, err := service.AuditTimeline(ctx, "case-1"); return err }},
		{name: "EvidenceTrends-case", call: func() error { _, err := service.EvidenceTrends(ctx, "case-1", "Z1"); return err }},
		{name: "EvidenceTrends-revisions", call: func() error { _, err := evidenceService.EvidenceTrends(ctx, "case-1", "Z1"); return err }},
		{name: "VerifyCredential", call: func() error { _, err := service.VerifyCredential(ctx, "MURAL-1"); return err }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("%s 丢失 context.Canceled 错误链：%v", test.name, err)
			}
		})
	}
}
