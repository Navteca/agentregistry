package service

import (
	"context"
	"encoding/json"
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

func TestCreateReviewOverride_PersistsAdminIdentityAndClearsFailure(t *testing.T) {
	db := internaldb.NewTestDB(t)
	cfg := &config.Config{
		EnableRegistryValidation: false,
		ReviewTypes:              []string{"security"},
		ReviewOutcomes:           []string{"pass", "fail", "override"},
		ReviewFailureOutcome:     "fail",
		ReviewOverrideOutcome:    "override",
	}
	registry := NewRegistryService(db, cfg, nil, realCapabilityTestAuthorizer(t))
	server := newOwnershipTestServer("review-override")
	ownerCtx := ownershipPermissionContext("owner-subject", "Owner", ownerEditPermissions())
	_, err := registry.CreateServer(ownerCtx, server)
	require.NoError(t, err)
	updateServerForReviewState(t, registry, ownerCtx, server, "baseline")

	target, err := registry.CreateReview(
		ownershipPermissionContext("reviewer-subject", "Reviewer", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionReview, ResourcePattern: "*"},
		}),
		"server", server.Name, server.Version, "security", "fail", "finding",
	)
	require.NoError(t, err)

	_, err = registry.CreateReviewOverride(
		ownershipPermissionContext("curator-subject", "Curator", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionReview, ResourcePattern: "*"},
		}),
		"server", server.Name, server.Version, target.ID, "curator override",
	)
	assert.ErrorIs(t, err, auth.ErrForbidden)

	override, err := registry.CreateReviewOverride(
		ownershipPermissionContext("admin-subject", "Admin", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionOverride, ResourcePattern: "*"},
		}),
		"server", server.Name, server.Version, target.ID, "accepted risk",
	)
	require.NoError(t, err)
	assert.True(t, override.IsOverride())
	assert.Equal(t, target.ID, *override.OverridesReviewID)
	assert.Equal(t, "admin-subject", override.ReviewerSubject)
	assert.Equal(t, "accepted risk", override.Notes)

	state, err := registry.GetReviewState(ownerCtx, "server", server.Name, server.Version)
	require.NoError(t, err)
	assert.True(t, state.Certified)
	assert.False(t, state.Rejected)
	assert.Equal(t, models.ReviewStatusOverridden, state.PerType[0].Status)

	reviews, err := registry.GetReviews(ownerCtx, "server", server.Name, server.Version)
	require.NoError(t, err)
	require.Len(t, reviews, 2)
	overrideRow := reviewWithNotes(t, reviews, "accepted risk")
	assert.True(t, overrideRow.IsOverride())
	assert.Equal(t, target.ID, *overrideRow.OverridesReviewID)

	_, err = registry.CreateReviewOverride(
		ownershipPermissionContext("admin-subject", "Admin", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionOverride, ResourcePattern: "*"},
		}),
		"server", server.Name, server.Version, override.ID, "nested override",
	)
	assert.ErrorIs(t, err, database.ErrInvalidInput)

	passReview, err := registry.CreateReview(
		ownershipPermissionContext("reviewer-subject", "Reviewer", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionReview, ResourcePattern: "*"},
		}),
		"server", server.Name, server.Version, "security", "pass", "no finding",
	)
	require.NoError(t, err)
	_, err = registry.CreateReviewOverride(
		ownershipPermissionContext("admin-subject", "Admin", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionOverride, ResourcePattern: "*"},
		}),
		"server", server.Name, server.Version, passReview.ID, "not a failure",
	)
	assert.ErrorIs(t, err, database.ErrInvalidInput)

	_, err = registry.CreateReviewOverride(
		ownershipPermissionContext("admin-subject", "Admin", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionOverride, ResourcePattern: "*"},
		}),
		"server", server.Name, server.Version, 999999999, "missing target",
	)
	assert.ErrorIs(t, err, database.ErrNotFound)

	otherServer := newOwnershipTestServer("review-override-other")
	_, err = registry.CreateServer(ownerCtx, otherServer)
	require.NoError(t, err)
	otherTarget, err := registry.CreateReview(
		ownershipPermissionContext("reviewer-subject", "Reviewer", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionReview, ResourcePattern: "*"},
		}),
		"server", otherServer.Name, otherServer.Version, "security", "fail", "other finding",
	)
	require.NoError(t, err)
	_, err = registry.CreateReviewOverride(
		ownershipPermissionContext("admin-subject", "Admin", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionOverride, ResourcePattern: "*"},
		}),
		"server", server.Name, server.Version, otherTarget.ID, "wrong artifact",
	)
	assert.ErrorIs(t, err, database.ErrInvalidInput)
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

