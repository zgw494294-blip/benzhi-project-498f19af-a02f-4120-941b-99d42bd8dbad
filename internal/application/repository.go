package application

import (
	"context"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

type Mutation struct {
	EventType string
	Actor     string
	Summary   string
	Details   map[string]any
	Apply     func(*domain.ConservationCase) error
}

type Repository interface {
	Create(context.Context, domain.ConservationCase, string, Mutation) (domain.ConservationCase, bool, error)
	Mutate(context.Context, string, int64, string, Mutation) (domain.ConservationCase, bool, error)
	Get(context.Context, string) (domain.ConservationCase, error)
	List(context.Context) ([]domain.ConservationCase, error)
	Audit(context.Context, string) ([]domain.AuditEvent, error)
	EvidenceRevisions(context.Context, string, string) ([]domain.EvidenceRevision, error)
	FindCredential(context.Context, string) (domain.SafetyCredential, domain.FrozenManifest, error)
	Close() error
}
