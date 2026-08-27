package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

type Clock func() time.Time

type Service struct {
	repo  Repository
	now   Clock
	idgen func(string) string
}

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

func requireStatus(c *domain.ConservationCase, action string, allowed ...domain.CaseStatus) error {
	for _, status := range allowed {
		if c.Status == status {
			return nil
		}
	}
	return &domain.StateError{Action: action, State: c.Status}
}

func (s *Service) GetCase(ctx context.Context, id string) (domain.ConservationCase, error) {
	if strings.TrimSpace(id) == "" {
		return domain.ConservationCase{}, domain.ErrNotFound
	}
	return s.repo.Get(ctx, id)
}

func (s *Service) ListCases(ctx context.Context) ([]domain.ConservationCase, error) {
	return s.repo.List(ctx)
}

func (s *Service) AuditTimeline(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	return s.repo.Audit(ctx, caseID)
}

func (s *Service) EvidenceTrends(ctx context.Context, caseID, zoneCode string) ([]domain.ZoneEvidenceTrend, error) {
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
	credential, manifest, err := s.repo.FindCredential(ctx, number)
	if err != nil {
		return VerificationResult{}, err
	}
	valid, message := domain.VerifyCredential(credential, manifest, s.now().UTC())
	return VerificationResult{Valid: valid, Message: message, Credential: credential, Manifest: manifest}, nil
}

func (s *Service) Close() error { return s.repo.Close() }
