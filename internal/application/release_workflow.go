package application

import (
	"context"
	"fmt"
	"strings"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

func (s *Service) Freeze(ctx context.Context, caseID string, command FreezeCommand) (domain.ConservationCase, error) {
	if err := s.ensureAvailable(); err != nil {
		return domain.ConservationCase{}, err
	}
	if err := validateMeta(command.CommandMeta); err != nil {
		return domain.ConservationCase{}, err
	}
	mutation := Mutation{EventType: "evidence_frozen", Actor: command.Actor, Summary: "冻结证据、方案、试验与复核引用清单", Apply: func(c *domain.ConservationCase) error {
		if err := requireStatus(c, "冻结证据", domain.StatusReviewApproved); err != nil {
			return err
		}
		manifest, err := domain.BuildFrozenManifest(*c, command.FrozenBy, s.now().UTC())
		if err != nil {
			return err
		}
		c.FrozenManifest = &manifest
		c.Status = domain.StatusFrozen
		return nil
	}}
	result, _, err := s.repo.Mutate(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, mutation)
	return result, err
}

func (s *Service) IssueCredential(ctx context.Context, caseID string, command IssueCredentialCommand) (domain.ConservationCase, error) {
	if err := s.ensureAvailable(); err != nil {
		return domain.ConservationCase{}, err
	}
	if err := validateMeta(command.CommandMeta); err != nil {
		return domain.ConservationCase{}, err
	}
	mutation := Mutation{EventType: "credential_issued", Actor: command.Actor, Summary: "签发开放安全凭据", Apply: func(c *domain.ConservationCase) error {
		if err := requireStatus(c, "签发凭据", domain.StatusFrozen); err != nil {
			return err
		}
		number := fmt.Sprintf("MURAL-%s-%s", s.now().UTC().Format("20060102"), strings.ToUpper(s.idgen("")[1:9]))
		credential, err := domain.BuildCredential(*c, number, append([]string(nil), command.AllowedZones...), append([]string(nil), command.Conditions...), command.IssuedBy, command.ValidUntil, s.now().UTC())
		if err != nil {
			return err
		}
		c.Credential = &credential
		c.Status = domain.StatusCredentialIssued
		return nil
	}}
	result, _, err := s.repo.Mutate(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, mutation)
	return result, err
}

func (s *Service) RevokeCredential(ctx context.Context, caseID string, command RevokeCredentialCommand) (domain.ConservationCase, error) {
	if err := s.ensureAvailable(); err != nil {
		return domain.ConservationCase{}, err
	}
	if err := validateMeta(command.CommandMeta); err != nil {
		return domain.ConservationCase{}, err
	}
	now := s.now().UTC()
	reason := strings.TrimSpace(command.Reason)
	number := strings.TrimSpace(command.CredentialNo)
	mutation := Mutation{
		EventType: "credential_revoked", Actor: command.Actor, Summary: "撤销开放安全凭据",
		Details: map[string]any{"credentialNo": number, "reason": reason, "revokedBy": strings.TrimSpace(command.Actor), "revokedAt": now},
		Apply: func(c *domain.ConservationCase) error {
			return domain.RevokeCredential(c, number, reason, command.Actor, now)
		},
	}
	result, _, err := s.repo.Mutate(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, mutation)
	return result, err
}
