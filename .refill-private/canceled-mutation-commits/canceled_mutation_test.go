package canceledmutationcommits

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

func TestCanceledMutationCannotCommit(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "canceled-mutation.db"))
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	defer repo.Close()

	setup := application.NewService(repo)
	created, err := setup.CreateCase(context.Background(), application.CreateCaseCommand{
		IdempotencyKey:      "create-before-cancel",
		CaveCode:            "CAVE-CANCEL",
		MuralZone:           "north-wall",
		MaterialSensitivity: domain.SensitivityHigh,
		DiscoveredAt:        time.Now().UTC().Add(-time.Hour),
		Owner:               "setup-technician",
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	fixed := time.Date(2026, 8, 27, 9, 30, 0, 0, time.UTC)
	cancelDuringApply := application.NewServiceWithClock(repo, func() time.Time {
		cancel()
		return fixed
	})
	_, mutationErr := cancelDuringApply.AddEvidence(ctx, created.ID, application.AddEvidenceCommand{
		CommandMeta: application.CommandMeta{
			ExpectedVersion: created.Version,
			IdempotencyKey:  "evidence-canceled-during-apply",
			Actor:           "field-technician",
		},
		ZoneCode:          "zone-cancel",
		SamplePoints:      []string{"sample-cancel-1"},
		MicroscopyFinding: "hyphae observed",
		CultureFinding:    "active culture",
		ImageDigest:       "sha256:canceled-request",
		CoveragePercent:   18,
		ActivityScore:     6,
	})

	persisted, getErr := setup.GetCase(context.Background(), created.ID)
	if getErr != nil {
		t.Fatalf("read case after canceled mutation: %v", getErr)
	}
	audit, auditErr := setup.AuditTimeline(context.Background(), created.ID)
	if auditErr != nil {
		t.Fatalf("read audit after canceled mutation: %v", auditErr)
	}
	if !errors.Is(mutationErr, context.Canceled) || persisted.Version != created.Version || len(persisted.Evidence) != 0 || len(audit) != 1 {
		t.Fatalf("canceled mutation was committed: err=%v version=%d evidence=%d audit=%d", mutationErr, persisted.Version, len(persisted.Evidence), len(audit))
	}
}
