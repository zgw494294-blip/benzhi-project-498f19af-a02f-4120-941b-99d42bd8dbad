package domain

import "time"

type CaseStatus string

const (
	StatusEvidenceCollecting CaseStatus = "evidence_collecting"
	StatusRiskAssessed       CaseStatus = "risk_assessed"
	StatusPilotReady         CaseStatus = "pilot_ready"
	StatusReviewPending      CaseStatus = "review_pending"
	StatusRemediation        CaseStatus = "remediation_required"
	StatusReviewApproved     CaseStatus = "review_approved"
	StatusFrozen             CaseStatus = "frozen"
	StatusCredentialIssued   CaseStatus = "credential_issued"
)

type MaterialSensitivity string

const (
	SensitivityLow    MaterialSensitivity = "low"
	SensitivityMedium MaterialSensitivity = "medium"
	SensitivityHigh   MaterialSensitivity = "high"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskModerate RiskLevel = "moderate"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type ConservationCase struct {
	ID                  string              `json:"id"`
	CaveCode            string              `json:"caveCode"`
	MuralZone           string              `json:"muralZone"`
	MaterialSensitivity MaterialSensitivity `json:"materialSensitivity"`
	DiscoveredAt        time.Time           `json:"discoveredAt"`
	Owner               string              `json:"owner"`
	Status              CaseStatus          `json:"status"`
	Version             int64               `json:"version"`
	CreatedAt           time.Time           `json:"createdAt"`
	UpdatedAt           time.Time           `json:"updatedAt"`
	Evidence            []EvidenceRevision  `json:"evidence"`
	RiskAssessments     []RiskAssessment    `json:"riskAssessments"`
	Plans               []TreatmentPlan     `json:"plans"`
	Trials              []PilotTrial        `json:"trials"`
	Review              *ReviewDecision     `json:"review,omitempty"`
	FrozenManifest      *FrozenManifest     `json:"frozenManifest,omitempty"`
	Credential          *SafetyCredential   `json:"credential,omitempty"`
}

type EvidenceRevision struct {
	ID                string    `json:"id"`
	CaseID            string    `json:"caseId"`
	ZoneCode          string    `json:"zoneCode"`
	Revision          int       `json:"revision"`
	SamplePoints      []string  `json:"samplePoints"`
	MicroscopyFinding string    `json:"microscopyFinding"`
	CultureFinding    string    `json:"cultureFinding"`
	ImageDigest       string    `json:"imageDigest"`
	CoveragePercent   float64   `json:"coveragePercent"`
	ActivityScore     float64   `json:"activityScore"`
	RiskLevel         RiskLevel `json:"riskLevel"`
	RiskReasonCodes   []string  `json:"riskReasonCodes"`
	RecordedAt        time.Time `json:"recordedAt"`
}

type EvidenceRevisionTrend struct {
	EvidenceID        string    `json:"evidenceId"`
	Revision          int       `json:"revision"`
	RecordedAt        time.Time `json:"recordedAt"`
	CoveragePercent   float64   `json:"coveragePercent"`
	ActivityScore     float64   `json:"activityScore"`
	RiskLevel         RiskLevel `json:"riskLevel"`
	RiskReasonCodes   []string  `json:"riskReasonCodes"`
	CoverageDelta     *float64  `json:"coverageDelta"`
	ActivityDelta     *float64  `json:"activityDelta"`
	CoverageDirection string    `json:"coverageDirection"`
	ActivityDirection string    `json:"activityDirection"`
	RiskDirection     string    `json:"riskDirection"`
	Conclusion        string    `json:"conclusion"`
}

type ZoneEvidenceTrend struct {
	ZoneCode         string                  `json:"zoneCode"`
	Revisions        []EvidenceRevisionTrend `json:"revisions"`
	LatestRiskLevel  RiskLevel               `json:"latestRiskLevel"`
	OverallDirection string                  `json:"overallDirection"`
}

type RiskAssessment struct {
	ID           string               `json:"id"`
	CaseID       string               `json:"caseId"`
	EvidenceRefs map[string]string    `json:"evidenceRefs"`
	OverallLevel RiskLevel            `json:"overallLevel"`
	ZoneLevels   map[string]RiskLevel `json:"zoneLevels"`
	ReasonCodes  []string             `json:"reasonCodes"`
	Explanation  string               `json:"explanation"`
	Assessor     string               `json:"assessor"`
	AssessedAt   time.Time            `json:"assessedAt"`
}

type ZoneInstruction struct {
	ZoneCode       string  `json:"zoneCode"`
	CleaningMedium string  `json:"cleaningMedium"`
	Concentration  float64 `json:"concentration"`
	ContactMinutes int     `json:"contactMinutes"`
}

type TreatmentPlan struct {
	ID                string            `json:"id"`
	CaseID            string            `json:"caseId"`
	Version           int               `json:"version"`
	ZoneInstructions  []ZoneInstruction `json:"zoneInstructions"`
	IsolationMeasures []string          `json:"isolationMeasures"`
	StopConditions    []string          `json:"stopConditions"`
	Rationale         string            `json:"rationale"`
	ValidationStatus  string            `json:"validationStatus"`
	SubmittedBy       string            `json:"submittedBy"`
	SubmittedAt       time.Time         `json:"submittedAt"`
}

type TrialObservation struct {
	ObservedAt      time.Time `json:"observedAt"`
	HoursSinceStart float64   `json:"hoursSinceStart"`
	ColorDelta      float64   `json:"colorDelta"`
	ActivityScore   float64   `json:"activityScore"`
	Note            string    `json:"note"`
}

type Deviation struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Resolution  string `json:"resolution,omitempty"`
	Decision    string `json:"decision,omitempty"`
}