func TestGetReviewsReturnsRowsAndAllowsReadOnlyCallers(t *testing.T) {
	db := internaldb.NewTestDB(t)
	cfg := &config.Config{
		EnableRegistryValidation: false,
		ReviewTypes:              []string{"security"},
		ReviewOutcomes:           []string{"pass", "fail"},
		ReviewFailureOutcome:     "fail",
	}
	registry := NewRegistryService(db, cfg, nil, realCapabilityTestAuthorizer(t))
	server := newOwnershipTestServer("review-read")
	ownerCtx := ownershipPermissionContext("review-owner", "Owner", ownerEditPermissions())
	_, err := registry.CreateServer(ownerCtx, server)
	require.NoError(t, err)
	updateServerForReviewState(t, registry, ownerCtx, server, "baseline")
	createStateReview(t, registry, server, "security", "pass", "visible finding")

	readerCtx := ownershipPermissionContext("read-only", "Reader", []auth.Permission{
		{Action: auth.PermissionActionRead, ResourcePattern: "*"},
	})
	reviews, err := registry.GetReviews(readerCtx, "server", server.Name, server.Version)
	require.NoError(t, err)
	require.Len(t, reviews, 1)
	assert.Equal(t, "visible finding", reviews[0].Notes)
	require.NotNil(t, reviews[0].IsCurrent)
	require.NotNil(t, reviews[0].IsStale)
	assert.True(t, *reviews[0].IsCurrent)
	assert.False(t, *reviews[0].IsStale)
}

