package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	internaldb "github.com/agentregistry-dev/agentregistry/internal/registry/database"
	"github.com/agentregistry-dev/agentregistry/pkg/models"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/database"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateReview_PersistsIdentityAndAppendsRevisions(t *testing.T) {
	db := internaldb.NewTestDB(t)
	cfg := &config.Config{
		EnableRegistryValidation: false,
		ReviewTypes:              []string{"security", "scientific"},
		ReviewOutcomes:           []string{"pass", "fail"},
		ReviewFailureOutcome:     "fail",
	}
	registry := NewRegistryService(db, cfg, nil, realCapabilityTestAuthorizer(t))
	server := newOwnershipTestServer("review-target")
	ownerCtx := ownershipPermissionContext("owner-subject", "Owner", ownerEditPermissions())

	created, err := registry.CreateServer(ownerCtx, server)
	require.NoError(t, err)
	require.NotNil(t, created)

	before, err := registry.GetServerByNameAndVersion(ownerCtx, server.Name, server.Version)
	require.NoError(t, err)

	curatorPermissions := []auth.Permission{
		{Action: auth.PermissionActionRead, ResourcePattern: "*"},
		{Action: auth.PermissionActionReview, ResourcePattern: "*"},
	}
	first, err := registry.CreateReview(
		ownershipPermissionContext("curator-one", "Curator One", curatorPermissions),
		"server",
		server.Name,
		server.Version,
		"security",
		"fail",
		"first finding",
	)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, "curator-one", first.ReviewerSubject)
	assert.Equal(t, "oidc", first.ReviewerAuthMethod)
	assert.Equal(t, "Curator One", first.ReviewerDisplayName)
	assert.Equal(t, "first finding", first.Notes)

	second, err := registry.CreateReview(
		ownershipPermissionContext("curator-two", "Curator Two", curatorPermissions),
		"server",
		server.Name,
		server.Version,
		"security",
		"pass",
		"independent finding",
	)
	require.NoError(t, err)
	require.NotNil(t, second)

	revision, err := registry.CreateReview(
		ownershipPermissionContext("curator-one", "Curator One", curatorPermissions),
		"server",
		server.Name,
		server.Version,
		"security",
		"pass",
		"revised finding",
	)
	require.NoError(t, err)
	require.NotNil(t, revision)
	assert.NotEqual(t, first.ID, second.ID)
	assert.NotEqual(t, first.ID, revision.ID)
	assert.NotEqual(t, second.ID, revision.ID)
	assert.Equal(t, "pass", revision.Outcome)
	assert.Equal(t, "revised finding", revision.Notes)

	after, err := registry.GetServerByNameAndVersion(ownerCtx, server.Name, server.Version)
	require.NoError(t, err)
	assert.Equal(t, before.Meta.Official.UpdatedAt, after.Meta.Official.UpdatedAt)
}

func TestCreateReview_UserIsForbiddenByReviewPermission(t *testing.T) {
	db := internaldb.NewTestDB(t)
	cfg := &config.Config{
		EnableRegistryValidation: false,
		ReviewTypes:              []string{"security"},
		ReviewOutcomes:           []string{"pass", "fail"},
		ReviewFailureOutcome:     "fail",
	}
	registry := NewRegistryService(db, cfg, nil, realCapabilityTestAuthorizer(t))
	server := newOwnershipTestServer("review-forbidden")
	_, err := registry.CreateServer(
		ownershipPermissionContext("owner-subject", "Owner", ownerEditPermissions()),
		server,
	)
	require.NoError(t, err)

	_, err = registry.CreateReview(
		ownershipPermissionContext("user-subject", "User", ownerEditPermissions()),
		"server",
		server.Name,
		server.Version,
		"security",
		"pass",
		"should not persist",
	)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrForbidden))
}

