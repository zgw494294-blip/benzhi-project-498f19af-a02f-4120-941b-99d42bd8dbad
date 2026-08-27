package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

func StableDigest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("生成规范摘要: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func BuildFrozenManifest(c ConservationCase, actor string, now time.Time) (FrozenManifest, error) {
	if c.Status != StatusReviewApproved || c.Review == nil || c.Review.Decision != "approve" {
		return FrozenManifest{}, NewRuleError("review_approval_required", "只有复核通过的处置案可以冻结", "status")
	}
	plan, ok := LatestPlan(c.Plans)
	if !ok {
		return FrozenManifest{}, NewRuleError("plan_required", "冻结前缺少有效方案", "plan")
	}
	trial, ok := LatestTrial(c.Trials)
	if !ok || trial.WindowStatus != "passed" {
		return FrozenManifest{}, NewRuleError("trial_required", "冻结前缺少通过的小区试验", "trial")
	}
	if strings.TrimSpace(actor) == "" {
		return FrozenManifest{}, NewRuleError("freezer_required", "冻结操作人不能为空", "actor")
	}
	refs := map[string]string{}
	for zone, evidence := range LatestEvidenceByZone(c.Evidence) {
		refs[zone] = evidence.ID
	}
	reviewDigest, err := StableDigest(c.Review)
	if err != nil {
		return FrozenManifest{}, err
	}
	manifest := FrozenManifest{
		CaseID: c.ID, EvidenceRefs: refs, PlanID: plan.ID, PlanVersion: plan.Version,
		TrialID: trial.ID, TrialRevision: trial.Revision, ReviewDigest: reviewDigest,
		FrozenBy: strings.TrimSpace(actor), FrozenAt: now.UTC(),
	}
	digestInput := struct {
		CaseID        string
		EvidenceRefs  map[string]string
		PlanID        string
		PlanVersion   int
		TrialID       string
		TrialRevision int
		ReviewDigest  string
	}{c.ID, refs, plan.ID, plan.Version, trial.ID, trial.Revision, reviewDigest}
	manifest.ManifestDigest, err = StableDigest(digestInput)
	return manifest, err
}

func BuildCredential(c ConservationCase, number string, zones, conditions []string, issuer string, validUntil, now time.Time) (SafetyCredential, error) {
	if c.Status != StatusFrozen || c.FrozenManifest == nil {
		return SafetyCredential{}, NewRuleError("frozen_manifest_required", "只有已冻结处置案可以签发凭据", "status")
	}
	if strings.TrimSpace(number) == "" || strings.TrimSpace(issuer) == "" {
		return SafetyCredential{}, NewRuleError("credential_identity_required", "凭据编号和签发人不能为空", "credential")
	}
	if len(zones) == 0 || len(conditions) == 0 {
		return SafetyCredential{}, NewRuleError("credential_scope_required", "凭据必须包含开放区域和开放条件", "allowedZones")
	}
	if !validUntil.After(now) {
		return SafetyCredential{}, NewRuleError("validity_invalid", "凭据有效期必须晚于签发时间", "validUntil")
	}
	allowed := map[string]bool{}
	for zone := range c.FrozenManifest.EvidenceRefs {
		allowed[zone] = true
	}
	for _, zone := range zones {
		if !allowed[zone] {
			return SafetyCredential{}, NewRuleError("zone_outside_manifest", "开放区域不在冻结清单中", "allowedZones")
		}
	}
	sort.Strings(zones)
	sort.Strings(conditions)
	credential := SafetyCredential{
		CredentialNo: number, CaseID: c.ID, FrozenManifestDigest: c.FrozenManifest.ManifestDigest,
		AllowedZones: zones, Conditions: conditions, IssuedBy: strings.TrimSpace(issuer),
		IssuedAt: now.UTC(), ValidUntil: validUntil.UTC(), RevocationStatus: "active",
	}
	signatureInput := struct {
		CredentialNo string
		CaseID       string
		Manifest     string
		Zones        []string
		Conditions   []string
		IssuedBy     string
		IssuedAt     string
		ValidUntil   string
	}{credential.CredentialNo, credential.CaseID, credential.FrozenManifestDigest, credential.AllowedZones, credential.Conditions, credential.IssuedBy, credential.IssuedAt.Format(time.RFC3339Nano), credential.ValidUntil.Format(time.RFC3339Nano)}
	signature, err := StableDigest(signatureInput)
	if err != nil {
		return SafetyCredential{}, err
	}
	credential.SignatureDigest = signature
	return credential, nil
}

func VerifyCredential(credential SafetyCredential, manifest FrozenManifest, now time.Time) (bool, string) {
	if credential.RevocationStatus != "active" {
		return false, "凭据已撤销"
	}
	if now.After(credential.ValidUntil) {
		return false, "凭据已过期"
	}
	if credential.FrozenManifestDigest != manifest.ManifestDigest {
		return false, "冻结清单摘要不匹配"
	}
	copyCredential := credential
	copyCredential.SignatureDigest = ""
	signatureInput := struct {
		CredentialNo string
		CaseID       string
		Manifest     string
		Zones        []string
		Conditions   []string
		IssuedBy     string
		IssuedAt     string
		ValidUntil   string
	}{copyCredential.CredentialNo, copyCredential.CaseID, copyCredential.FrozenManifestDigest, copyCredential.AllowedZones, copyCredential.Conditions, copyCredential.IssuedBy, copyCredential.IssuedAt.Format(time.RFC3339Nano), copyCredential.ValidUntil.Format(time.RFC3339Nano)}
	expected, err := StableDigest(signatureInput)
	if err != nil || expected != credential.SignatureDigest {
		return false, "签名摘要不匹配"
	}
	return true, "凭据有效，且与冻结清单一致"
}

func RevokeCredential(c *ConservationCase, number, reason, actor string, now time.Time) error {
	if c.Status != StatusCredentialIssued || c.Credential == nil {
		return &StateError{Action: "撤销凭据", State: c.Status}
	}
	if strings.TrimSpace(number) == "" || number != c.Credential.CredentialNo {
		return NewRuleError("credential_number_mismatch", "请求凭据编号与案件当前凭据不一致", "credentialNo")
	}
	if strings.TrimSpace(reason) == "" {
		return NewRuleError("revocation_reason_required", "撤销原因不能为空", "reason")
	}
	if strings.TrimSpace(actor) == "" {
		return NewRuleError("revocation_actor_required", "撤销操作人不能为空", "actor")
	}
	if c.Credential.RevocationStatus != "active" {
		return ErrCredentialRevoked
	}
	revokedAt := now.UTC()
	c.Credential.RevocationStatus = "revoked"
	c.Credential.RevocationReason = strings.TrimSpace(reason)
	c.Credential.RevokedBy = strings.TrimSpace(actor)
	c.Credential.RevokedAt = &revokedAt
	return nil
}

func AuditHash(previous string, event AuditEvent) string {
	payload := fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s", previous, event.CaseID, event.Sequence, event.EventType, event.Actor, event.Summary, event.OccurredAt.UTC().Format(time.RFC3339Nano))
	hash := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(hash[:])
}