func TestGetReviewsSetsSupersededMarkers(t *testing.T) {
	t.Run("single review is not superseded", func(t *testing.T) {
		registry, ownerCtx, server := newReviewStateFixture(t, "superseded-single")
		createReviewAs(t, registry, server, "reviewer-one", "Reviewer One", "security", "pass", "single")

		reviews, err := registry.GetReviews(ownerCtx, "server", server.Name, server.Version)
		require.NoError(t, err)
		require.Len(t, reviews, 1)
		assertReviewMarkers(t, reviews[0], true, false, false)
	})

	t.Run("revision supersedes the older review", func(t *testing.T) {
		registry, ownerCtx, server := newReviewStateFixture(t, "superseded-revision")
		createReviewAs(t, registry, server, "reviewer-one", "Reviewer One", "security", "pass", "older")
		createReviewAs(t, registry, server, "reviewer-one", "Reviewer One", "security", "fail", "newer")

		reviews, err := registry.GetReviews(ownerCtx, "server", server.Name, server.Version)
		require.NoError(t, err)
		require.Len(t, reviews, 2)
		assertReviewMarkers(t, reviewWithNotes(t, reviews, "older"), false, false, true)
		assertReviewMarkers(t, reviewWithNotes(t, reviews, "newer"), true, false, false)
	})

	t.Run("independent reviewers are not superseded", func(t *testing.T) {
		registry, ownerCtx, server := newReviewStateFixture(t, "superseded-independent")
		createReviewAs(t, registry, server, "reviewer-one", "Reviewer One", "security", "pass", "one")
		createReviewAs(t, registry, server, "reviewer-two", "Reviewer Two", "security", "fail", "two")

		reviews, err := registry.GetReviews(ownerCtx, "server", server.Name, server.Version)
		require.NoError(t, err)
		require.Len(t, reviews, 2)
		assertReviewMarkers(t, reviewWithNotes(t, reviews, "one"), true, false, false)
		assertReviewMarkers(t, reviewWithNotes(t, reviews, "two"), true, false, false)
	})

	t.Run("revised review can be both superseded and stale", func(t *testing.T) {
		registry, ownerCtx, server := newReviewStateFixture(t, "superseded-stale")
		createReviewAs(t, registry, server, "reviewer-one", "Reviewer One", "security", "pass", "older")
		updateServerForReviewState(t, registry, auth.WithSystemContext(context.Background()), server, "edited content")
		createReviewAs(t, registry, server, "reviewer-one", "Reviewer One", "security", "pass", "newer")

		reviews, err := registry.GetReviews(ownerCtx, "server", server.Name, server.Version)
		require.NoError(t, err)
		require.Len(t, reviews, 2)
		assertReviewMarkers(t, reviewWithNotes(t, reviews, "older"), false, true, true)
		assertReviewMarkers(t, reviewWithNotes(t, reviews, "newer"), true, false, false)
	})

	t.Run("stale review that was never revised is not superseded", func(t *testing.T) {
		registry, ownerCtx, server := newReviewStateFixture(t, "superseded-stale-only")
		createReviewAs(t, registry, server, "reviewer-one", "Reviewer One", "security", "pass", "stale")
		updateServerForReviewState(t, registry, auth.WithSystemContext(context.Background()), server, "edited content")

		reviews, err := registry.GetReviews(ownerCtx, "server", server.Name, server.Version)
		require.NoError(t, err)
		require.Len(t, reviews, 1)
		assertReviewMarkers(t, reviews[0], false, true, false)
	})

	t.Run("latest stale review is not superseded", func(t *testing.T) {
		registry, ownerCtx, server := newReviewStateFixture(t, "superseded-latest-stale")
		createReviewAs(t, registry, server, "reviewer-one", "Reviewer One", "security", "pass", "latest")
		updateServerForReviewState(t, registry, auth.WithSystemContext(context.Background()), server, "edited content")

		reviews, err := registry.GetReviews(ownerCtx, "server", server.Name, server.Version)
		require.NoError(t, err)
		require.Len(t, reviews, 1)
		assertReviewMarkers(t, reviews[0], false, true, false)
	})
}

func TestGetReviewsReturnsEmptyForExistingArtifactAndNotFoundForMissingVersion(t *testing.T) {
	db := internaldb.NewTestDB(t)
	cfg := &config.Config{
		EnableRegistryValidation: false,
		ReviewTypes:              []string{"security"},
		ReviewOutcomes:           []string{"pass"},
		ReviewFailureOutcome:     "fail",
	}
	registry := NewRegistryService(db, cfg, nil, realCapabilityTestAuthorizer(t))
	server := newOwnershipTestServer("review-empty")
	ctx := ownershipPermissionContext("reader", "Reader", []auth.Permission{
		{Action: auth.PermissionActionRead, ResourcePattern: "*"},
		{Action: auth.PermissionActionPublish, ResourcePattern: "*"},
	})
	_, err := registry.CreateServer(ctx, server)
	require.NoError(t, err)

	reviews, err := registry.GetReviews(ctx, "server", server.Name, server.Version)
	require.NoError(t, err)
	assert.NotNil(t, reviews)
	assert.Empty(t, reviews)

	_, err = registry.GetReviews(ctx, "server", server.Name, "9.9.9")
	assert.ErrorIs(t, err, database.ErrNotFound)
}