func TestCreateReview_RejectsUnconfiguredValues(t *testing.T) {
	db := internaldb.NewTestDB(t)
	cfg := &config.Config{
		EnableRegistryValidation: false,
		ReviewTypes:              []string{"security"},
		ReviewOutcomes:           []string{"pass"},
		ReviewFailureOutcome:     "fail",
	}
	registry := NewRegistryService(db, cfg, nil, realCapabilityTestAuthorizer(t))

	_, err := registry.CreateReview(
		ownershipPermissionContext("curator-subject", "Curator", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionReview, ResourcePattern: "*"},
		}),
		"server",
		"com.example/missing",
		"1.0.0",
		"scientific",
		"pass",
		"not written",
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, database.ErrInvalidInput)
}

func TestCreateReview_RequiresExistingArtifactVersion(t *testing.T) {
	db := internaldb.NewTestDB(t)
	cfg := &config.Config{
		EnableRegistryValidation: false,
		ReviewTypes:              []string{"security"},
		ReviewOutcomes:           []string{"pass"},
		ReviewFailureOutcome:     "fail",
	}
	registry := NewRegistryService(db, cfg, nil, realCapabilityTestAuthorizer(t))

	_, err := registry.CreateReview(
		ownershipPermissionContext("curator-subject", "Curator", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionReview, ResourcePattern: "*"},
		}),
		"server",
		"com.example/missing",
		"1.0.0",
		"security",
		"pass",
		"not written",
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, database.ErrNotFound)
}

func TestResolveReviewState_DerivesConfiguredStatuses(t *testing.T) {
	updatedAt := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	settings := (&config.Config{
		ReviewTypes:          []string{"security", "scientific"},
		ReviewOutcomes:       []string{"pass", "fail"},
		ReviewFailureOutcome: "fail",
	}).ReviewConfig()

	tests := []struct {
		name           string
		reviews        []models.Review
		certified      bool
		rejected       bool
		pending        bool
		expectedStatus []string
	}{
		{
			name:           "no reviews",
			expectedStatus: []string{"pending", "pending"},
			pending:        true,
		},
		{
			name: "partial review",
			reviews: []models.Review{
				testReview(1, "security", "pass", "reviewer", updatedAt),
			},
			expectedStatus: []string{"pass", "pending"},
			pending:        true,
		},
		{
			name: "all pass",
			reviews: []models.Review{
				testReview(1, "security", "pass", "reviewer", updatedAt),
				testReview(2, "scientific", "pass", "reviewer", updatedAt),
			},
			expectedStatus: []string{"pass", "pass"},
			certified:      true,
		},
		{
			name: "one fail rejects",
			reviews: []models.Review{
				testReview(1, "security", "fail", "reviewer", updatedAt),
				testReview(2, "scientific", "pass", "reviewer", updatedAt),
			},
			expectedStatus: []string{"fail", "pass"},
			rejected:       true,
		},
		{
			name: "independent current reviewers",
			reviews: []models.Review{
				testReview(1, "security", "pass", "reviewer-one", updatedAt),
				testReview(2, "security", "fail", "reviewer-two", updatedAt),
				testReview(3, "scientific", "pass", "reviewer-one", updatedAt),
			},
			expectedStatus: []string{"fail", "pass"},
			rejected:       true,
		},
		{
			name: "later revision wins",
			reviews: []models.Review{
				testReview(1, "security", "pass", "reviewer", updatedAt),
				testReview(2, "security", "fail", "reviewer", updatedAt.Add(time.Second)),
				testReview(3, "scientific", "pass", "reviewer", updatedAt),
			},
			expectedStatus: []string{"fail", "pass"},
			rejected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := ResolveReviewState(tt.reviews, updatedAt, settings)
			assert.Equal(t, tt.certified, state.Certified)
			assert.Equal(t, tt.rejected, state.Rejected)
			assert.Equal(t, tt.pending, state.Pending)
			require.Len(t, state.PerType, len(tt.expectedStatus))
			for i, expected := range tt.expectedStatus {
				assert.Equal(t, expected, state.PerType[i].Status)
			}
		})
	}
}

