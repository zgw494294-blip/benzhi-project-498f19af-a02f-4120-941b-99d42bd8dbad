package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

func (s *Service) SubmitPlan(ctx context.Context, caseID string, command SubmitPlanCommand) (domain.ConservationCase, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return domain.ConservationCase{}, err
	}
	mutation := Mutation{EventType: "plan_submitted", Actor: command.Actor, Summary: "提交并通过安全边界校验的处置方案", Apply: func(c *domain.ConservationCase) error {
		if err := requireStatus(c, "提交方案", domain.StatusRiskAssessed); err != nil {
			return err
		}
		version := len(c.Plans) + 1
		plan := domain.TreatmentPlan{
			ID: s.idgen("plan"), CaseID: c.ID, Version: version, ZoneInstructions: command.ZoneInstructions,
			IsolationMeasures: command.IsolationMeasures, StopConditions: command.StopConditions,
			Rationale: strings.TrimSpace(command.Rationale), ValidationStatus: "validated",
			SubmittedBy: command.Actor, SubmittedAt: s.now().UTC(),
		}
		if err := domain.ValidatePlan(plan, *c); err != nil {
			return err
		}
		c.Plans = append(c.Plans, plan)
		c.Status = domain.StatusPilotReady
		return nil
	}}
	result, _, err := s.repo.Mutate(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, mutation)
	return result, err
}

func (s *Service) StartTrial(ctx context.Context, caseID string, command StartTrialCommand) (domain.ConservationCase, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return domain.ConservationCase{}, err
	}
	mutation := Mutation{EventType: "pilot_trial_started", Actor: command.Actor, Summary: "建立小区试验基线，观察窗状态为不足", Apply: func(c *domain.ConservationCase) error {
		if err := requireStatus(c, "建立小区试验", domain.StatusPilotReady); err != nil {
			return err
		}
		plan, ok := domain.LatestPlan(c.Plans)
		if !ok {
			return domain.NewRuleError("plan_required", "缺少已验证方案", "plan")
		}
		if command.PlanVersion != plan.Version {
			return domain.NewRuleError("stale_plan_version", "试验必须绑定当前方案版本", "planVersion")
		}
		if latest, ok := domain.LatestTrial(c.Trials); ok && latest.PlanVersion == plan.Version {
			return domain.NewRuleError("trial_already_started", "当前方案已经建立小区试验", "trial")
		}
		startedAt := command.StartedAt.UTC()
		if startedAt.IsZero() || startedAt.After(s.now().UTC().Add(5*time.Minute)) {
			return domain.NewRuleError("trial_start_invalid", "试验开始时间不能为空或晚于当前时间", "startedAt")
		}
		trial := domain.PilotTrial{
			ID: s.idgen("trial"), CaseID: c.ID, Revision: len(c.Trials) + 1, PlanVersion: plan.Version,
			PlotCode: strings.TrimSpace(command.PlotCode), StartedAt: startedAt,
			Baseline: strings.TrimSpace(command.Baseline), BaselineActivity: command.BaselineActivity,
			Observations: []domain.TrialObservation{}, WindowStatus: "insufficient",
			WindowReasonCodes: []string{"OBSERVATION_REQUIRED", "OBSERVATION_WINDOW_SHORT"},
		}
		window, err := domain.EvaluateTrial(trial)
		if err != nil {
			return err
		}
		domain.ApplyWindowResult(&trial, window, s.now().UTC())
		c.Trials = append(c.Trials, trial)
		return nil
	}}
	result, _, err := s.repo.Mutate(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, mutation)
	return result, err
}

