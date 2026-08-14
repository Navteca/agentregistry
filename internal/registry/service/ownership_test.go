package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	internaldb "github.com/agentregistry-dev/agentregistry/internal/registry/database"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ownershipTestSession struct {
	user auth.User
}

func (s *ownershipTestSession) Principal() auth.Principal {
	return auth.Principal{User: s.user}
}

func ownershipTestContext(subject, displayName string, method auth.Method) context.Context {
	return auth.AuthSessionTo(context.Background(), &ownershipTestSession{
		user: auth.User{
			Permissions: []auth.Permission{{
				Action:          auth.PermissionActionAdmin,
				ResourcePattern: "*",
			}},
			AuthMethod:  method,
			Subject:     subject,
			DisplayName: displayName,
		},
	})
}

func newOwnershipTestServer(name string) *apiv0.ServerJSON {
	return &apiv0.ServerJSON{
		Name:        "com.example/" + name,
		Description: "Ownership test server",
		Version:     "1.0.0",
		Meta: &apiv0.ServerMeta{
			PublisherProvided: map[string]any{
				"aregistry.ai/ownership": map[string]any{
					"subject":     "request-subject",
					"displayName": "Request Display Name",
					"authMethod":  "request-method",
				},
			},
		},
	}
}

func TestCreateServerResolvesAndPersistsOwnership(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)
	ctx := ownershipTestContext("oidc-subject", "Ada Lovelace", auth.MethodOIDC)

	created, err := registry.CreateServer(ctx, newOwnershipTestServer("ownership-create"))
	require.NoError(t, err)
	require.NotNil(t, created.Meta.Ownership)
	assert.Equal(t, "oidc-subject", created.Meta.Ownership.Subject)
	assert.Equal(t, "Ada Lovelace", created.Meta.Ownership.DisplayName)
	assert.Equal(t, "oidc", created.Meta.Ownership.AuthMethod)

	fetched, err := registry.GetServerByName(ctx, "com.example/ownership-create")
	require.NoError(t, err)
	require.NotNil(t, fetched.Meta.Ownership)
	assert.Equal(t, "oidc-subject", fetched.Meta.Ownership.Subject)
	assert.Equal(t, "Ada Lovelace", fetched.Meta.Ownership.DisplayName)
	assert.Equal(t, "oidc", fetched.Meta.Ownership.AuthMethod)
}

func TestCreateServerAnonymousHasNoOwnership(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)
	ctx := ownershipTestContext("anonymous", "Anonymous", auth.MethodNone)

	created, err := registry.CreateServer(ctx, newOwnershipTestServer("ownership-anonymous"))
	require.NoError(t, err)
	assert.Nil(t, created.Meta.Ownership)

	fetched, err := registry.GetServerByName(ctx, "com.example/ownership-anonymous")
	require.NoError(t, err)
	assert.Nil(t, fetched.Meta.Ownership)

	serialized, err := json.Marshal(fetched)
	require.NoError(t, err)
	assert.NotContains(t, string(serialized), `"aregistry.ai/ownership"`)
}

func TestCreateServerSubjectsRemainDistinctWithSameDisplayName(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)

	for _, subject := range []string{"subject-one", "subject-two"} {
		ctx := ownershipTestContext(subject, "Shared Display Name", auth.MethodOIDC)
		_, err := registry.CreateServer(ctx, newOwnershipTestServer("ownership-"+subject))
		require.NoError(t, err)
	}

	first, err := registry.GetServerByName(ownershipTestContext("subject-one", "Shared Display Name", auth.MethodOIDC), "com.example/ownership-subject-one")
	require.NoError(t, err)
	second, err := registry.GetServerByName(ownershipTestContext("subject-two", "Shared Display Name", auth.MethodOIDC), "com.example/ownership-subject-two")
	require.NoError(t, err)
	require.NotNil(t, first.Meta.Ownership)
	require.NotNil(t, second.Meta.Ownership)
	assert.Equal(t, "subject-one", first.Meta.Ownership.Subject)
	assert.Equal(t, "subject-two", second.Meta.Ownership.Subject)
	assert.Equal(t, first.Meta.Ownership.DisplayName, second.Meta.Ownership.DisplayName)
}

func TestCreateServerDoesNotInventDisplayName(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)
	ctx := ownershipTestContext("github-user", "", auth.MethodGitHubAT)

	created, err := registry.CreateServer(ctx, newOwnershipTestServer("ownership-no-display"))
	require.NoError(t, err)
	require.NotNil(t, created.Meta.Ownership)
	assert.Equal(t, "github-user", created.Meta.Ownership.Subject)
	assert.Empty(t, created.Meta.Ownership.DisplayName)
	assert.Equal(t, "github-at", created.Meta.Ownership.AuthMethod)

	serialized, err := json.Marshal(created)
	require.NoError(t, err)
	assert.NotContains(t, string(serialized), `"displayName"`)
}
