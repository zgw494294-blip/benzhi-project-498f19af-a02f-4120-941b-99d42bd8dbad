package immutable_sync_rollback_cache_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/store"
	_ "modernc.org/sqlite"
)

func TestImmutableSyncCacheDoesNotSurviveRollback(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	dbPath := filepath.Join(t.TempDir(), "rollback-cache.db")
	repo, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	defer repo.Close()
	service := application.NewServiceWithClock(repo, func() time.Time { return now })

	created, err := service.CreateCase(ctx, application.CreateCaseCommand{
		IdempotencyKey:      "create-rollback-cache-case",
		CaveCode:            "CAVE-RB",
		MuralZone:           "north-wall",
		MaterialSensitivity: domain.SensitivityHigh,
		DiscoveredAt:        now.Add(-time.Hour),
		Owner:               "technician",
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	control, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("open control connection: %v", err)
	}
	defer control.Close()
	_, err = control.ExecContext(ctx, `CREATE TRIGGER fail_evidence_audit
		BEFORE INSERT ON audit_events
		WHEN NEW.event_type = 'evidence_revised'
		BEGIN SELECT RAISE(ABORT, 'forced audit failure'); END`)
	if err != nil {
		t.Fatalf("install failure trigger: %v", err)
	}

	command := application.AddEvidenceCommand{
		CommandMeta: application.CommandMeta{
			ExpectedVersion: 1,
			IdempotencyKey:  "evidence-after-rollback",
			Actor:           "assessor",
		},
		ZoneCode:          "zone-a",
		SamplePoints:      []string{"sample-1"},
		MicroscopyFinding: "active hyphae",
		CultureFinding:    "positive",
		ImageDigest:       "sha256:rollback-evidence",
		CoveragePercent:   28,
		ActivityScore:     7,
	}
	if _, err := service.AddEvidence(ctx, created.ID, command); err == nil {
		t.Fatal("first mutation should fail after immutable evidence insertion")
	}
	if _, err := control.ExecContext(ctx, `DROP TRIGGER fail_evidence_audit`); err != nil {
		t.Fatalf("remove failure trigger: %v", err)
	}

	retried, err := service.AddEvidence(ctx, created.ID, command)
	if err != nil {
		t.Fatalf("retry evidence mutation: %v", err)
	}
	if len(retried.Evidence) != 1 {
		t.Fatalf("aggregate should contain one evidence revision, got %d", len(retried.Evidence))
	}
	trends, err := service.EvidenceTrends(ctx, created.ID, "zone-a")
	if err != nil {
		t.Fatalf("retry committed but normalized evidence is missing: %v", err)
	}
	if len(trends) != 1 || len(trends[0].Revisions) != 1 {
		t.Fatalf("expected one persisted evidence revision, got %#v", trends)
	}
}
