package application

import (
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

type CommandMeta struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Actor           string `json:"actor"`
}

type CreateCaseCommand struct {
	IdempotencyKey      string                     `json:"idempotencyKey"`
	CaveCode            string                     `json:"caveCode"`
	MuralZone           string                     `json:"muralZone"`
	MaterialSensitivity domain.MaterialSensitivity `json:"materialSensitivity"`
	DiscoveredAt        time.Time                  `json:"discoveredAt"`
	Owner               string                     `json:"owner"`
}

type AddEvidenceCommand struct {
	CommandMeta
	ZoneCode          string   `json:"zoneCode"`
	SamplePoints      []string `json:"samplePoints"`
	MicroscopyFinding string   `json:"microscopyFinding"`
	CultureFinding    string   `json:"cultureFinding"`
	ImageDigest       string   `json:"imageDigest"`
	CoveragePercent   float64  `json:"coveragePercent"`
	ActivityScore     float64  `json:"activityScore"`
}

type AssessRiskCommand struct {
	CommandMeta
	Assessor string `json:"assessor"`
}

type SubmitPlanCommand struct {
	CommandMeta
	ZoneInstructions  []domain.ZoneInstruction `json:"zoneInstructions"`
	IsolationMeasures []string                 `json:"isolationMeasures"`
	StopConditions    []string                 `json:"stopConditions"`
	Rationale         string                   `json:"rationale"`
}

type RecordTrialCommand struct {
	CommandMeta
	PlotCode         string                    `json:"plotCode"`
	StartedAt        time.Time                 `json:"startedAt"`
	Baseline         string                    `json:"baseline"`
	BaselineActivity float64                   `json:"baselineActivity"`
	Observations     []domain.TrialObservation `json:"observations"`
	Deviations       []domain.Deviation        `json:"deviations"`
}

type StartTrialCommand struct {
	CommandMeta
	PlanVersion      int       `json:"planVersion"`
	PlotCode         string    `json:"plotCode"`
	StartedAt        time.Time `json:"startedAt"`
	Baseline         string    `json:"baseline"`
	BaselineActivity float64   `json:"baselineActivity"`
}

type AppendTrialObservationCommand struct {
	CommandMeta
	TrialID     string                  `json:"trialId"`
	PlanVersion int                     `json:"planVersion"`
	Observation domain.TrialObservation `json:"observation"`
	Deviations  []domain.Deviation      `json:"deviations,omitempty"`
}

type ReviewCommand struct {
	CommandMeta
	Reviewer   string             `json:"reviewer"`
	Decision   string             `json:"decision"`
	Notes      string             `json:"notes"`
	Deviations []domain.Deviation `json:"deviations"`
}

type FreezeCommand struct {
	CommandMeta
	FrozenBy string `json:"frozenBy"`
}

type IssueCredentialCommand struct {
	CommandMeta
	AllowedZones []string  `json:"allowedZones"`
	Conditions   []string  `json:"conditions"`
	IssuedBy     string    `json:"issuedBy"`
	ValidUntil   time.Time `json:"validUntil"`
}

type RevokeCredentialCommand struct {
	CommandMeta
	CredentialNo string `json:"credentialNo"`
	Reason       string `json:"reason"`
}

type VerificationResult struct {
	Valid      bool                    `json:"valid"`
	Message    string                  `json:"message"`
	Credential domain.SafetyCredential `json:"credential"`
	Manifest   domain.FrozenManifest   `json:"manifest"`
}
