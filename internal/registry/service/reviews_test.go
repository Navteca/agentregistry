package service

import (
	"errors"
	"testing"

	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	internaldb "github.com/agentregistry-dev/agentregistry/internal/registry/database"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateReview_PersistsIdentityAndAppendsRevisions(t *testing.T) {
	db := internaldb.NewTestDB(t)
	cfg := &config.Config{
		EnableRegistryValidation: false,
		ReviewTypes:              []string{"security", "scientific"},
		ReviewOutcomes:           []string{"pass", "fail"},
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
