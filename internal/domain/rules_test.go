package domain

import (
	"errors"
	"testing"
	"time"
)

func TestCalculateRiskExplainsCriticalFragileCase(t *testing.T) {
	result, err := CalculateRisk(80, 9, SensitivityHigh)
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != RiskCritical {
		t.Fatalf("want critical, got %s", result.Level)
	}
	if len(result.ReasonCodes) < 3 {
		t.Fatalf("reason codes missing: %#v", result.ReasonCodes)
	}
	if result.Explanation == "" {
		t.Fatal("explanation must not be empty")
	}
}

func TestValidatePlanRejectsFragileMaterialBoundary(t *testing.T) {
	c := ConservationCase{
		MaterialSensitivity: SensitivityHigh,
		Evidence:            []EvidenceRevision{{ID: "ev-1", ZoneCode: "Z1", Revision: 1}},
		RiskAssessments:     []RiskAssessment{{ID: "risk-1"}},
	}
	plan := TreatmentPlan{
		ZoneInstructions:  []ZoneInstruction{{ZoneCode: "Z1", CleaningMedium: "乙醇", Concentration: 50, ContactMinutes: 5}},
		IsolationMeasures: []string{"隔离"}, StopConditions: []string{"色差超限"}, Rationale: "测试",
	}
	err := ValidatePlan(plan, c)
	if err == nil {
		t.Fatal("unsafe concentration should fail")
	}
	var rule *RuleError
	if !errors.As(err, &rule) || rule.Code != "fragile_material_boundary" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvaluateTrialObservationRules(t *testing.T) {
	trial := PilotTrial{
		PlotCode: "P1", Baseline: "stable", BaselineActivity: 8,
		Observations: []TrialObservation{
			{HoursSinceStart: 0, ColorDelta: 0, ActivityScore: 8},
			{HoursSinceStart: 72, ColorDelta: 1.5, ActivityScore: 2},
		},
	}
	result, err := EvaluateTrial(trial)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "passed" {
		t.Fatalf("want passed: %#v", result)
	}
	if result.ActivityReduction != 75 {
		t.Fatalf("want 75 reduction, got %f", result.ActivityReduction)
	}
	trial.Observations[1].HoursSinceStart = 24
	result, err = EvaluateTrial(trial)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "insufficient" {
		t.Fatalf("want insufficient, got %s", result.Status)
	}
}

func TestBuildEvidenceTrendSortsAndCalculatesDeltas(t *testing.T) {
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	trend, err := BuildEvidenceTrend("Z1", []EvidenceRevision{
		{ID: "ev-3", ZoneCode: "Z1", Revision: 3, CoveragePercent: 12, ActivityScore: 3, RiskLevel: RiskModerate, RiskReasonCodes: []string{"LOCALIZED_COVERAGE"}, RecordedAt: now.Add(2 * time.Hour)},
		{ID: "ev-1", ZoneCode: "Z1", Revision: 1, CoveragePercent: 30, ActivityScore: 8, RiskLevel: RiskHigh, RiskReasonCodes: []string{"HIGH_BIOACTIVITY"}, RecordedAt: now},
		{ID: "ev-2", ZoneCode: "Z1", Revision: 2, CoveragePercent: 18, ActivityScore: 5, RiskLevel: RiskHigh, RiskReasonCodes: []string{"ACTIVE_GROWTH"}, RecordedAt: now.Add(time.Hour)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if trend.OverallDirection != TrendImproving || trend.LatestRiskLevel != RiskModerate {
		t.Fatalf("unexpected trend: %#v", trend)
	}
	if trend.Revisions[0].CoverageDelta != nil || trend.Revisions[0].Conclusion != TrendBaseline {
		t.Fatalf("baseline must have empty deltas: %#v", trend.Revisions[0])
	}
	if got := *trend.Revisions[1].CoverageDelta; got != -12 {
		t.Fatalf("want coverage delta -12, got %v", got)
	}
	if got := *trend.Revisions[2].ActivityDelta; got != -2 {
		t.Fatalf("want activity delta -2, got %v", got)
	}
	if trend.Revisions[2].RiskReasonCodes[0] != "LOCALIZED_COVERAGE" {
		t.Fatal("risk reasons were not retained")
	}
}

func TestBuildEvidenceTrendRejectsRevisionGap(t *testing.T) {
	_, err := BuildEvidenceTrend("Z1", []EvidenceRevision{
		{ID: "ev-1", ZoneCode: "Z1", Revision: 1},
		{ID: "ev-3", ZoneCode: "Z1", Revision: 3},
	})
	if !errors.Is(err, ErrDataConsistency) {
		t.Fatalf("want consistency error, got %v", err)
	}
}

func TestCredentialVerificationDetectsManifestChange(t *testing.T) {
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	c := ConservationCase{
		ID: "case-1", Status: StatusFrozen,
		FrozenManifest: &FrozenManifest{CaseID: "case-1", ManifestDigest: "manifest-a", EvidenceRefs: map[string]string{"Z1": "ev-1"}},
	}
	credential, err := BuildCredential(c, "MURAL-1", []string{"Z1"}, []string{"保持稳定"}, "复核员", now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if valid, _ := VerifyCredential(credential, *c.FrozenManifest, now); !valid {
		t.Fatal("credential should verify")
	}
	changed := *c.FrozenManifest
	changed.ManifestDigest = "manifest-b"
	if valid, _ := VerifyCredential(credential, changed, now); valid {
		t.Fatal("changed manifest should fail")
	}
}
