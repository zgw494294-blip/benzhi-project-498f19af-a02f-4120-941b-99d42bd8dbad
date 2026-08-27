package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

type Clock func() time.Time

type Service struct {
	repo        Repository
	now         Clock
	idgen       func(string) string
	lifecycleMu sync.RWMutex
	closed      bool
}

var errServiceClosed = errors.New("应用服务已经关闭")

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now, idgen: randomID}
}

func NewServiceWithClock(repo Repository, clock Clock) *Service {
	service := NewService(repo)
	service.now = clock
	return service
}

func randomID(prefix string) string {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(raw)
}

func validateMeta(meta CommandMeta) error {
	if meta.ExpectedVersion < 1 {
		return domain.NewRuleError("expected_version_required", "expectedVersion 必须为正整数", "expectedVersion")
	}
	if strings.TrimSpace(meta.IdempotencyKey) == "" {
		return domain.NewRuleError("idempotency_key_required", "idempotencyKey 不能为空", "idempotencyKey")
	}
	if strings.TrimSpace(meta.Actor) == "" {
		return domain.NewRuleError("actor_required", "actor 不能为空", "actor")
	}
	return nil
}

func (s *Service) ensureAvailable() error {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
	if s.closed {
		return errServiceClosed
	}
	return nil
}

func requireStatus(c *domain.ConservationCase, action string, allowed ...domain.CaseStatus) error {
	for _, status := range allowed {
		if c.Status == status {
			return nil
		}
	}
	return &domain.StateError{Action: action, State: c.Status}
}

func (s *Service) GetCase(ctx context.Context, id string) (domain.ConservationCase, error) {
	if err := s.ensureAvailable(); err != nil {
		return domain.ConservationCase{}, err
	}
	if strings.TrimSpace(id) == "" {
		return domain.ConservationCase{}, domain.ErrNotFound
	}
	return s.repo.Get(ctx, id)
}

func (s *Service) ListCases(ctx context.Context) ([]domain.ConservationCase, error) {
	if err := s.ensureAvailable(); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *Service) AuditTimeline(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	if err := s.ensureAvailable(); err != nil {
		return nil, err
	}
	return s.repo.Audit(ctx, caseID)
}

func (s *Service) EvidenceTrends(ctx context.Context, caseID, zoneCode string) ([]domain.ZoneEvidenceTrend, error) {
	if err := s.ensureAvailable(); err != nil {
		return nil, err
	}
	if _, err := s.repo.Get(ctx, caseID); err != nil {
		return nil, err
	}
	items, err := s.repo.EvidenceRevisions(ctx, caseID, strings.TrimSpace(zoneCode))
	if err != nil {
		return nil, err
	}
	grouped := make(map[string][]domain.EvidenceRevision)
	for _, item := range items {
		grouped[item.ZoneCode] = append(grouped[item.ZoneCode], item)
	}
	zones := make([]string, 0, len(grouped))
	for zone := range grouped {
		zones = append(zones, zone)
	}
	sort.Strings(zones)
	result := make([]domain.ZoneEvidenceTrend, 0, len(zones))
	for _, zone := range zones {
		trend, err := domain.BuildEvidenceTrend(zone, grouped[zone])
		if err != nil {
			return nil, err
		}
		result = append(result, trend)
	}
	return result, nil
}

func (s *Service) VerifyCredential(ctx context.Context, number string) (VerificationResult, error) {
	if err := s.ensureAvailable(); err != nil {
		return VerificationResult{}, err
	}
	credential, manifest, err := s.repo.FindCredential(ctx, number)
	if err != nil {
		return VerificationResult{}, err
	}
	valid, message := domain.VerifyCredential(credential, manifest, s.now().UTC())
	return VerificationResult{Valid: valid, Message: message, Credential: credential, Manifest: manifest}, nil
}

func (s *Service) Close() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.repo.Close()
}
