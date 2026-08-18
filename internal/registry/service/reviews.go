package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentregistry-dev/agentregistry/pkg/models"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/database"
	"github.com/jackc/pgx/v5"
)

func (s *registryServiceImpl) CreateReview(
	ctx context.Context,
	artifactType,
	artifactName,
	artifactVersion,
	reviewType,
	outcome,
	notes string,
) (*models.Review, error) {
	resourceType, normalizedArtifactType, err := reviewResourceType(artifactType)
	if err != nil {
		return nil, err
	}
	if s.cfg == nil {
		return nil, fmt.Errorf("%w: configuration is unavailable", database.ErrInvalidInput)
	}

	reviewConfig := s.cfg.ReviewConfig()
	if !reviewConfig.HasType(reviewType) {
		return nil, fmt.Errorf("%w: unconfigured review type %q", database.ErrInvalidInput, reviewType)
	}
	if !reviewConfig.HasOutcome(outcome) {
		return nil, fmt.Errorf("%w: unconfigured outcome %q", database.ErrInvalidInput, outcome)
	}

	session, ok := auth.AuthSessionFrom(ctx)
	if !ok || session == nil {
		return nil, auth.ErrUnauthenticated
	}
	if !s.canPerform(ctx, artifactName, resourceType, auth.PermissionActionReview) {
		return nil, auth.ErrForbidden
	}

	user := session.Principal().User
	review := &models.Review{
		ArtifactType:        normalizedArtifactType,
		ArtifactName:        artifactName,
		ArtifactVersion:     artifactVersion,
		ReviewType:          reviewType,
		Outcome:             outcome,
		ReviewerSubject:     user.Subject,
		ReviewerAuthMethod:  string(user.AuthMethod),
		ReviewerDisplayName: user.DisplayName,
		Notes:               notes,
	}

	return database.InTransactionT(ctx, s.db, func(ctx context.Context, tx pgx.Tx) (*models.Review, error) {
		if err := s.ensureReviewArtifactExists(ctx, tx, normalizedArtifactType, artifactName, artifactVersion); err != nil {
			return nil, err
		}
		return s.db.CreateReview(ctx, tx, review)
	})
}

func reviewResourceType(artifactType string) (auth.PermissionArtifactType, string, error) {
	normalized := strings.ToLower(strings.TrimSpace(artifactType))
	switch normalized {
	case string(auth.PermissionArtifactTypeServer):
		return auth.PermissionArtifactTypeServer, normalized, nil
	case string(auth.PermissionArtifactTypeAgent):
		return auth.PermissionArtifactTypeAgent, normalized, nil
	case string(auth.PermissionArtifactTypeSkill):
		return auth.PermissionArtifactTypeSkill, normalized, nil
	case string(auth.PermissionArtifactTypePrompt):
		return auth.PermissionArtifactTypePrompt, normalized, nil
	default:
		return "", "", fmt.Errorf("%w: unsupported artifact type %q", database.ErrInvalidInput, artifactType)
	}
}

func (s *registryServiceImpl) ensureReviewArtifactExists(
	ctx context.Context,
	tx pgx.Tx,
	artifactType,
	artifactName,
	artifactVersion string,
) error {
	switch artifactType {
	case string(auth.PermissionArtifactTypeServer):
		_, err := s.db.GetServerByNameAndVersion(ctx, tx, artifactName, artifactVersion)
		return err
	case string(auth.PermissionArtifactTypeAgent):
		_, err := s.db.GetAgentByNameAndVersion(ctx, tx, artifactName, artifactVersion)
		return err
	case string(auth.PermissionArtifactTypeSkill):
		_, err := s.db.GetSkillByNameAndVersion(ctx, tx, artifactName, artifactVersion)
		return err
	case string(auth.PermissionArtifactTypePrompt):
		_, err := s.db.GetPromptByNameAndVersion(ctx, tx, artifactName, artifactVersion)
		return err
	default:
		return fmt.Errorf("%w: unsupported artifact type %q", database.ErrInvalidInput, artifactType)
	}
}
