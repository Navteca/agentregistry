package v0

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	"github.com/agentregistry-dev/agentregistry/internal/registry/service"
	"github.com/agentregistry-dev/agentregistry/pkg/models"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/database"
	"github.com/agentregistry-dev/agentregistry/pkg/types"
	"github.com/danielgtaylor/huma/v2"
)

// CreateReviewInput represents a review attached to one artifact version.
type CreateReviewInput struct {
	ArtifactType    string           `path:"artifactType" json:"artifactType"`
	ArtifactName    string           `path:"artifactName" json:"artifactName"`
	ArtifactVersion string           `path:"version" json:"version"`
	Body            CreateReviewBody `body:""`
}

// ListReviewsInput identifies one artifact version whose review rows are read.
type ListReviewsInput struct {
	ArtifactType    string `path:"artifactType" json:"artifactType"`
	ArtifactName    string `path:"artifactName" json:"artifactName"`
	ArtifactVersion string `path:"version" json:"version"`
}

// CreateReviewBody contains client-supplied review content. Optional reviewer
// identity fields are accepted for compatibility but always ignored; identity
// is taken from the authenticated session.
type CreateReviewBody struct {
	ReviewType          string `json:"review_type"`
	Outcome             string `json:"outcome"`
	Notes               string `json:"notes"`
	ReviewerSubject     string `json:"reviewer_subject,omitempty"`
	ReviewerAuthMethod  string `json:"reviewer_auth_method,omitempty"`
	ReviewerDisplayName string `json:"reviewer_display_name,omitempty"`
}

// RegisterReviewsEndpoint registers review read and create operations.
func RegisterReviewsEndpoint(api huma.API, pathPrefix string, cfg *config.Config, registry service.RegistryService) {
	huma.Register(api, huma.Operation{
		OperationID: "list-reviews" + strings.ReplaceAll(pathPrefix, "/", "-"),
		Method:      http.MethodGet,
		Path:        pathPrefix + "/reviews/{artifactType}/{artifactName}/versions/{version}",
		Summary:     "List artifact reviews",
		Description: "List all reviews for a specific artifact version, including current and stale markers.",
		Tags:        []string{"reviews"},
	}, func(ctx context.Context, input *ListReviewsInput) (*types.Response[[]models.Review], error) {
		artifactType, err := url.PathUnescape(input.ArtifactType)
		if err != nil {
			return nil, huma.Error400BadRequest("Invalid artifact type encoding", err)
		}
		artifactName, err := url.PathUnescape(input.ArtifactName)
		if err != nil {
			return nil, huma.Error400BadRequest("Invalid artifact name encoding", err)
		}
		version, err := url.PathUnescape(input.ArtifactVersion)
		if err != nil {
			return nil, huma.Error400BadRequest("Invalid artifact version encoding", err)
		}

		reviews, err := registry.GetReviews(ctx, artifactType, artifactName, version)
		if err != nil {
			switch {
			case errors.Is(err, database.ErrInvalidInput):
				return nil, huma.Error400BadRequest("Invalid review request", err)
			case errors.Is(err, database.ErrNotFound):
				return nil, huma.Error404NotFound("Artifact version not found")
			case errors.Is(err, auth.ErrUnauthenticated):
				return nil, huma.Error401Unauthorized("Authentication required")
			case errors.Is(err, auth.ErrForbidden):
				return nil, huma.Error403Forbidden("Forbidden")
			default:
				return nil, huma.Error500InternalServerError("Failed to list reviews", err)
			}
		}

		return &types.Response[[]models.Review]{Body: reviews}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-review" + strings.ReplaceAll(pathPrefix, "/", "-"),
		Method:      http.MethodPost,
		Path:        pathPrefix + "/reviews/{artifactType}/{artifactName}/versions/{version}",
		Summary:     "Create an artifact review",
		Description: "Record an append-only review for a specific artifact version.",
		Tags:        []string{"reviews"},
	}, func(ctx context.Context, input *CreateReviewInput) (*types.Response[models.Review], error) {
		artifactType, err := url.PathUnescape(input.ArtifactType)
		if err != nil {
			return nil, huma.Error400BadRequest("Invalid artifact type encoding", err)
		}
		artifactName, err := url.PathUnescape(input.ArtifactName)
		if err != nil {
			return nil, huma.Error400BadRequest("Invalid artifact name encoding", err)
		}
		version, err := url.PathUnescape(input.ArtifactVersion)
		if err != nil {
			return nil, huma.Error400BadRequest("Invalid artifact version encoding", err)
		}

		if cfg == nil {
			return nil, huma.Error500InternalServerError("Review configuration is unavailable", nil)
		}
		reviewConfig := cfg.ReviewConfig()
		if !reviewConfig.HasType(input.Body.ReviewType) {
			return nil, huma.Error400BadRequest("Unconfigured review type", nil)
		}
		if !reviewConfig.HasOutcome(input.Body.Outcome) {
			return nil, huma.Error400BadRequest("Unconfigured review outcome", nil)
		}

		review, err := registry.CreateReview(
			ctx,
			artifactType,
			artifactName,
			version,
			input.Body.ReviewType,
			input.Body.Outcome,
			input.Body.Notes,
		)
		if err != nil {
			switch {
			case errors.Is(err, database.ErrInvalidInput):
				return nil, huma.Error400BadRequest("Invalid review request", err)
			case errors.Is(err, database.ErrNotFound):
				return nil, huma.Error404NotFound("Artifact version not found")
			case errors.Is(err, auth.ErrUnauthenticated):
				return nil, huma.Error401Unauthorized("Authentication required")
			case errors.Is(err, auth.ErrForbidden):
				return nil, huma.Error403Forbidden("Forbidden")
			default:
				return nil, huma.Error500InternalServerError("Failed to create review", err)
			}
		}

		return &types.Response[models.Review]{Body: *review}, nil
	})
}
