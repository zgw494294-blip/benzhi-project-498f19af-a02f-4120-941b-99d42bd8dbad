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

func TestTrendStagedTrialAndCredentialRevocationPersist(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "extensions.db")
	repo, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	now := time.Now().UTC()
	c, err := service.CreateCase(ctx, application.CreateCaseCommand{
		IdempotencyKey: "extension-create", CaveCode: "C2", MuralZone: "东壁",
		MaterialSensitivity: domain.SensitivityHigh, DiscoveredAt: now.Add(-24 * time.Hour), Owner: "技术员",
	})
	if err != nil {
		t.Fatal(err)
	}
	for index, reading := range []struct{ coverage, activity float64 }{{30, 8}, {18, 5}, {12, 3}} {
		c, err = service.AddEvidence(ctx, c.ID, application.AddEvidenceCommand{
			CommandMeta: application.CommandMeta{ExpectedVersion: c.Version, IdempotencyKey: "extension-evidence-" + string(rune('1'+index)), Actor: "技术员"},
			ZoneCode: "Z1", SamplePoints: []string{"P1"}, MicroscopyFinding: "菌丝", CultureFinding: "阳性",
			ImageDigest: "sha256:trend", CoveragePercent: reading.coverage, ActivityScore: reading.activity,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	trends, err := service.EvidenceTrends(ctx, c.ID, "Z1")
	if err != nil || len(trends) != 1 || trends[0].OverallDirection != domain.TrendImproving {
		t.Fatalf("trend failed: %#v %v", trends, err)
	}
	if _, err := service.EvidenceTrends(ctx, c.ID, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want missing zone error, got %v", err)
	}

	c, err = service.AssessRisk(ctx, c.ID, application.AssessRiskCommand{CommandMeta: meta(c, "extension-risk", "评估员"), Assessor: "评估员"})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.SubmitPlan(ctx, c.ID, application.SubmitPlanCommand{
		CommandMeta: meta(c, "extension-plan", "评估员"),
		ZoneInstructions: []domain.ZoneInstruction{{ZoneCode: "Z1", CleaningMedium: "去离子水", Concentration: 10, ContactMinutes: 5}},
		IsolationMeasures: []string{"隔离"}, StopConditions: []string{"色差超限"}, Rationale: "低干预",
	})
	if err != nil {
		t.Fatal(err)
	}
	started := now.Add(-74 * time.Hour)
	c, err = service.StartTrial(ctx, c.ID, application.StartTrialCommand{
		CommandMeta: meta(c, "extension-trial-start", "技术员"), PlanVersion: 1, PlotCode: "PLOT-1",
		StartedAt: started, Baseline: "处置前稳定", BaselineActivity: 8,
	})
	if err != nil || c.Status != domain.StatusPilotReady || c.Trials[0].WindowStatus != "insufficient" {
		t.Fatalf("start trial failed: %#v %v", c, err)
	}
	readings := []domain.TrialObservation{
		{ObservedAt: started.Add(24 * time.Hour), HoursSinceStart: 24, ColorDelta: 0.8, ActivityScore: 6},
		{ObservedAt: started.Add(48 * time.Hour), HoursSinceStart: 48, ColorDelta: 1, ActivityScore: 4},
		{ObservedAt: started.Add(73 * time.Hour), HoursSinceStart: 73, ColorDelta: 1.2, ActivityScore: 2},
	}
	for index, observation := range readings {
		latest := c.Trials[len(c.Trials)-1]
		c, err = service.AppendTrialObservation(ctx, c.ID, application.AppendTrialObservationCommand{
			CommandMeta: meta(c, "extension-observation-"+string(rune('1'+index)), "技术员"),
			TrialID: latest.ID, PlanVersion: 1, Observation: observation,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	latest := c.Trials[len(c.Trials)-1]
	if c.Status != domain.StatusReviewPending || latest.WindowStatus != "passed" || len(latest.Observations) != 3 || len(c.Trials) != 4 {
		t.Fatalf("staged observations did not pass: %#v", latest)
	}

	c, err = service.Review(ctx, c.ID, application.ReviewCommand{CommandMeta: meta(c, "extension-review", "复核员"), Reviewer: "复核员", Decision: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.Freeze(ctx, c.ID, application.FreezeCommand{CommandMeta: meta(c, "extension-freeze", "复核员"), FrozenBy: "复核员"})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.IssueCredential(ctx, c.ID, application.IssueCredentialCommand{
		CommandMeta: meta(c, "extension-issue", "复核员"), AllowedZones: []string{"Z1"}, Conditions: []string{"巡检"},
		IssuedBy: "复核员", ValidUntil: now.Add(365 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	number := c.Credential.CredentialNo
	issuedVersion := c.Version
	c, err = service.RevokeCredential(ctx, c.ID, application.RevokeCredentialCommand{
		CommandMeta: meta(c, "extension-revoke", "复核员"), CredentialNo: number, Reason: "现场出现新污染",
	})
	if err != nil || c.Credential.RevocationStatus != "revoked" || c.Version != issuedVersion+1 {
		t.Fatalf("revocation failed: %#v %v", c.Credential, err)
	}
	retried, err := service.RevokeCredential(ctx, c.ID, application.RevokeCredentialCommand{
		CommandMeta: application.CommandMeta{ExpectedVersion: issuedVersion, IdempotencyKey: "extension-revoke", Actor: "复核员"},
		CredentialNo: number, Reason: "不同重试内容",
	})
	if err != nil || retried.Version != c.Version {
		t.Fatalf("idempotent revocation retry failed: %#v %v", retried, err)
	}
	_, err = service.RevokeCredential(ctx, c.ID, application.RevokeCredentialCommand{
		CommandMeta: meta(c, "extension-revoke-again", "复核员"), CredentialNo: number, Reason: "再次撤销",
	})
	if !errors.Is(err, domain.ErrCredentialRevoked) {
		t.Fatalf("want already revoked error, got %v", err)
	}
	verification, err := service.VerifyCredential(ctx, number)
	if err != nil || verification.Valid || verification.Message != "凭据已撤销" {
		t.Fatalf("revoked credential verified: %#v %v", verification, err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}

	reopenedRepo, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	reopened := application.NewService(reopenedRepo)
	t.Cleanup(func() { _ = reopened.Close() })
	verification, err = reopened.VerifyCredential(ctx, number)
	if err != nil || verification.Valid || verification.Credential.RevocationReason != "现场出现新污染" {
		t.Fatalf("revocation was not persisted: %#v %v", verification, err)
	}
}
