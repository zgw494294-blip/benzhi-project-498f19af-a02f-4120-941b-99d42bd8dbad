package audit_timeline_cache_alias_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/store"
)

func TestAuditTimelineCacheDoesNotExposeMutableState(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "audit-cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	t.Cleanup(func() { _ = service.Close() })

	created, err := service.CreateCase(context.Background(), application.CreateCaseCommand{
		IdempotencyKey:      "audit-cache-create",
		CaveCode:            "C-AUDIT",
		MuralZone:           "北壁",
		MaterialSensitivity: domain.SensitivityHigh,
		DiscoveredAt:        time.Now().UTC().Add(-time.Hour),
		Owner:               "技术员",
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.AuditTimeline(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Details == nil {
		t.Fatalf("unexpected initial timeline: %#v", first)
	}
	originalSummary := first[0].Summary
	originalCaveCode := first[0].Details["caveCode"]
	first[0].Summary = "调用方伪造的摘要"
	first[0].Details["caveCode"] = "调用方伪造的洞窟"

	second, err := service.AuditTimeline(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second[0].Summary != originalSummary || second[0].Details["caveCode"] != originalCaveCode {
		t.Fatalf("TestAuditTimelineCacheDoesNotExposeMutableState: cached timeline was polluted: %#v", second[0])
	}
}
