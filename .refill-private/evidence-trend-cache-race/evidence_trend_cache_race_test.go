package evidence_trend_cache_race_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

type barrierRepository struct {
	cases    map[string]domain.ConservationCase
	arrived  chan struct{}
	release  chan struct{}
	evidence map[string][]domain.EvidenceRevision
}

func (r *barrierRepository) Create(context.Context, domain.ConservationCase, string, application.Mutation) (domain.ConservationCase, bool, error) {
	return domain.ConservationCase{}, false, errors.New("unused")
}

func (r *barrierRepository) Mutate(context.Context, string, int64, string, application.Mutation) (domain.ConservationCase, bool, error) {
	return domain.ConservationCase{}, false, errors.New("unused")
}

func (r *barrierRepository) Get(_ context.Context, id string) (domain.ConservationCase, error) {
	c, ok := r.cases[id]
	if !ok {
		return domain.ConservationCase{}, domain.ErrNotFound
	}
	return c, nil
}

func (r *barrierRepository) List(context.Context) ([]domain.ConservationCase, error) {
	return nil, errors.New("unused")
}

func (r *barrierRepository) Audit(context.Context, string) ([]domain.AuditEvent, error) {
	return nil, errors.New("unused")
}

func (r *barrierRepository) EvidenceRevisions(_ context.Context, caseID, _ string) ([]domain.EvidenceRevision, error) {
	r.arrived <- struct{}{}
	<-r.release
	return append([]domain.EvidenceRevision(nil), r.evidence[caseID]...), nil
}

func (r *barrierRepository) FindCredential(context.Context, string) (domain.SafetyCredential, domain.FrozenManifest, error) {
	return domain.SafetyCredential{}, domain.FrozenManifest{}, errors.New("unused")
}

func (r *barrierRepository) Close() error { return nil }

func TestEvidenceTrendCacheSerializesConcurrentQueries(t *testing.T) {
	repo := &barrierRepository{
		cases: map[string]domain.ConservationCase{
			"case-a": {ID: "case-a", Version: 3},
			"case-b": {ID: "case-b", Version: 7},
		},
		arrived: make(chan struct{}, 2),
		release: make(chan struct{}),
		evidence: map[string][]domain.EvidenceRevision{
			"case-a": {{ID: "evidence-a", CaseID: "case-a", ZoneCode: "Z1", Revision: 1, RecordedAt: time.Unix(1, 0).UTC()}},
			"case-b": {{ID: "evidence-b", CaseID: "case-b", ZoneCode: "Z1", Revision: 1, RecordedAt: time.Unix(2, 0).UTC()}},
		},
	}
	service := application.NewService(repo)
	errorsByQuery := make(chan error, 2)
	for _, caseID := range []string{"case-a", "case-b"} {
		caseID := caseID
		go func() {
			trends, err := service.EvidenceTrends(context.Background(), caseID, "Z1")
			if err == nil && (len(trends) != 1 || trends[0].ZoneCode != "Z1") {
				err = errors.New("unexpected trend result")
			}
			errorsByQuery <- err
		}()
	}

	<-repo.arrived
	<-repo.arrived
	close(repo.release)
	for range 2 {
		if err := <-errorsByQuery; err != nil {
			t.Fatal(err)
		}
	}
}
