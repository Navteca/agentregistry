package service

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
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
	if reviewConfig.OverrideOutcome() != "" && outcome == reviewConfig.OverrideOutcome() {
		return nil, fmt.Errorf("%w: override outcome requires the override endpoint", database.ErrInvalidInput)
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

func (s *registryServiceImpl) CreateReviewOverride(
	ctx context.Context,
	artifactType,
	artifactName,
	artifactVersion string,
	targetReviewID int64,
	reason string,
) (*models.Review, error) {
	resourceType, normalizedArtifactType, err := reviewResourceType(artifactType)
	if err != nil {
		return nil, err
	}
	if s.cfg == nil {
		return nil, fmt.Errorf("%w: configuration is unavailable", database.ErrInvalidInput)
	}
	reviewConfig := s.cfg.ReviewConfig()
	if reviewConfig.OverrideOutcome() == "" ||
		reviewConfig.OverrideOutcome() == reviewConfig.FailureOutcome() {
		return nil, fmt.Errorf("%w: invalid review override configuration", database.ErrInvalidInput)
	}
	if strings.TrimSpace(reason) == "" {
		return nil, fmt.Errorf("%w: override reason must not be empty", database.ErrInvalidInput)
	}
	session, ok := auth.AuthSessionFrom(ctx)
	if !ok || session == nil {
		return nil, auth.ErrUnauthenticated
	}
	if !s.canPerform(ctx, artifactName, resourceType, auth.PermissionActionOverride) {
		return nil, auth.ErrForbidden
	}
	user := session.Principal().User

	return database.InTransactionT(ctx, s.db, func(ctx context.Context, tx pgx.Tx) (*models.Review, error) {
		if err := s.ensureReviewArtifactExists(ctx, tx, normalizedArtifactType, artifactName, artifactVersion); err != nil {
			return nil, err
		}
		target, err := s.db.GetReviewByID(ctx, tx, targetReviewID)
		if err != nil {
			return nil, err
		}
		if target.ArtifactType != normalizedArtifactType ||
			target.ArtifactName != artifactName ||
			target.ArtifactVersion != artifactVersion {
			return nil, fmt.Errorf("%w: override target belongs to a different artifact", database.ErrInvalidInput)
		}
		if target.IsOverride() {
			return nil, fmt.Errorf("%w: override target is already an override", database.ErrInvalidInput)
		}
		if target.Outcome != reviewConfig.FailureOutcome() {
			return nil, fmt.Errorf("%w: override target is not a failure", database.ErrInvalidInput)
		}

		override := &models.Review{
			ArtifactType:        normalizedArtifactType,
			ArtifactName:        artifactName,
			ArtifactVersion:     artifactVersion,
			ReviewType:          target.ReviewType,
			Outcome:             reviewConfig.OverrideOutcome(),
			ReviewerSubject:     user.Subject,
			ReviewerAuthMethod:  string(user.AuthMethod),
			ReviewerDisplayName: user.DisplayName,
			Notes:               strings.TrimSpace(reason),
			OverridesReviewID:   &targetReviewID,
		}
		return s.db.CreateReview(ctx, tx, override)
	})
}

func (s *registryServiceImpl) GetReviewState(
	ctx context.Context,
	artifactType,
	artifactName,
	artifactVersion string,
) (*models.ReviewState, error) {
	_, normalizedArtifactType, err := reviewResourceType(artifactType)
	if err != nil {
		return nil, err
	}
	if s.cfg == nil {
		return nil, fmt.Errorf("%w: configuration is unavailable", database.ErrInvalidInput)
	}

	return database.InTransactionT(ctx, s.db, func(ctx context.Context, tx pgx.Tx) (*models.ReviewState, error) {
		updatedAt, err := s.reviewArtifactUpdatedAt(ctx, tx, normalizedArtifactType, artifactName, artifactVersion)
		if err != nil {
			return nil, err
		}
		return s.reviewStateForUpdatedAt(
			ctx,
			tx,
			auth.PermissionArtifactType(normalizedArtifactType),
			artifactName,
			artifactVersion,
			updatedAt,
		)
	})
}

func (s *registryServiceImpl) GetReviews(
	ctx context.Context,
	artifactType,
	artifactName,
	artifactVersion string,
) ([]models.Review, error) {
	_, normalizedArtifactType, err := reviewResourceType(artifactType)
	if err != nil {
		return nil, err
	}

	return database.InTransactionT(ctx, s.db, func(ctx context.Context, tx pgx.Tx) ([]models.Review, error) {
		updatedAt, err := s.reviewArtifactUpdatedAt(ctx, tx, normalizedArtifactType, artifactName, artifactVersion)
		if err != nil {
			return nil, err
		}
		reviews, err := s.db.ListReviews(ctx, tx, normalizedArtifactType, artifactName, artifactVersion)
		if err != nil {
			return nil, err
		}
		if reviews == nil {
			reviews = make([]models.Review, 0)
		}
		resolved := resolveReviewRows(reviews, updatedAt)
		for i := range reviews {
			current := resolved[i].current
			stale := resolved[i].stale
			superseded := resolved[i].superseded
			reviews[i].IsCurrent = &current
			reviews[i].IsStale = &stale
			reviews[i].IsSuperseded = &superseded
		}
		return reviews, nil
	})
}

func (s *registryServiceImpl) reviewStateForUpdatedAt(
	ctx context.Context,
	tx pgx.Tx,
	artifactType auth.PermissionArtifactType,
	artifactName,
	artifactVersion string,
	updatedAt time.Time,
) (*models.ReviewState, error) {
	reviews, err := s.db.ListReviews(ctx, tx, string(artifactType), artifactName, artifactVersion)
	if err != nil {
		return nil, err
	}

	state := ResolveReviewState(reviews, updatedAt, s.cfg.ReviewConfig())
	return &state, nil
}

func (s *registryServiceImpl) reviewArtifactUpdatedAt(
	ctx context.Context,
	tx pgx.Tx,
	artifactType,
	artifactName,
	artifactVersion string,
) (time.Time, error) {
	switch artifactType {
	case string(auth.PermissionArtifactTypeServer):
		response, err := s.db.GetServerByNameAndVersion(ctx, tx, artifactName, artifactVersion)
		if err != nil {
			return time.Time{}, err
		}
		if response.Meta.Official == nil {
			return time.Time{}, fmt.Errorf("%w: artifact has no official metadata", database.ErrInvalidInput)
		}
		return response.Meta.Official.UpdatedAt, nil
	case string(auth.PermissionArtifactTypeAgent):
		response, err := s.db.GetAgentByNameAndVersion(ctx, tx, artifactName, artifactVersion)
		if err != nil {
			return time.Time{}, err
		}
		if response.Meta.Official == nil {
			return time.Time{}, fmt.Errorf("%w: artifact has no official metadata", database.ErrInvalidInput)
		}
		return response.Meta.Official.UpdatedAt, nil
	case string(auth.PermissionArtifactTypeSkill):
		response, err := s.db.GetSkillByNameAndVersion(ctx, tx, artifactName, artifactVersion)
		if err != nil {
			return time.Time{}, err
		}
		if response.Meta.Official == nil {
			return time.Time{}, fmt.Errorf("%w: artifact has no official metadata", database.ErrInvalidInput)
		}
		return response.Meta.Official.UpdatedAt, nil
	case string(auth.PermissionArtifactTypePrompt):
		response, err := s.db.GetPromptByNameAndVersion(ctx, tx, artifactName, artifactVersion)
		if err != nil {
			return time.Time{}, err
		}
		if response.Meta.Official == nil {
			return time.Time{}, fmt.Errorf("%w: artifact has no official metadata", database.ErrInvalidInput)
		}
		return response.Meta.Official.UpdatedAt, nil
	default:
		return time.Time{}, fmt.Errorf("%w: unsupported artifact type %q", database.ErrInvalidInput, artifactType)
	}
}

// ResolveCurrentReviews applies the single current-review definition:
// discard rows predating updatedAt, then retain the latest row per artifact,
// review type, and reviewer subject.
func ResolveCurrentReviews(reviews []models.Review, updatedAt time.Time) []models.Review {
	resolved := resolveReviewRows(reviews, updatedAt)
	current := make([]models.Review, 0, len(resolved))
	for _, row := range resolved {
		if row.current {
			current = append(current, row.review)
		}
	}
	slices.SortFunc(current, compareReviews)
	return current
}

type reviewResolution struct {
	review     models.Review
	current    bool
	stale      bool
	superseded bool
}

func resolveReviewRows(reviews []models.Review, updatedAt time.Time) []reviewResolution {
	resolved := make([]reviewResolution, len(reviews))
	latestByKey := make(map[string]int, len(reviews))
	for i, review := range reviews {
		stale := review.CreatedAt.Before(updatedAt)
		resolved[i] = reviewResolution{
			review:  review,
			current: !stale,
			stale:   stale,
		}

		key := reviewResolutionKey(review)
		latestIndex, exists := latestByKey[key]
		if !exists {
			latestByKey[key] = i
			continue
		}
		if !reviewIsLater(review, resolved[latestIndex].review) {
			resolved[i].current = false
			resolved[i].superseded = true
			continue
		}

		resolved[latestIndex].current = false
		resolved[latestIndex].superseded = true
		latestByKey[key] = i
	}
	return resolved
}

func reviewResolutionKey(review models.Review) string {
	return strings.Join([]string{
		review.ArtifactType,
		review.ArtifactName,
		review.ArtifactVersion,
		review.ReviewType,
		review.ReviewerSubject,
	}, "\x00")
}

// ResolveReviewState derives certification, rejection, and per-type status
// from the shared current-review resolution.
func ResolveReviewState(
	reviews []models.Review,
	updatedAt time.Time,
	reviewConfig config.ReviewSettings,
) models.ReviewState {
	current := ResolveCurrentReviews(reviews, updatedAt)
	byType := make(map[string][]models.Review)
	byID := make(map[int64]models.Review, len(current))
	overrideTargets := make(map[int64]struct{})
	for _, review := range current {
		byID[review.ID] = review
		if review.IsOverride() {
			if review.OverridesReviewID != nil {
				overrideTargets[*review.OverridesReviewID] = struct{}{}
			}
		}
	}
	rejected := false
	failureOutcome := reviewConfig.FailureOutcome()
	for _, review := range current {
		byType[review.ReviewType] = append(byType[review.ReviewType], review)
		if review.Outcome == failureOutcome {
			if _, overridden := overrideTargets[review.ID]; overridden {
				continue
			}
			rejected = true
		}
	}

	configuredTypes := reviewConfig.Types()
	perType := make([]models.ReviewTypeStatus, 0, len(configuredTypes))
	allTypesReviewed := len(configuredTypes) > 0
	for _, reviewType := range configuredTypes {
		typeReviews := byType[reviewType]
		slices.SortFunc(typeReviews, compareReviews)
		status := models.ReviewStatusPending
		outcome := ""
		if len(typeReviews) == 0 {
			allTypesReviewed = false
		} else {
			latestTypeReview := typeReviews[0]
			hasFailure := false
			hasOverride := false
			for _, review := range typeReviews {
				if reviewIsLater(review, latestTypeReview) {
					latestTypeReview = review
				}
				if review.Outcome == failureOutcome {
					if _, overridden := overrideTargets[review.ID]; !overridden {
						hasFailure = true
					}
				}
				if review.IsOverride() {
					if targetID := review.OverridesReviewID; targetID != nil {
						if _, targetCurrent := byID[*targetID]; targetCurrent {
							hasOverride = true
						}
					}
				}
			}
			outcome = latestTypeReview.Outcome
			if hasFailure {
				status = models.ReviewStatusFail
			} else if hasOverride {
				status = models.ReviewStatusOverridden
				outcome = reviewConfig.OverrideOutcome()
			} else {
				status = models.ReviewStatusPass
			}
		}
		perType = append(perType, models.ReviewTypeStatus{
			ReviewType:     reviewType,
			Status:         status,
			Outcome:        outcome,
			CurrentReviews: slices.Clone(typeReviews),
		})
	}

	certified := allTypesReviewed && !rejected
	return models.ReviewState{
		Certified:      certified,
		Rejected:       rejected,
		Pending:        !certified && !rejected,
		CurrentReviews: current,
		PerType:        perType,
	}
}

// SummarizeReviewState removes findings and sensitive reviewer identity from
// derived state before it is attached to an artifact response.
func SummarizeReviewState(state models.ReviewState) models.ReviewSummary {
	status := models.ReviewStatusPending
	if state.Certified {
		status = "certified"
	} else if state.Rejected {
		status = "rejected"
	}

	perType := make([]models.ReviewTypeSummary, 0, len(state.PerType))
	for _, typeState := range state.PerType {
		displayNames := make([]string, 0, len(typeState.CurrentReviews))
		for _, review := range typeState.CurrentReviews {
			if review.ReviewerDisplayName != "" {
				displayNames = append(displayNames, review.ReviewerDisplayName)
			}
		}
		perType = append(perType, models.ReviewTypeSummary{
			ReviewType:           typeState.ReviewType,
			Status:               typeState.Status,
			Outcome:              typeState.Outcome,
			ReviewerDisplayNames: displayNames,
		})
	}
	return models.ReviewSummary{Status: status, PerType: perType}
}

func reviewIsLater(candidate, current models.Review) bool {
	if candidate.CreatedAt.After(current.CreatedAt) {
		return true
	}
	return candidate.CreatedAt.Equal(current.CreatedAt) && candidate.ID > current.ID
}

func compareReviews(a, b models.Review) int {
	if result := cmp.Compare(a.ReviewType, b.ReviewType); result != 0 {
		return result
	}
	if result := cmp.Compare(a.ReviewerSubject, b.ReviewerSubject); result != 0 {
		return result
	}
	if a.CreatedAt.After(b.CreatedAt) {
		return -1
	}
	if a.CreatedAt.Before(b.CreatedAt) {
		return 1
	}
	return cmp.Compare(b.ID, a.ID)
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
