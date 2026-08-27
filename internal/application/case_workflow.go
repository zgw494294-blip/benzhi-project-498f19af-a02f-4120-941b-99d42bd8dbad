package application

import (
	"context"
	"strings"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

func (s *Service) CreateCase(ctx context.Context, command CreateCaseCommand) (domain.ConservationCase, error) {
	command.CaveCode = strings.TrimSpace(command.CaveCode)
	command.MuralZone = strings.TrimSpace(command.MuralZone)
	command.Owner = strings.TrimSpace(command.Owner)
	if strings.TrimSpace(command.IdempotencyKey) == "" {
		return domain.ConservationCase{}, domain.NewRuleError("idempotency_key_required", "idempotencyKey 不能为空", "idempotencyKey")
	}
	if command.CaveCode == "" || command.MuralZone == "" || command.Owner == "" {
		return domain.ConservationCase{}, domain.NewRuleError("case_fields_required", "洞窟、壁画区域和负责人不能为空", "case")
	}
	if command.MaterialSensitivity != domain.SensitivityLow && command.MaterialSensitivity != domain.SensitivityMedium && command.MaterialSensitivity != domain.SensitivityHigh {
		return domain.ConservationCase{}, domain.NewRuleError("sensitivity_invalid", "材料敏感性无效", "materialSensitivity")
	}
	if command.DiscoveredAt.IsZero() || command.DiscoveredAt.After(s.now().Add(5*time.Minute)) {
		return domain.ConservationCase{}, domain.NewRuleError("discovery_time_invalid", "发现时间不能为空或晚于当前时间", "discoveredAt")
	}
	now := s.now().UTC()
	c := domain.ConservationCase{
		ID: s.idgen("case"), CaveCode: command.CaveCode, MuralZone: command.MuralZone,
		MaterialSensitivity: command.MaterialSensitivity, DiscoveredAt: command.DiscoveredAt.UTC(), Owner: command.Owner,
		Status: domain.StatusEvidenceCollecting, Version: 1, CreatedAt: now, UpdatedAt: now,
		Evidence: []domain.EvidenceRevision{}, RiskAssessments: []domain.RiskAssessment{}, Plans: []domain.TreatmentPlan{}, Trials: []domain.PilotTrial{},
	}
	created, _, err := s.repo.Create(ctx, c, command.IdempotencyKey, Mutation{
		EventType: "case_created", Actor: command.Owner, Summary: "建立处置案并进入污染证据收集阶段",
		Details: map[string]any{"caveCode": c.CaveCode, "muralZone": c.MuralZone},
	})
	return created, err
}

func (s *Service) AddEvidence(ctx context.Context, caseID string, command AddEvidenceCommand) (domain.ConservationCase, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return domain.ConservationCase{}, err
	}
	if strings.TrimSpace(command.ZoneCode) == "" || len(command.SamplePoints) == 0 {
		return domain.ConservationCase{}, domain.NewRuleError("evidence_identity_required", "污染区和采样点不能为空", "evidence")
	}
	if strings.TrimSpace(command.MicroscopyFinding) == "" || strings.TrimSpace(command.CultureFinding) == "" || strings.TrimSpace(command.ImageDigest) == "" {
		return domain.ConservationCase{}, domain.NewRuleError("evidence_finding_required", "显微观察、培养结果和图像摘要不能为空", "evidence")
	}
	mutation := Mutation{EventType: "evidence_revised", Actor: command.Actor, Summary: "登记不可变污染证据修订", Apply: func(c *domain.ConservationCase) error {
		if err := requireStatus(c, "登记证据", domain.StatusEvidenceCollecting, domain.StatusRemediation); err != nil {
			return err
		}
		result, err := domain.CalculateRisk(command.CoveragePercent, command.ActivityScore, c.MaterialSensitivity)
		if err != nil {
			return err
		}
		revision := 1
		for _, item := range c.Evidence {
			if item.ZoneCode == command.ZoneCode && item.Revision >= revision {
				revision = item.Revision + 1
			}
		}
		item := domain.EvidenceRevision{
			ID: s.idgen("evidence"), CaseID: c.ID, ZoneCode: strings.TrimSpace(command.ZoneCode), Revision: revision,
			SamplePoints: command.SamplePoints, MicroscopyFinding: strings.TrimSpace(command.MicroscopyFinding),
			CultureFinding: strings.TrimSpace(command.CultureFinding), ImageDigest: strings.TrimSpace(command.ImageDigest),
			CoveragePercent: command.CoveragePercent, ActivityScore: command.ActivityScore,
			RiskLevel: result.Level, RiskReasonCodes: result.ReasonCodes, RecordedAt: s.now().UTC(),
		}
		c.Evidence = append(c.Evidence, item)
		if c.Status == domain.StatusRemediation {
			c.Status = domain.StatusEvidenceCollecting
			c.RiskAssessments = nil
			c.Review = nil
		}
		return nil
	}}
	result, _, err := s.repo.Mutate(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, mutation)
	return result, err
}

func (s *Service) AssessRisk(ctx context.Context, caseID string, command AssessRiskCommand) (domain.ConservationCase, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return domain.ConservationCase{}, err
	}
	mutation := Mutation{EventType: "risk_assessed", Actor: command.Actor, Summary: "完成污染风险分级", Apply: func(c *domain.ConservationCase) error {
		if err := requireStatus(c, "风险评估", domain.StatusEvidenceCollecting); err != nil {
			return err
		}
		assessment, err := domain.BuildAssessment(c.Evidence, c.MaterialSensitivity, command.Assessor)
		if err != nil {
			return err
		}
		assessment.ID, assessment.CaseID, assessment.AssessedAt = s.idgen("risk"), c.ID, s.now().UTC()
		c.RiskAssessments = append(c.RiskAssessments, assessment)
		c.Status = domain.StatusRiskAssessed
		return nil
	}}
	result, _, err := s.repo.Mutate(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, mutation)
	return result, err
}
