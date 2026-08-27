package domain

import (
	"fmt"
	"sort"
	"strings"
)

type RiskResult struct {
	Level       RiskLevel `json:"level"`
	Score       float64   `json:"score"`
	ReasonCodes []string  `json:"reasonCodes"`
	Explanation string    `json:"explanation"`
}

func CalculateRisk(coverage, activity float64, sensitivity MaterialSensitivity) (RiskResult, error) {
	if coverage < 0 || coverage > 100 {
		return RiskResult{}, NewRuleError("coverage_out_of_range", "覆盖范围必须在 0 到 100 之间", "coveragePercent")
	}
	if activity < 0 || activity > 10 {
		return RiskResult{}, NewRuleError("activity_out_of_range", "活性评分必须在 0 到 10 之间", "activityScore")
	}
	sensitivityWeight := 0.0
	reasons := make([]string, 0, 4)
	switch sensitivity {
	case SensitivityLow:
		sensitivityWeight = 8
	case SensitivityMedium:
		sensitivityWeight = 18
		reasons = append(reasons, "MATERIAL_MODERATELY_FRAGILE")
	case SensitivityHigh:
		sensitivityWeight = 30
		reasons = append(reasons, "MATERIAL_HIGHLY_FRAGILE")
	default:
		return RiskResult{}, NewRuleError("invalid_sensitivity", "材料敏感性必须为 low、medium 或 high", "materialSensitivity")
	}
	score := coverage*0.38 + activity*3.2 + sensitivityWeight
	if coverage >= 35 {
		reasons = append(reasons, "WIDE_COVERAGE")
	} else if coverage >= 10 {
		reasons = append(reasons, "LOCALIZED_COVERAGE")
	}
	if activity >= 7 {
		reasons = append(reasons, "HIGH_BIOACTIVITY")
	} else if activity >= 4 {
		reasons = append(reasons, "ACTIVE_GROWTH")
	}
	level := RiskLow
	switch {
	case score >= 70:
		level = RiskCritical
	case score >= 50:
		level = RiskHigh
	case score >= 28:
		level = RiskModerate
	}
	sort.Strings(reasons)
	return RiskResult{
		Level:       level,
		Score:       score,
		ReasonCodes: reasons,
		Explanation: fmt.Sprintf("覆盖 %.1f%%、活性 %.1f/10、材料敏感性 %s，综合分 %.1f，判定为 %s。", coverage, activity, sensitivity, score, level),
	}, nil
}

func BuildAssessment(evidence []EvidenceRevision, sensitivity MaterialSensitivity, assessor string) (RiskAssessment, error) {
	if strings.TrimSpace(assessor) == "" {
		return RiskAssessment{}, NewRuleError("assessor_required", "评估人员不能为空", "assessor")
	}
	if len(evidence) == 0 {
		return RiskAssessment{}, NewRuleError("evidence_required", "至少需要一个污染区证据修订", "evidence")
	}
	latest := LatestEvidenceByZone(evidence)
	assessment := RiskAssessment{EvidenceRefs: map[string]string{}, ZoneLevels: map[string]RiskLevel{}}
	maxRank := -1
	reasonSet := map[string]struct{}{}
	explanations := make([]string, 0, len(latest))
	for zone, item := range latest {
		result, err := CalculateRisk(item.CoveragePercent, item.ActivityScore, sensitivity)
		if err != nil {
			return RiskAssessment{}, err
		}
		assessment.EvidenceRefs[zone] = item.ID
		assessment.ZoneLevels[zone] = result.Level
		if riskRank(result.Level) > maxRank {
			maxRank = riskRank(result.Level)
			assessment.OverallLevel = result.Level
		}
		for _, reason := range result.ReasonCodes {
			reasonSet[reason] = struct{}{}
		}
		explanations = append(explanations, zone+"："+result.Explanation)
	}
	for reason := range reasonSet {
		assessment.ReasonCodes = append(assessment.ReasonCodes, reason)
	}
	sort.Strings(assessment.ReasonCodes)
	sort.Strings(explanations)
	assessment.Explanation = strings.Join(explanations, " ")
	assessment.Assessor = strings.TrimSpace(assessor)
	return assessment, nil
}

func LatestEvidenceByZone(items []EvidenceRevision) map[string]EvidenceRevision {
	latest := make(map[string]EvidenceRevision)
	for _, item := range items {
		current, exists := latest[item.ZoneCode]
		if !exists || item.Revision > current.Revision {
			latest[item.ZoneCode] = item
		}
	}
	return latest
}

func riskRank(level RiskLevel) int {
	switch level {
	case RiskCritical:
		return 4
	case RiskHigh:
		return 3
	case RiskModerate:
		return 2
	case RiskLow:
		return 1
	default:
		return 0
	}
}