func (s *Service) AppendTrialObservation(ctx context.Context, caseID string, command AppendTrialObservationCommand) (domain.ConservationCase, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return domain.ConservationCase{}, err
	}
	mutation := Mutation{EventType: "pilot_observation_appended", Actor: command.Actor, Summary: "追加小区试验分期观察并重新判定观察窗", Apply: func(c *domain.ConservationCase) error {
		if err := requireStatus(c, "追加小区试验观察", domain.StatusPilotReady); err != nil {
			return err
		}
		plan, ok := domain.LatestPlan(c.Plans)
		if !ok {
			return domain.NewRuleError("plan_required", "缺少已验证方案", "plan")
		}
		latest, ok := domain.LatestTrial(c.Trials)
		if !ok {
			return domain.NewRuleError("trial_required", "请先建立小区试验基线", "trialId")
		}
		if strings.TrimSpace(command.TrialID) == "" || command.TrialID != latest.ID {
			return domain.NewRuleError("stale_trial_revision", "只能向最新试验修订追加观察", "trialId")
		}
		if command.PlanVersion != plan.Version || latest.PlanVersion != plan.Version {
			return domain.NewRuleError("stale_plan_version", "观察读数绑定了旧方案版本", "planVersion")
		}
		observation := command.Observation
		observation.ObservedAt = observation.ObservedAt.UTC()
		if err := domain.ValidateObservationAppend(latest, observation); err != nil {
			return err
		}
		trial := latest
		trial.ID = s.idgen("trial")
		trial.Revision = len(c.Trials) + 1
		trial.Observations = append(append([]domain.TrialObservation(nil), latest.Observations...), observation)
		trial.Deviations = append(append([]domain.Deviation(nil), latest.Deviations...), command.Deviations...)
		window, err := domain.EvaluateTrial(trial)
		if err != nil {
			return err
		}
		domain.ApplyWindowResult(&trial, window, s.now().UTC())
		c.Trials = append(c.Trials, trial)
		if window.Status == "passed" {
			c.Status = domain.StatusReviewPending
		}
		return nil
	}}
	result, _, err := s.repo.Mutate(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, mutation)
	return result, err
}

func (s *Service) RecordTrial(ctx context.Context, caseID string, command RecordTrialCommand) (domain.ConservationCase, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return domain.ConservationCase{}, err
	}
	mutation := Mutation{EventType: "pilot_trial_recorded", Actor: command.Actor, Summary: "登记小区试验并判定观察窗", Apply: func(c *domain.ConservationCase) error {
		if err := requireStatus(c, "登记小区试验", domain.StatusPilotReady); err != nil {
			return err
		}
		plan, ok := domain.LatestPlan(c.Plans)
		if !ok {
			return domain.NewRuleError("plan_required", "缺少已验证方案", "plan")
		}
		trial := domain.PilotTrial{
			ID: s.idgen("trial"), CaseID: c.ID, Revision: len(c.Trials) + 1, PlanVersion: plan.Version,
			PlotCode: strings.TrimSpace(command.PlotCode), StartedAt: command.StartedAt.UTC(), Baseline: strings.TrimSpace(command.Baseline),
			BaselineActivity: command.BaselineActivity, Observations: command.Observations, Deviations: command.Deviations,
		}
		if trial.StartedAt.IsZero() {
			return domain.NewRuleError("trial_start_required", "试验开始时间不能为空", "startedAt")
		}
		window, err := domain.EvaluateTrial(trial)
		if err != nil {
			return err
		}
		domain.ApplyWindowResult(&trial, window, s.now().UTC())
		c.Trials = append(c.Trials, trial)
		if window.Status == "passed" {
			c.Status = domain.StatusReviewPending
		}
		return nil
	}}
	result, _, err := s.repo.Mutate(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, mutation)
	return result, err
}

func (s *Service) Review(ctx context.Context, caseID string, command ReviewCommand) (domain.ConservationCase, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return domain.ConservationCase{}, err
	}
	mutation := Mutation{EventType: "review_decided", Actor: command.Actor, Summary: fmt.Sprintf("复核结论：%s", command.Decision), Apply: func(c *domain.ConservationCase) error {
		if err := requireStatus(c, "复核裁决", domain.StatusReviewPending); err != nil {
			return err
		}
		trial, ok := domain.LatestTrial(c.Trials)
		if !ok {
			return domain.NewRuleError("trial_required", "缺少小区试验", "trial")
		}
		plan, ok := domain.LatestPlan(c.Plans)
		if !ok {
			return domain.NewRuleError("plan_required", "缺少处置方案", "plan")
		}
		decision := domain.ReviewDecision{
			Reviewer: strings.TrimSpace(command.Reviewer), Decision: command.Decision, Notes: strings.TrimSpace(command.Notes),
			Deviations: command.Deviations, EvidenceCount: len(domain.LatestEvidenceByZone(c.Evidence)),
			PlanVersion: plan.Version, TrialRevision: trial.Revision, DecidedAt: s.now().UTC(),
		}
		if err := domain.ValidateReview(trial, decision); err != nil {
			return err
		}
		c.Review = &decision
		if decision.Decision == "approve" {
			c.Status = domain.StatusReviewApproved
		} else {
			c.Status = domain.StatusRemediation
		}
		return nil
	}}
	result, _, err := s.repo.Mutate(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, mutation)
	return result, err
}
