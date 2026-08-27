package application_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/store"
)

func TestCompleteWorkflowPersistsAndVerifies(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "workflow.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	t.Cleanup(func() { service.Close() })
	ctx := context.Background()
	now := time.Now().UTC()
	c, err := service.CreateCase(ctx, application.CreateCaseCommand{
		IdempotencyKey: "create-1", CaveCode: "C1", MuralZone: "北壁", MaterialSensitivity: domain.SensitivityHigh,
		DiscoveredAt: now.Add(-time.Hour), Owner: "技术员",
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.AddEvidence(ctx, c.ID, application.AddEvidenceCommand{
		CommandMeta: meta(c, "ev-1", "技术员"), ZoneCode: "Z1", SamplePoints: []string{"P1"},
		MicroscopyFinding: "菌丝", CultureFinding: "阳性", ImageDigest: "sha256:x", CoveragePercent: 20, ActivityScore: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	retried, err := service.AddEvidence(ctx, c.ID, application.AddEvidenceCommand{
		CommandMeta: application.CommandMeta{ExpectedVersion: 1, IdempotencyKey: "ev-1", Actor: "技术员"},
		ZoneCode:    "DIFFERENT", SamplePoints: []string{"P2"}, MicroscopyFinding: "different",
		CultureFinding: "different", ImageDigest: "different", CoveragePercent: 1, ActivityScore: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if retried.Version != c.Version || len(retried.Evidence) != 1 {
		t.Fatal("idempotent retry changed aggregate")
	}
	_, err = service.AssessRisk(ctx, c.ID, application.AssessRiskCommand{CommandMeta: application.CommandMeta{ExpectedVersion: 1, IdempotencyKey: "stale", Actor: "评估员"}, Assessor: "评估员"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	c, err = service.AssessRisk(ctx, c.ID, application.AssessRiskCommand{CommandMeta: meta(c, "risk", "评估员"), Assessor: "评估员"})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.SubmitPlan(ctx, c.ID, application.SubmitPlanCommand{
		CommandMeta:       meta(c, "plan", "评估员"),
		ZoneInstructions:  []domain.ZoneInstruction{{ZoneCode: "Z1", CleaningMedium: "去离子水", Concentration: 10, ContactMinutes: 5}},
		IsolationMeasures: []string{"隔离"}, StopConditions: []string{"色差超限"}, Rationale: "低干预",
	})
	if err != nil {
		t.Fatal(err)
	}
	started := now.Add(-73 * time.Hour)
	c, err = service.RecordTrial(ctx, c.ID, application.RecordTrialCommand{
		CommandMeta: meta(c, "trial", "技术员"), PlotCode: "PLOT1", StartedAt: started, Baseline: "稳定", BaselineActivity: 8,
		Observations: []domain.TrialObservation{
			{ObservedAt: started, HoursSinceStart: 0, ColorDelta: 0, ActivityScore: 8},
			{ObservedAt: now, HoursSinceStart: 73, ColorDelta: 1, ActivityScore: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.Review(ctx, c.ID, application.ReviewCommand{
		CommandMeta: meta(c, "review", "复核员"), Reviewer: "复核员", Decision: "approve", Notes: "通过",
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.Freeze(ctx, c.ID, application.FreezeCommand{CommandMeta: meta(c, "freeze", "复核员"), FrozenBy: "复核员"})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.IssueCredential(ctx, c.ID, application.IssueCredentialCommand{
		CommandMeta: meta(c, "credential", "复核员"), AllowedZones: []string{"Z1"}, Conditions: []string{"巡检"},
		IssuedBy: "复核员", ValidUntil: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	verification, err := service.VerifyCredential(ctx, c.Credential.CredentialNo)
	if err != nil || !verification.Valid {
		t.Fatalf("verification failed: %#v %v", verification, err)
	}
	audit, err := service.AuditTimeline(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(audit) != 8 {
		t.Fatalf("want 8 events, got %d", len(audit))
	}
	for index := 1; index < len(audit); index++ {
		if audit[index].PreviousHash != audit[index-1].EventHash {
			t.Fatal("audit chain broken")
		}
	}
}

func meta(c domain.ConservationCase, key, actor string) application.CommandMeta {
	return application.CommandMeta{ExpectedVersion: c.Version, IdempotencyKey: key, Actor: actor}
}
