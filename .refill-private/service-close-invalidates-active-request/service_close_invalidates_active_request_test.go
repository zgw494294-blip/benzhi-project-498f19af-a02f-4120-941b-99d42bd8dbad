package service_close_invalidates_active_request_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

var errRepositoryResourceClosed = errors.New("repository resource closed while request was active")

type lifecycleRepository struct {
	operationEntered  chan string
	releaseOperations chan struct{}

	mu     sync.Mutex
	closed bool
}

func newLifecycleRepository() *lifecycleRepository {
	return &lifecycleRepository{
		operationEntered:  make(chan string, 3),
		releaseOperations: make(chan struct{}),
	}
}

func (r *lifecycleRepository) operationError(name string) error {
	r.operationEntered <- name
	<-r.releaseOperations
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return errRepositoryResourceClosed
	}
	return nil
}

func (r *lifecycleRepository) Get(context.Context, string) (domain.ConservationCase, error) {
	if err := r.operationError("GetCase"); err != nil {
		return domain.ConservationCase{}, err
	}
	return domain.ConservationCase{ID: "case-active", Version: 1}, nil
}

func (r *lifecycleRepository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

func (r *lifecycleRepository) Create(context.Context, domain.ConservationCase, string, application.Mutation) (domain.ConservationCase, bool, error) {
	panic("unexpected Create")
}

func (r *lifecycleRepository) Mutate(context.Context, string, int64, string, application.Mutation) (domain.ConservationCase, bool, error) {
	panic("unexpected Mutate")
}

func (r *lifecycleRepository) List(context.Context) ([]domain.ConservationCase, error) {
	if err := r.operationError("ListCases"); err != nil {
		return nil, err
	}
	return []domain.ConservationCase{{ID: "case-active", Version: 1}}, nil
}

func (r *lifecycleRepository) Audit(context.Context, string) ([]domain.AuditEvent, error) {
	if err := r.operationError("AuditTimeline"); err != nil {
		return nil, err
	}
	return []domain.AuditEvent{{CaseID: "case-active", Sequence: 1}}, nil
}

func (r *lifecycleRepository) EvidenceRevisions(context.Context, string, string) ([]domain.EvidenceRevision, error) {
	panic("unexpected EvidenceRevisions")
}

func (r *lifecycleRepository) FindCredential(context.Context, string) (domain.SafetyCredential, domain.FrozenManifest, error) {
	panic("unexpected FindCredential")
}

func TestServiceCloseWaitsForActiveRequests(t *testing.T) {
	repository := newLifecycleRepository()
	service := application.NewService(repository)
	type requestResult struct {
		operation string
		err       error
	}
	requestResults := make(chan requestResult, 3)

	go func() {
		_, err := service.GetCase(context.Background(), "case-active")
		requestResults <- requestResult{operation: "GetCase", err: err}
	}()
	go func() {
		_, err := service.ListCases(context.Background())
		requestResults <- requestResult{operation: "ListCases", err: err}
	}()
	go func() {
		_, err := service.AuditTimeline(context.Background(), "case-active")
		requestResults <- requestResult{operation: "AuditTimeline", err: err}
	}()

	for range 3 {
		<-repository.operationEntered
	}
	if err := service.Close(); err != nil {
		t.Fatalf("close service: %v", err)
	}
	close(repository.releaseOperations)

	for range 3 {
		result := <-requestResults
		if result.err != nil {
			t.Errorf("active %s request lost its repository lease: %v", result.operation, result.err)
		}
	}
}