type PilotTrial struct {
	ID                string             `json:"id"`
	CaseID            string             `json:"caseId"`
	Revision          int                `json:"revision"`
	PlanVersion       int                `json:"planVersion"`
	PlotCode          string             `json:"plotCode"`
	StartedAt         time.Time          `json:"startedAt"`
	Baseline          string             `json:"baseline"`
	BaselineActivity  float64            `json:"baselineActivity"`
	Observations      []TrialObservation `json:"observations"`
	ColorDelta        float64            `json:"colorDelta"`
	ActivityReduction float64            `json:"activityReduction"`
	Deviations        []Deviation        `json:"deviations"`
	WindowStatus      string             `json:"windowStatus"`
	WindowReasonCodes []string           `json:"windowReasonCodes"`
	CompletedAt       *time.Time         `json:"completedAt,omitempty"`
}

type ReviewDecision struct {
	Reviewer      string      `json:"reviewer"`
	Decision      string      `json:"decision"`
	Notes         string      `json:"notes"`
	Deviations    []Deviation `json:"deviations"`
	EvidenceCount int         `json:"evidenceCount"`
	PlanVersion   int         `json:"planVersion"`
	TrialRevision int         `json:"trialRevision"`
	DecidedAt     time.Time   `json:"decidedAt"`
}

type FrozenManifest struct {
	CaseID         string            `json:"caseId"`
	EvidenceRefs   map[string]string `json:"evidenceRefs"`
	PlanID         string            `json:"planId"`
	PlanVersion    int               `json:"planVersion"`
	TrialID        string            `json:"trialId"`
	TrialRevision  int               `json:"trialRevision"`
	ReviewDigest   string            `json:"reviewDigest"`
	ManifestDigest string            `json:"manifestDigest"`
	FrozenBy       string            `json:"frozenBy"`
	FrozenAt       time.Time         `json:"frozenAt"`
}

type SafetyCredential struct {
	CredentialNo         string     `json:"credentialNo"`
	CaseID               string     `json:"caseId"`
	FrozenManifestDigest string     `json:"frozenManifestDigest"`
	AllowedZones         []string   `json:"allowedZones"`
	Conditions           []string   `json:"conditions"`
	IssuedBy             string     `json:"issuedBy"`
	IssuedAt             time.Time  `json:"issuedAt"`
	ValidUntil           time.Time  `json:"validUntil"`
	SignatureDigest      string     `json:"signatureDigest"`
	RevocationStatus     string     `json:"revocationStatus"`
	RevocationReason     string     `json:"revocationReason,omitempty"`
	RevokedBy            string     `json:"revokedBy,omitempty"`
	RevokedAt            *time.Time `json:"revokedAt,omitempty"`
}

type AuditEvent struct {
	CaseID       string         `json:"caseId"`
	Sequence     int64          `json:"sequence"`
	EventType    string         `json:"eventType"`
	Actor        string         `json:"actor"`
	Summary      string         `json:"summary"`
	OccurredAt   time.Time      `json:"occurredAt"`
	CaseVersion  int64          `json:"caseVersion"`
	PreviousHash string         `json:"previousHash"`
	EventHash    string         `json:"eventHash"`
	Details      map[string]any `json:"details,omitempty"`
}
