package credentialverificationcachestale_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/store"
)

func TestVerificationCacheDoesNotSurviveRevocation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	repo, err := store.Open(filepath.Join(t.TempDir(), "reproduction.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewServiceWithClock(repo, func() time.Time { return now })
	t.Cleanup(func() { _ = service.Close() })

	c, err := service.CreateCase(ctx, application.CreateCaseCommand{
		IdempotencyKey: "cache-create", CaveCode: "C1", MuralZone: "北壁",
		MaterialSensitivity: domain.SensitivityHigh, DiscoveredAt: now.Add(-time.Hour), Owner: "技术员",
	})
	mustSucceed(t, err)
	c, err = service.AddEvidence(ctx, c.ID, application.AddEvidenceCommand{
		CommandMeta: meta(c, "cache-evidence", "技术员"), ZoneCode: "Z1", SamplePoints: []string{"P1"},
		MicroscopyFinding: "菌丝", CultureFinding: "阳性", ImageDigest: "sha256:evidence",
		CoveragePercent: 20, ActivityScore: 7,
	})
	mustSucceed(t, err)
	c, err = service.AssessRisk(ctx, c.ID, application.AssessRiskCommand{
		CommandMeta: meta(c, "cache-risk", "评估员"), Assessor: "评估员",
	})
	mustSucceed(t, err)
	c, err = service.SubmitPlan(ctx, c.ID, application.SubmitPlanCommand{
		CommandMeta: meta(c, "cache-plan", "评估员"),
		ZoneInstructions: []domain.ZoneInstruction{{
			ZoneCode: "Z1", CleaningMedium: "去离子水", Concentration: 10, ContactMinutes: 5,
		}},
		IsolationMeasures: []string{"隔离"}, StopConditions: []string{"色差超限"}, Rationale: "低干预",
	})
	mustSucceed(t, err)
	started := now.Add(-73 * time.Hour)
	c, err = service.RecordTrial(ctx, c.ID, application.RecordTrialCommand{
		CommandMeta: meta(c, "cache-trial", "技术员"), PlotCode: "PLOT1", StartedAt: started,
		Baseline: "稳定", BaselineActivity: 8,
		Observations: []domain.TrialObservation{
			{ObservedAt: started, HoursSinceStart: 0, ColorDelta: 0, ActivityScore: 8},
			{ObservedAt: now, HoursSinceStart: 73, ColorDelta: 1, ActivityScore: 2},
		},
	})
	mustSucceed(t, err)
	c, err = service.Review(ctx, c.ID, application.ReviewCommand{
		CommandMeta: meta(c, "cache-review", "复核员"), Reviewer: "复核员", Decision: "approve", Notes: "通过",
	})
	mustSucceed(t, err)
	c, err = service.Freeze(ctx, c.ID, application.FreezeCommand{
		CommandMeta: meta(c, "cache-freeze", "复核员"), FrozenBy: "复核员",
	})
	mustSucceed(t, err)
	c, err = service.IssueCredential(ctx, c.ID, application.IssueCredentialCommand{
		CommandMeta: meta(c, "cache-issue", "复核员"), AllowedZones: []string{"Z1"},
		Conditions: []string{"巡检"}, IssuedBy: "复核员", ValidUntil: now.Add(24 * time.Hour),
	})
	mustSucceed(t, err)
	number := c.Credential.CredentialNo

	warmed, err := service.VerifyCredential(ctx, number)
	if err != nil || !warmed.Valid {
		t.Fatalf("failed to warm verification cache: result=%#v err=%v", warmed, err)
	}
	c, err = service.RevokeCredential(ctx, c.ID, application.RevokeCredentialCommand{
		CommandMeta: meta(c, "cache-revoke", "复核员"), CredentialNo: number, Reason: "现场出现新污染",
	})
	if err != nil || c.Credential.RevocationStatus != "revoked" {
		t.Fatalf("failed to persist revocation: credential=%#v err=%v", c.Credential, err)
	}

	afterRevocation, err := service.VerifyCredential(ctx, number)
	if err != nil {
		t.Fatal(err)
	}
	if afterRevocation.Valid || afterRevocation.Message != "凭据已撤销" || afterRevocation.Credential.RevocationStatus != "revoked" {
		t.Fatalf("cached verification remained valid after revocation: %#v", afterRevocation)
	}
}

func meta(c domain.ConservationCase, key, actor string) application.CommandMeta {
	return application.CommandMeta{ExpectedVersion: c.Version, IdempotencyKey: key, Actor: actor}
}

func mustSucceed(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
