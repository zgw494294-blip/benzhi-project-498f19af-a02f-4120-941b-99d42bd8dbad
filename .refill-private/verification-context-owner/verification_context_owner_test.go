package verification_context_owner_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

type blockingCredentialRepository struct {
	credential domain.SafetyCredential
	manifest   domain.FrozenManifest
	started    chan struct{}
	release    chan struct{}
	startOnce  sync.Once
	calls      atomic.Int32
}

func (r *blockingCredentialRepository) Create(context.Context, domain.ConservationCase, string, application.Mutation) (domain.ConservationCase, bool, error) {
	panic("unexpected Create")
}

func (r *blockingCredentialRepository) Mutate(context.Context, string, int64, string, application.Mutation) (domain.ConservationCase, bool, error) {
	panic("unexpected Mutate")
}

func (r *blockingCredentialRepository) Get(context.Context, string) (domain.ConservationCase, error) {
	panic("unexpected Get")
}

func (r *blockingCredentialRepository) List(context.Context) ([]domain.ConservationCase, error) {
	panic("unexpected List")
}

func (r *blockingCredentialRepository) Audit(context.Context, string) ([]domain.AuditEvent, error) {
	panic("unexpected Audit")
}

func (r *blockingCredentialRepository) EvidenceRevisions(context.Context, string, string) ([]domain.EvidenceRevision, error) {
	panic("unexpected EvidenceRevisions")
}

func (r *blockingCredentialRepository) FindCredential(ctx context.Context, _ string) (domain.SafetyCredential, domain.FrozenManifest, error) {
	r.calls.Add(1)
	r.startOnce.Do(func() { close(r.started) })
	select {
	case <-ctx.Done():
		return domain.SafetyCredential{}, domain.FrozenManifest{}, ctx.Err()
	case <-r.release:
		return r.credential, r.manifest, nil
	}
}

func (r *blockingCredentialRepository) Close() error { return nil }

type doneObservedContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (c *doneObservedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

type verificationOutcome struct {
	result application.VerificationResult
	err    error
}

func TestHealthyFollowerDoesNotInheritLeaderCancellation(t *testing.T) {
	now := time.Date(2026, time.August, 27, 9, 0, 0, 0, time.UTC)
	manifest := domain.FrozenManifest{
		CaseID:         "case-context-owner",
		EvidenceRefs:   map[string]string{"Z-1": "evidence-1"},
		ManifestDigest: "manifest-context-owner",
	}
	frozen := domain.ConservationCase{
		ID:             manifest.CaseID,
		Status:         domain.StatusFrozen,
		FrozenManifest: &manifest,
	}
	credential, err := domain.BuildCredential(
		frozen,
		"MURAL-CONTEXT-OWNER",
		[]string{"Z-1"},
		[]string{"保持隔离"},
		"复核员",
		now.Add(24*time.Hour),
		now,
	)
	if err != nil {
		t.Fatalf("构造有效凭据: %v", err)
	}

	repo := &blockingCredentialRepository{
		credential: credential,
		manifest:   manifest,
		started:    make(chan struct{}),
		release:    make(chan struct{}),
	}
	service := application.NewServiceWithClock(repo, func() time.Time { return now })
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderDone := make(chan verificationOutcome, 1)
	go func() {
		result, callErr := service.VerifyCredential(leaderCtx, credential.CredentialNo)
		leaderDone <- verificationOutcome{result: result, err: callErr}
	}()

	<-repo.started
	followerCtx := &doneObservedContext{Context: context.Background(), observed: make(chan struct{})}
	followerDone := make(chan verificationOutcome, 1)
	go func() {
		result, callErr := service.VerifyCredential(followerCtx, credential.CredentialNo)
		followerDone <- verificationOutcome{result: result, err: callErr}
	}()

	<-followerCtx.observed
	cancelLeader()
	leader := <-leaderDone
	if !errors.Is(leader.err, context.Canceled) {
		t.Fatalf("首请求应观察到 context.Canceled，实际为 %v", leader.err)
	}
	close(repo.release)

	follower := <-followerDone
	if follower.err != nil {
		t.Fatalf("健康跟随请求继承了首请求取消: %v", follower.err)
	}
	if !follower.result.Valid {
		t.Fatalf("健康跟随请求应获得有效凭据，实际结果为 %#v", follower.result)
	}
	if got := repo.calls.Load(); got != 1 {
		t.Fatalf("同编号并发验真应只执行一次共享读取，实际执行 %d 次", got)
	}
}
