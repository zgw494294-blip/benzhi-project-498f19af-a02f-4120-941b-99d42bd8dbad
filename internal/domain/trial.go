package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	MinimumObservationHours  = 72.0
	MaximumColorDelta        = 3.0
	MinimumActivityReduction = 60.0
)

type WindowResult struct {
	Status            string   `json:"status"`
	ColorDelta        float64  `json:"colorDelta"`
	ActivityReduction float64  `json:"activityReduction"`
	ReasonCodes       []string `json:"reasonCodes"`
}

func EvaluateTrial(trial PilotTrial) (WindowResult, error) {
	if strings.TrimSpace(trial.PlotCode) == "" {
		return WindowResult{}, NewRuleError("plot_required", "试验小区编号不能为空", "plotCode")
	}
	if strings.TrimSpace(trial.Baseline) == "" {
		return WindowResult{}, NewRuleError("baseline_required", "必须填写试验基线", "baseline")
	}
	if trial.BaselineActivity <= 0 || trial.BaselineActivity > 10 {
		return WindowResult{}, NewRuleError("baseline_activity_invalid", "基线活性必须大于 0 且不超过 10", "baselineActivity")
	}
	if len(trial.Observations) == 0 {
		return WindowResult{Status: "insufficient", ReasonCodes: []string{"OBSERVATION_REQUIRED", "OBSERVATION_WINDOW_SHORT"}}, nil
	}
	latest := trial.Observations[0]
	for i, observation := range trial.Observations {
		if (!trial.StartedAt.IsZero() && (observation.ObservedAt.IsZero() || observation.ObservedAt.Before(trial.StartedAt))) || observation.HoursSinceStart < 0 {
			return WindowResult{}, NewRuleError("observation_time_invalid", "观察时间不能早于试验开始时间", fmt.Sprintf("observations[%d].observedAt", i))
		}
		if observation.ColorDelta < 0 || observation.ActivityScore < 0 || observation.ActivityScore > 10 {
			return WindowResult{}, NewRuleError("observation_reading_invalid", "色差和活性读数超出允许范围", fmt.Sprintf("observations[%d]", i))
		}
		if i > 0 {
			previous := trial.Observations[i-1]
			timeNotIncreasing := !observation.ObservedAt.IsZero() || !previous.ObservedAt.IsZero()
			if (timeNotIncreasing && !observation.ObservedAt.After(previous.ObservedAt)) || observation.HoursSinceStart <= previous.HoursSinceStart {
				return WindowResult{}, NewRuleError("observation_not_monotonic", "观察时间和观察时长必须严格递增且不得重复", fmt.Sprintf("observations[%d]", i))
			}
		}
		latest = observation
	}
	reduction := (trial.BaselineActivity - latest.ActivityScore) / trial.BaselineActivity * 100
	result := WindowResult{Status: "passed", ColorDelta: latest.ColorDelta, ActivityReduction: reduction}
	if len(trial.Observations) < 2 {
		result.Status = "insufficient"
		result.ReasonCodes = append(result.ReasonCodes, "TWO_READINGS_REQUIRED")
	}
	if latest.HoursSinceStart < MinimumObservationHours {
		result.Status = "insufficient"
		result.ReasonCodes = append(result.ReasonCodes, "OBSERVATION_WINDOW_SHORT")
	}
	if latest.ColorDelta > MaximumColorDelta {
		result.Status = "failed"
		result.ReasonCodes = append(result.ReasonCodes, "COLOR_DELTA_EXCEEDED")
	}
	if reduction < MinimumActivityReduction {
		result.Status = "failed"
		result.ReasonCodes = append(result.ReasonCodes, "ACTIVITY_REDUCTION_LOW")
	}
	for _, deviation := range trial.Deviations {
		if strings.TrimSpace(deviation.Code) == "" || strings.TrimSpace(deviation.Description) == "" {
			return WindowResult{}, NewRuleError("deviation_incomplete", "偏差必须包含编号和说明", "deviations")
		}
	}
	return result, nil
}

func ApplyWindowResult(trial *PilotTrial, result WindowResult, now time.Time) {
	trial.WindowStatus = result.Status
	trial.ColorDelta = result.ColorDelta
	trial.ActivityReduction = result.ActivityReduction
	trial.WindowReasonCodes = append([]string(nil), result.ReasonCodes...)
	trial.CompletedAt = nil
	if result.Status == "passed" {
		completed := now
		trial.CompletedAt = &completed
	}
}

func ValidateObservationAppend(trial PilotTrial, observation TrialObservation) error {
	if observation.ObservedAt.IsZero() || !observation.ObservedAt.After(trial.StartedAt) {
		return NewRuleError("observation_time_invalid", "观察时间必须晚于试验开始时间", "observation.observedAt")
	}
	if observation.HoursSinceStart <= 0 {
		return NewRuleError("observation_time_invalid", "观察时长必须大于 0", "observation.hoursSinceStart")
	}
	if observation.ColorDelta < 0 {
		return NewRuleError("color_delta_invalid", "色差不能为负数", "observation.colorDelta")
	}
	if observation.ActivityScore < 0 || observation.ActivityScore > 10 {
		return NewRuleError("activity_score_invalid", "活性评分必须在 0 到 10 之间", "observation.activityScore")
	}
	if len(trial.Observations) == 0 {
		return nil
	}
	latest := trial.Observations[len(trial.Observations)-1]
	if !observation.ObservedAt.After(latest.ObservedAt) || observation.HoursSinceStart <= latest.HoursSinceStart {
		return NewRuleError("observation_not_monotonic", "观察时间和观察时长必须严格递增且不得重复", "observation")
	}
	return nil
}

func LatestTrial(trials []PilotTrial) (PilotTrial, bool) {
	if len(trials) == 0 {
		return PilotTrial{}, false
	}
	latest := trials[0]
	for _, trial := range trials[1:] {
		if trial.Revision > latest.Revision {
			latest = trial
		}
	}
	return latest, true
}

func ValidateReview(trial PilotTrial, decision ReviewDecision) error {
	if trial.WindowStatus != "passed" {
		return NewRuleError("trial_not_passed", "小区试验观察窗尚未通过", "trial")
	}
	if strings.TrimSpace(decision.Reviewer) == "" {
		return NewRuleError("reviewer_required", "复核员不能为空", "reviewer")
	}
	if decision.Decision != "approve" && decision.Decision != "reject" {
		return NewRuleError("decision_invalid", "复核决定必须为 approve 或 reject", "decision")
	}
	input := make(map[string]Deviation, len(decision.Deviations))
	for _, item := range decision.Deviations {
		input[item.Code] = item
	}
	for _, deviation := range trial.Deviations {
		judged, ok := input[deviation.Code]
		if !ok || (judged.Decision != "accept" && judged.Decision != "reject") {
			return NewRuleError("deviation_unresolved", "必须逐项接受或驳回全部偏差", "deviations")
		}
		if judged.Decision == "reject" && strings.TrimSpace(judged.Resolution) == "" {
			return NewRuleError("remediation_required", "驳回偏差必须填写整改要求", "deviations")
		}
	}
	if decision.Decision == "approve" {
		for _, judged := range decision.Deviations {
			if judged.Decision == "reject" {
				return NewRuleError("approval_has_rejection", "存在被驳回偏差时不能通过复核", "decision")
			}
		}
	}
	return nil
}
