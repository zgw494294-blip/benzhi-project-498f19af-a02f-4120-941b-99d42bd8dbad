package domain

import (
	"fmt"
	"sort"
	"strings"
)

type MediumBoundary struct {
	MaxConcentration  float64
	MaxContactMinutes int
}

var mediumBoundaries = map[string]MediumBoundary{
	"去离子水":   {MaxConcentration: 100, MaxContactMinutes: 20},
	"乙醇":     {MaxConcentration: 70, MaxContactMinutes: 10},
	"季铵盐":    {MaxConcentration: 0.5, MaxContactMinutes: 8},
	"酶清洁剂":   {MaxConcentration: 2, MaxContactMinutes: 15},
	"低压干式清洁": {MaxConcentration: 0, MaxContactMinutes: 12},
}

func ValidatePlan(plan TreatmentPlan, c ConservationCase) error {
	if len(c.Evidence) == 0 || len(c.RiskAssessments) == 0 {
		return NewRuleError("risk_assessment_required", "提交方案前必须完成证据登记和风险评估", "riskAssessment")
	}
	if len(plan.ZoneInstructions) == 0 {
		return NewRuleError("zone_instruction_required", "至少需要一条污染区处置指令", "zoneInstructions")
	}
	if len(plan.IsolationMeasures) == 0 {
		return NewRuleError("isolation_required", "必须填写隔离措施", "isolationMeasures")
	}
	if len(plan.StopConditions) == 0 {
		return NewRuleError("stop_condition_required", "必须填写停止条件", "stopConditions")
	}
	if strings.TrimSpace(plan.Rationale) == "" {
		return NewRuleError("rationale_required", "必须填写方案依据", "rationale")
	}
	latestEvidence := LatestEvidenceByZone(c.Evidence)
	seen := map[string]bool{}
	for i, instruction := range plan.ZoneInstructions {
		prefix := fmt.Sprintf("zoneInstructions[%d]", i)
		if strings.TrimSpace(instruction.ZoneCode) == "" {
			return NewRuleError("zone_required", "污染区编号不能为空", prefix+".zoneCode")
		}
		if seen[instruction.ZoneCode] {
			return NewRuleError("duplicate_zone", "同一方案中污染区编号不能重复", prefix+".zoneCode")
		}
		seen[instruction.ZoneCode] = true
		if _, ok := latestEvidence[instruction.ZoneCode]; !ok {
			return NewRuleError("zone_evidence_missing", "处置污染区必须存在有效证据", prefix+".zoneCode")
		}
		boundary, ok := mediumBoundaries[instruction.CleaningMedium]
		if !ok {
			return NewRuleError("unsupported_medium", "清洁介质不在安全清单内", prefix+".cleaningMedium")
		}
		if instruction.Concentration < 0 || instruction.Concentration > boundary.MaxConcentration {
			return NewRuleError("concentration_unsafe", fmt.Sprintf("%s 浓度不得超过 %.2f", instruction.CleaningMedium, boundary.MaxConcentration), prefix+".concentration")
		}
		if instruction.ContactMinutes <= 0 || instruction.ContactMinutes > boundary.MaxContactMinutes {
			return NewRuleError("contact_time_unsafe", fmt.Sprintf("%s 接触时长必须在 1 到 %d 分钟", instruction.CleaningMedium, boundary.MaxContactMinutes), prefix+".contactMinutes")
		}
		if c.MaterialSensitivity == SensitivityHigh && instruction.CleaningMedium == "乙醇" && instruction.Concentration > 30 {
			return NewRuleError("fragile_material_boundary", "高敏感材料使用乙醇时浓度不得超过 30", prefix+".concentration")
		}
	}
	for zone := range latestEvidence {
		if !seen[zone] {
			return NewRuleError("zone_not_covered", "方案必须覆盖每个已登记污染区", "zoneInstructions")
		}
	}
	for _, condition := range plan.StopConditions {
		if strings.TrimSpace(condition) == "" {
			return NewRuleError("empty_stop_condition", "停止条件不能包含空项", "stopConditions")
		}
	}
	return nil
}

func SupportedMedia() []string {
	result := make([]string, 0, len(mediumBoundaries))
	for medium := range mediumBoundaries {
		result = append(result, medium)
	}
	sort.Strings(result)
	return result
}

func LatestPlan(plans []TreatmentPlan) (TreatmentPlan, bool) {
	if len(plans) == 0 {
		return TreatmentPlan{}, false
	}
	latest := plans[0]
	for _, plan := range plans[1:] {
		if plan.Version > latest.Version {
			latest = plan
		}
	}
	return latest, true
}