func TestArtifactReviewSummaryIsSanitizedJSON(t *testing.T) {
	db := internaldb.NewTestDB(t)
	cfg := &config.Config{
		EnableRegistryValidation: false,
		ReviewTypes:              []string{"security", "scientific"},
		ReviewOutcomes:           []string{"pass", "fail"},
		ReviewFailureOutcome:     "fail",
	}
	registry := NewRegistryService(db, cfg, nil, realCapabilityTestAuthorizer(t))
	server := newOwnershipTestServer("review-summary")
	ownerCtx := ownershipPermissionContext("owner", "Owner", ownerEditPermissions())
	_, err := registry.CreateServer(ownerCtx, server)
	require.NoError(t, err)
	updateServerForReviewState(t, registry, ownerCtx, server, "baseline")
	createStateReview(t, registry, server, "security", "pass", "do not expose this finding")
	createStateReview(t, registry, server, "scientific", "pass", "another private finding")

	readerCtx := ownershipPermissionContext("reader", "Reader", []auth.Permission{
		{Action: auth.PermissionActionRead, ResourcePattern: "*"},
	})
	response, err := registry.GetServerByNameAndVersion(readerCtx, server.Name, server.Version)
	require.NoError(t, err)
	require.NotNil(t, response.Meta.Review)
	assert.Equal(t, "certified", response.Meta.Review.Status)
	require.Len(t, response.Meta.Review.PerType, 2)

	serialized, err := json.Marshal(response)
	require.NoError(t, err)
	body := string(serialized)
	assert.NotContains(t, body, "do not expose this finding")
	assert.NotContains(t, body, "another private finding")
	assert.NotContains(t, body, `"reviewer_subject"`)
	assert.NotContains(t, body, `"reviewer_auth_method"`)

	var wire map[string]any
	require.NoError(t, json.Unmarshal(serialized, &wire))
	meta, ok := wire["_meta"].(map[string]any)
	require.True(t, ok)
	summary, ok := meta["aregistry.ai/review"].(map[string]any)
	require.True(t, ok)
	summaryJSON, err := json.Marshal(summary)
	require.NoError(t, err)
	assert.NotContains(t, string(summaryJSON), `"notes"`)
	assert.NotContains(t, string(summaryJSON), `"subject"`)
	assert.NotContains(t, string(summaryJSON), `"auth_method"`)
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
		overallStatus  string
	}{
		{
			name:           "no reviews",
			expectedStatus: []string{"pending", "pending"},
			pending:        true,
			overallStatus:  "pending",
		},
		{
			name: "partial review",
			reviews: []models.Review{
				testReview(1, "security", "pass", "reviewer", updatedAt),
			},
			expectedStatus: []string{"pass", "pending"},
			pending:        true,
			overallStatus:  "pending",
		},
		{
			name: "all pass",
			reviews: []models.Review{
				testReview(1, "security", "pass", "reviewer", updatedAt),
				testReview(2, "scientific", "pass", "reviewer", updatedAt),
			},
			expectedStatus: []string{"pass", "pass"},
			certified:      true,
			overallStatus:  "certified",
		},
		{
			name: "one fail rejects",
			reviews: []models.Review{
				testReview(1, "security", "fail", "reviewer", updatedAt),
				testReview(2, "scientific", "pass", "reviewer", updatedAt),
			},
			expectedStatus: []string{"fail", "pass"},
			rejected:       true,
			overallStatus:  "rejected",
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
			overallStatus:  "rejected",
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
			overallStatus:  "rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := ResolveReviewState(tt.reviews, updatedAt, settings)
			assert.Equal(t, tt.certified, state.Certified)
			assert.Equal(t, tt.rejected, state.Rejected)
			assert.Equal(t, tt.pending, state.Pending)
			assert.Equal(t, tt.overallStatus, SummarizeReviewState(state).Status)
			require.Len(t, state.PerType, len(tt.expectedStatus))
			for i, expected := range tt.expectedStatus {
				assert.Equal(t, expected, state.PerType[i].Status)
			}
		})
	}
}