func TestResolveCurrentReviews_UsesStalenessAndIDTieBreak(t *testing.T) {
	updatedAt := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	reviews := []models.Review{
		testReview(1, "security", "pass", "reviewer", updatedAt.Add(-time.Nanosecond)),
		testReview(2, "security", "fail", "reviewer", updatedAt),
		testReview(3, "security", "pass", "reviewer", updatedAt),
	}

	current := ResolveCurrentReviews(reviews, updatedAt)
	require.Len(t, current, 1)
	assert.Equal(t, int64(3), current[0].ID)
	assert.Equal(t, "pass", current[0].Outcome)
}

func TestResolveReviewState_NewConfiguredTypeIsPending(t *testing.T) {
	updatedAt := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	cfg := &config.Config{
		ReviewTypes:          []string{"security"},
		ReviewOutcomes:       []string{"pass", "fail"},
		ReviewFailureOutcome: "fail",
	}
	state := ResolveReviewState(
		[]models.Review{testReview(1, "security", "pass", "reviewer", updatedAt)},
		updatedAt,
		cfg.ReviewConfig(),
	)
	require.True(t, state.Certified)

	cfg.ReviewTypes = []string{"security", "scientific"}
	state = ResolveReviewState(
		[]models.Review{testReview(1, "security", "pass", "reviewer", updatedAt)},
		updatedAt,
		cfg.ReviewConfig(),
	)
	assert.False(t, state.Certified)
	assert.True(t, state.Pending)
	assert.Equal(t, models.ReviewStatusPending, state.PerType[1].Status)
}

func TestResolveReviewState_RevisionRecomputesBothDirections(t *testing.T) {
	updatedAt := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	settings := (&config.Config{
		ReviewTypes:          []string{"security"},
		ReviewOutcomes:       []string{"pass", "fail"},
		ReviewFailureOutcome: "fail",
	}).ReviewConfig()

	state := ResolveReviewState([]models.Review{
		testReview(1, "security", "pass", "reviewer", updatedAt),
		testReview(2, "security", "fail", "reviewer", updatedAt.Add(time.Second)),
	}, updatedAt, settings)
	assert.True(t, state.Rejected)
	assert.False(t, state.Certified)

	state = ResolveReviewState([]models.Review{
		testReview(3, "security", "fail", "reviewer", updatedAt),
		testReview(4, "security", "pass", "reviewer", updatedAt.Add(time.Second)),
	}, updatedAt, settings)
	assert.False(t, state.Rejected)
	assert.True(t, state.Certified)
}

func TestResolveReviewState_MapsConfiguredOutcomesToStableStatuses(t *testing.T) {
	updatedAt := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	settings := (&config.Config{
		ReviewTypes:          []string{"security"},
		ReviewOutcomes:       []string{"approved", "reject"},
		ReviewFailureOutcome: "reject",
	}).ReviewConfig()

	state := ResolveReviewState(
		[]models.Review{testReview(1, "security", "approved", "reviewer", updatedAt)},
		updatedAt,
		settings,
	)
	assert.True(t, state.Certified)
	assert.Equal(t, models.ReviewStatusPass, state.PerType[0].Status)
	assert.Equal(t, "approved", state.PerType[0].Outcome)

	state = ResolveReviewState(
		[]models.Review{testReview(2, "security", "reject", "reviewer", updatedAt)},
		updatedAt,
		settings,
	)
	assert.True(t, state.Rejected)
	assert.Equal(t, models.ReviewStatusFail, state.PerType[0].Status)
	assert.Equal(t, "reject", state.PerType[0].Outcome)
}

