package regression_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/store"
)

func TestIdempotencyKeyCannotAliasDifferentOperations(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "idempotency.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	t.Cleanup(func() { _ = service.Close() })
	ctx := context.Background()
	now := time.Now().UTC()
	c, err := service.CreateCase(ctx, application.CreateCaseCommand{
		IdempotencyKey: "create", CaveCode: "C1", MuralZone: "北壁",
		MaterialSensitivity: domain.SensitivityHigh, DiscoveredAt: now.Add(-time.Hour), Owner: "技术员",
	})
	if err != nil {
		t.Fatal(err)
	}
	const reusedKey = "same-key-for-different-commands"
	c, err = service.AddEvidence(ctx, c.ID, application.AddEvidenceCommand{
		CommandMeta: application.CommandMeta{ExpectedVersion: c.Version, IdempotencyKey: reusedKey, Actor: "技术员"},
		ZoneCode: "Z1", SamplePoints: []string{"P1"}, MicroscopyFinding: "菌丝",
		CultureFinding: "阳性", ImageDigest: "sha256:x", CoveragePercent: 20, ActivityScore: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.AssessRisk(ctx, c.ID, application.AssessRiskCommand{
		CommandMeta: application.CommandMeta{ExpectedVersion: c.Version, IdempotencyKey: reusedKey, Actor: "评估员"},
		Assessor:    "评估员",
	})
	if err == nil {
		t.Fatalf("TestIdempotencyKeyCannotAliasDifferentOperations: reused key returned success with stale status %s", result.Status)
	}
}