func TestAllArtifactTypesCarryPendingReviewSummary(t *testing.T) {
	db := internaldb.NewTestDB(t)
	cfg := &config.Config{
		EnableRegistryValidation: false,
		ReviewTypes:              []string{"security"},
		ReviewOutcomes:           []string{"pass", "fail"},
		ReviewFailureOutcome:     "fail",
	}
	registry := NewRegistryService(db, cfg, nil, realCapabilityTestAuthorizer(t))
	ctx := ownershipPermissionContext("summary-owner", "Owner", ownerEditPermissions())

	server, err := registry.CreateServer(ctx, newOwnershipTestServer("summary-server"))
	require.NoError(t, err)
	agent, err := registry.CreateAgent(ctx, newOwnershipTestAgent("summary-agent"))
	require.NoError(t, err)
	skill, err := registry.CreateSkill(ctx, newOwnershipTestSkill("summary-skill"))
	require.NoError(t, err)
	prompt, err := registry.CreatePrompt(ctx, newOwnershipTestPrompt("summary-prompt", "1.0.0"))
	require.NoError(t, err)

	require.NotNil(t, server.Meta.Review)
	assert.Equal(t, "pending", server.Meta.Review.Status)
	require.Len(t, server.Meta.Review.PerType, 1)
	assert.Equal(t, "pending", server.Meta.Review.PerType[0].Status)
	require.NotNil(t, agent.Meta.Review)
	assert.Equal(t, "pending", agent.Meta.Review.Status)
	require.Len(t, agent.Meta.Review.PerType, 1)
	assert.Equal(t, "pending", agent.Meta.Review.PerType[0].Status)
	require.NotNil(t, skill.Meta.Review)
	assert.Equal(t, "pending", skill.Meta.Review.Status)
	require.Len(t, skill.Meta.Review.PerType, 1)
	assert.Equal(t, "pending", skill.Meta.Review.PerType[0].Status)
	require.NotNil(t, prompt.Meta.Review)
	assert.Equal(t, "pending", prompt.Meta.Review.Status)
	require.Len(t, prompt.Meta.Review.PerType, 1)
	assert.Equal(t, "pending", prompt.Meta.Review.PerType[0].Status)
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

func TestResolveReviewState_OverrideClearsCurrentFailure(t *testing.T) {
	updatedAt := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	targetID := int64(1)
	settings := (&config.Config{
		ReviewTypes:           []string{"security"},
		ReviewOutcomes:        []string{"pass", "fail", "override"},
		ReviewFailureOutcome:  "fail",
		ReviewOverrideOutcome: "override",
	}).ReviewConfig()

	state := ResolveReviewState([]models.Review{
		testReview(targetID, "security", "fail", "reviewer", updatedAt),
		{
			ID:                2,
			ArtifactType:      "server",
			ArtifactName:      "com.example/review-state",
			ArtifactVersion:   "1.0.0",
			ReviewType:        "security",
			Outcome:           "override",
			ReviewerSubject:   "admin",
			CreatedAt:         updatedAt.Add(time.Second),
			OverridesReviewID: &targetID,
		},
	}, updatedAt, settings)

	require.True(t, state.Certified)
	assert.False(t, state.Rejected)
	assert.Equal(t, models.ReviewStatusOverridden, state.PerType[0].Status)
	assert.Equal(t, "override", state.PerType[0].Outcome)
}

func TestResolveReviewState_StaleOverrideDoesNotClearFailure(t *testing.T) {
	updatedAt := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	targetID := int64(1)
	settings := (&config.Config{
		ReviewTypes:           []string{"security"},
		ReviewOutcomes:        []string{"pass", "fail", "override"},
		ReviewFailureOutcome:  "fail",
		ReviewOverrideOutcome: "override",
	}).ReviewConfig()

	state := ResolveReviewState([]models.Review{
		testReview(targetID, "security", "fail", "reviewer", updatedAt),
		{
			ID:                2,
			ArtifactType:      "server",
			ArtifactName:      "com.example/review-state",
			ArtifactVersion:   "1.0.0",
			ReviewType:        "security",
			Outcome:           "override",
			ReviewerSubject:   "admin",
			CreatedAt:         updatedAt.Add(-time.Second),
			OverridesReviewID: &targetID,
		},
	}, updatedAt, settings)

	assert.False(t, state.Certified)
	assert.True(t, state.Rejected)
	assert.Equal(t, models.ReviewStatusFail, state.PerType[0].Status)
}

func TestResolveReviewState_OverrideDoesNotHideOtherFailures(t *testing.T) {
	updatedAt := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	settings := (&config.Config{
		ReviewTypes:           []string{"security"},
		ReviewOutcomes:        []string{"pass", "fail"},
		ReviewFailureOutcome:  "fail",
		ReviewOverrideOutcome: "override",
	}).ReviewConfig()

	t.Run("an admin pass does not clear a curator failure", func(t *testing.T) {
		state := ResolveReviewState([]models.Review{
			testReview(1, "security", "fail", "curator", updatedAt),
			testReview(2, "security", "pass", "admin", updatedAt.Add(time.Second)),
		}, updatedAt, settings)

		assert.True(t, state.Rejected)
		assert.Equal(t, models.ReviewStatusFail, state.PerType[0].Status)
	})

	t.Run("one of two failures can be overridden without certifying", func(t *testing.T) {
		targetID := int64(1)
		state := ResolveReviewState([]models.Review{
			testReview(1, "security", "fail", "curator-one", updatedAt),
			testReview(2, "security", "fail", "curator-two", updatedAt),
			{
				ID:                3,
				ArtifactType:      "server",
				ArtifactName:      "com.example/review-state",
				ArtifactVersion:   "1.0.0",
				ReviewType:        "security",
				Outcome:           "override",
				ReviewerSubject:   "admin",
				CreatedAt:         updatedAt.Add(time.Second),
				OverridesReviewID: &targetID,
			},
		}, updatedAt, settings)

		assert.True(t, state.Rejected)
		assert.Equal(t, models.ReviewStatusFail, state.PerType[0].Status)
	})

	t.Run("an override does not carry to a revised failure", func(t *testing.T) {
		targetID := int64(1)
		state := ResolveReviewState([]models.Review{
			testReview(1, "security", "fail", "curator", updatedAt),
			{
				ID:                2,
				ArtifactType:      "server",
				ArtifactName:      "com.example/review-state",
				ArtifactVersion:   "1.0.0",
				ReviewType:        "security",
				Outcome:           "override",
				ReviewerSubject:   "admin",
				CreatedAt:         updatedAt.Add(time.Second),
				OverridesReviewID: &targetID,
			},
			testReview(3, "security", "fail", "curator", updatedAt.Add(2*time.Second)),
		}, updatedAt, settings)

		assert.True(t, state.Rejected)
		assert.Equal(t, models.ReviewStatusFail, state.PerType[0].Status)
	})
}

func TestReviewStateSequence_EditMakesBothPassesStale(t *testing.T) {
	registry, ownerCtx, server := newReviewStateFixture(t, "review-state-stale")
	createStateReview(t, registry, server, "security", "pass", "security-review")
	createStateReview(t, registry, server, "scientific", "pass", "scientific-review")

	state, err := registry.GetReviewState(ownerCtx, "server", server.Name, server.Version)
	require.NoError(t, err)
	require.True(t, state.Certified)

	updateServerForReviewState(t, registry, auth.WithSystemContext(context.Background()), server, "edited content")
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

	updateServerForReviewState(t, registry, auth.WithSystemContext(context.Background()), server, "reopened content")
	createStateReview(t, registry, server, "scientific", "pass", "scientific-repair")

	state, err = registry.GetReviewState(ownerCtx, "server", server.Name, server.Version)
	require.NoError(t, err)
	assert.False(t, state.Certified)
	assert.False(t, state.Rejected)
	assert.True(t, state.Pending)
	assert.Equal(t, models.ReviewStatusPending, state.PerType[0].Status)
	assert.Equal(t, models.ReviewStatusPass, state.PerType[1].Status)
}

func TestAuthorizeServerUpdate_UserRules(t *testing.T) {
	t.Run("owner can edit before any review", func(t *testing.T) {
		registry, ownerCtx, server := newReviewStateFixture(t, "user-unreviewed")
		_, err := updateReviewFixture(registry, ownerCtx, server, "owner edit")
		require.NoError(t, err)
	})

	for _, outcome := range []string{"pass", "fail"} {
		t.Run("owner is blocked after "+outcome, func(t *testing.T) {
			registry, ownerCtx, server := newReviewStateFixture(t, "user-"+outcome)
			createStateReview(t, registry, server, "security", outcome, outcome+" review")

			_, err := updateReviewFixture(registry, ownerCtx, server, "blocked edit")
			assert.ErrorIs(t, err, auth.ErrForbidden)
		})
	}

	t.Run("stale review still blocks owner", func(t *testing.T) {
		registry, ownerCtx, server := newReviewStateFixture(t, "user-stale")
		createStateReview(t, registry, server, "security", "pass", "stale review")
		updateServerForReviewState(t, registry, auth.WithSystemContext(context.Background()), server, "system edit")

		_, err := updateReviewFixture(registry, ownerCtx, server, "owner edit")
		assert.ErrorIs(t, err, auth.ErrForbidden)
	})

	t.Run("owner cannot edit someone else's artifact", func(t *testing.T) {
		registry, _, server := newReviewStateFixture(t, "other-owner")
		otherCtx := ownershipPermissionContext("other-subject", "Other", ownerEditPermissions())

		_, err := updateReviewFixture(registry, otherCtx, server, "unreviewed attempt")
		assert.ErrorIs(t, err, auth.ErrForbidden)

		createStateReview(t, registry, server, "security", "pass", "review")
		_, err = updateReviewFixture(registry, otherCtx, server, "reviewed attempt")
		assert.ErrorIs(t, err, auth.ErrForbidden)
	})
}

func TestAuthorizeServerUpdate_CertifiedFreeze(t *testing.T) {
	registry, _, server := newReviewStateFixture(t, "certified-freeze")
	createStateReview(t, registry, server, "security", "pass", "security")
	createStateReview(t, registry, server, "scientific", "pass", "scientific")

	curatorCtx := ownershipPermissionContext("curator-subject", "Curator", curatorEditPermissions())
	adminCtx := ownershipPermissionContext("admin-subject", "Admin", []auth.Permission{
		{Action: auth.PermissionActionAdmin, ResourcePattern: "*"},
	})

	_, err := updateReviewFixture(registry, curatorCtx, server, "curator attempt")
	assert.ErrorIs(t, err, auth.ErrForbidden)
	_, err = updateReviewFixture(registry, adminCtx, server, "admin attempt")
	assert.ErrorIs(t, err, auth.ErrForbidden)

	response, err := registry.GetServerByNameAndVersion(adminCtx, server.Name, server.Version)
	require.NoError(t, err)
	require.NotNil(t, response.Meta.Capabilities)
	assert.False(t, response.Meta.Capabilities.CanUpdate)

	_, err = updateReviewFixture(registry, auth.WithSystemContext(context.Background()), server, "system reconciliation")
	require.NoError(t, err)
}

func TestAuthorizeServerUpdate_RejectionAndPendingReopenForCurator(t *testing.T) {
	t.Run("pending type remains editable", func(t *testing.T) {
		registry, _, server := newReviewStateFixture(t, "curator-pending")
		createStateReview(t, registry, server, "security", "pass", "security")
		curatorCtx := ownershipPermissionContext("curator-subject", "Curator", curatorEditPermissions())

		_, err := updateReviewFixture(registry, curatorCtx, server, "pending edit")
		require.NoError(t, err)
	})

	t.Run("rejected version reopens for curator", func(t *testing.T) {
		registry, _, server := newReviewStateFixture(t, "curator-rejected")
		createStateReview(t, registry, server, "security", "fail", "security failure")
		curatorCtx := ownershipPermissionContext("curator-subject", "Curator", curatorEditPermissions())

		_, err := updateReviewFixture(registry, curatorCtx, server, "rejected edit")
		require.NoError(t, err)
	})
}

func TestServerCapabilitiesReviewAction(t *testing.T) {
	registry, _, server := newReviewStateFixture(t, "review-capability")
	reviewerCtx := ownershipPermissionContext("reviewer-subject", "Reviewer", []auth.Permission{
		{Action: auth.PermissionActionRead, ResourcePattern: "*"},
		{Action: auth.PermissionActionReview, ResourcePattern: "*"},
	})
	readOnlyCtx := ownershipPermissionContext("reader-subject", "Reader", []auth.Permission{
		{Action: auth.PermissionActionRead, ResourcePattern: "*"},
	})

	reviewerResponse, err := registry.GetServerByNameAndVersion(reviewerCtx, server.Name, server.Version)
	require.NoError(t, err)
	readOnlyResponse, err := registry.GetServerByNameAndVersion(readOnlyCtx, server.Name, server.Version)
	require.NoError(t, err)
	assert.True(t, reviewerResponse.Meta.Capabilities.CanReview)
	assert.False(t, readOnlyResponse.Meta.Capabilities.CanReview)
}

func TestAuthorizeServerUpdateRejectsMismatchedReviewSnapshot(t *testing.T) {
	svc := &registryServiceImpl{
		cfg: &config.Config{
			ReviewTypes:          []string{"security"},
			ReviewOutcomes:       []string{"pass", "fail"},
			ReviewFailureOutcome: "fail",
		},
	}
	current := &models.ServerResponse{
		Server: apiv0.ServerJSON{
			Name:    "com.example/current",
			Version: "1.0.0",
		},
		Meta: models.ServerResponseMeta{
			Official: &apiv0.RegistryExtensions{
				UpdatedAt: time.Now(),
			},
		},
	}

	err := svc.authorizeServerUpdateWithReviews(
		ownershipPermissionContext("owner", "Owner", ownerEditPermissions()),
		current.Server.Name,
		current,
		serverReviewSnapshot{
			serverName: "com.example/other",
			version:    current.Server.Version,
		},
	)
	assert.ErrorIs(t, err, database.ErrInvalidInput)
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
	createReviewAs(t, registry, server, "reviewer-"+reviewType, "Reviewer", reviewType, outcome, notes)
}

func createReviewAs(
	t *testing.T,
	registry RegistryService,
	server *apiv0.ServerJSON,
	subject,
	displayName,
	reviewType,
	outcome,
	notes string,
) {
	t.Helper()
	_, err := registry.CreateReview(
		ownershipPermissionContext(subject, displayName, []auth.Permission{
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

func reviewWithNotes(t *testing.T, reviews []models.Review, notes string) models.Review {
	t.Helper()
	for _, review := range reviews {
		if review.Notes == notes {
			return review
		}
	}
	t.Fatalf("review with notes %q not found", notes)
	return models.Review{}
}

func assertReviewMarkers(t *testing.T, review models.Review, current, stale, superseded bool) {
	t.Helper()
	require.NotNil(t, review.IsCurrent)
	require.NotNil(t, review.IsStale)
	require.NotNil(t, review.IsSuperseded)
	assert.Equal(t, current, *review.IsCurrent)
	assert.Equal(t, stale, *review.IsStale)
	assert.Equal(t, superseded, *review.IsSuperseded)
}

func updateServerForReviewState(t *testing.T, registry RegistryService, editCtx context.Context, server *apiv0.ServerJSON, description string) {
	t.Helper()
	_, err := updateReviewFixture(registry, editCtx, server, description)
	require.NoError(t, err)
}

func updateReviewFixture(registry RegistryService, editCtx context.Context, server *apiv0.ServerJSON, description string) (*models.ServerResponse, error) {
	updated := *server
	updated.Description = description
	return registry.UpdateServer(editCtx, server.Name, server.Version, &updated, nil)
}