func TestReviewStateSequence_EditMakesBothPassesStale(t *testing.T) {
	registry, ownerCtx, server := newReviewStateFixture(t, "review-state-stale")
	createStateReview(t, registry, server, "security", "pass", "security-review")
	createStateReview(t, registry, server, "scientific", "pass", "scientific-review")

	state, err := registry.GetReviewState(ownerCtx, "server", server.Name, server.Version)
	require.NoError(t, err)
	require.True(t, state.Certified)

	updateServerForReviewState(t, registry, ownerCtx, server, "edited content")
	state, err = registry.GetReviewState(ownerCtx, "server", server.Name, server.Version)
	require.NoError(t, err)
	assert.False(t, state.Certified)
	assert.False(t, state.Rejected)
	assert.True(t, state.Pending)
	assert.Equal(t, models.ReviewStatusPending, state.PerType[0].Status)
	assert.Equal(t, models.ReviewStatusPending, state.PerType[1].Status)
}

func TestReviewStateSequence_ReopenCannotRecertifyWithStaleSecurity(t *testing.T) {
	registry, ownerCtx, server := newReviewStateFixture(t, "review-state-hole")
	createStateReview(t, registry, server, "security", "pass", "security-review")
	createStateReview(t, registry, server, "scientific", "pass", "scientific-review")

	state, err := registry.GetReviewState(ownerCtx, "server", server.Name, server.Version)
	require.NoError(t, err)
	require.True(t, state.Certified)

	createStateReview(t, registry, server, "scientific", "fail", "scientific-failure")
	state, err = registry.GetReviewState(ownerCtx, "server", server.Name, server.Version)
	require.NoError(t, err)
	require.True(t, state.Rejected)

	updateServerForReviewState(t, registry, ownerCtx, server, "reopened content")
	createStateReview(t, registry, server, "scientific", "pass", "scientific-repair")

	state, err = registry.GetReviewState(ownerCtx, "server", server.Name, server.Version)
	require.NoError(t, err)
	assert.False(t, state.Certified)
	assert.False(t, state.Rejected)
	assert.True(t, state.Pending)
	assert.Equal(t, models.ReviewStatusPending, state.PerType[0].Status)
	assert.Equal(t, models.ReviewStatusPass, state.PerType[1].Status)
}

func testReview(id int64, reviewType, outcome, subject string, createdAt time.Time) models.Review {
	return models.Review{
		ID:              id,
		ArtifactType:    "server",
		ArtifactName:    "com.example/review-state",
		ArtifactVersion: "1.0.0",
		ReviewType:      reviewType,
		Outcome:         outcome,
		ReviewerSubject: subject,
		CreatedAt:       createdAt,
	}
}

func newReviewStateFixture(t *testing.T, name string) (RegistryService, context.Context, *apiv0.ServerJSON) {
	t.Helper()

	db := internaldb.NewTestDB(t)
	cfg := &config.Config{
		EnableRegistryValidation: false,
		ReviewTypes:              []string{"security", "scientific"},
		ReviewOutcomes:           []string{"pass", "fail"},
		ReviewFailureOutcome:     "fail",
	}
	registry := NewRegistryService(db, cfg, nil, realCapabilityTestAuthorizer(t))
	server := newOwnershipTestServer(name)
	ownerCtx := ownershipPermissionContext("review-owner", "Review Owner", ownerEditPermissions())
	_, err := registry.CreateServer(ownerCtx, server)
	require.NoError(t, err)
	updateServerForReviewState(t, registry, ownerCtx, server, "baseline content")
	return registry, ownerCtx, server
}

func createStateReview(t *testing.T, registry RegistryService, server *apiv0.ServerJSON, reviewType, outcome, notes string) {
	t.Helper()
	_, err := registry.CreateReview(
		ownershipPermissionContext("reviewer-"+reviewType, "Reviewer", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionReview, ResourcePattern: "*"},
		}),
		"server",
		server.Name,
		server.Version,
		reviewType,
		outcome,
		notes,
	)
	require.NoError(t, err)
}

func updateServerForReviewState(t *testing.T, registry RegistryService, ownerCtx context.Context, server *apiv0.ServerJSON, description string) {
	t.Helper()
	updated := *server
	updated.Description = description
	_, err := registry.UpdateServer(ownerCtx, server.Name, server.Version, &updated, nil)
	require.NoError(t, err)
}
