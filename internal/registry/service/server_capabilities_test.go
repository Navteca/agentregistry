package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	internaldb "github.com/agentregistry-dev/agentregistry/internal/registry/database"
	"github.com/agentregistry-dev/agentregistry/pkg/models"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerCapabilitiesForAllReadPaths(t *testing.T) {
	db := internaldb.NewTestDB(t)
	cfg := &config.Config{EnableRegistryValidation: false}
	registry := NewRegistryService(db, cfg, nil, realCapabilityTestAuthorizer(t))
	serverName := "com.example/server-capabilities"

	_, err := registry.CreateServer(
		ownershipTestContext("capability-owner", "Owner", auth.MethodOIDC),
		newOwnershipTestServer("server-capabilities"),
	)
	require.NoError(t, err)

	readers := []struct {
		name string
		read func(context.Context) ([]*models.ServerResponse, error)
	}{
		{
			name: "get by name",
			read: func(ctx context.Context) ([]*models.ServerResponse, error) {
				response, err := registry.GetServerByName(ctx, serverName)
				if err != nil {
					return nil, err
				}
				return []*models.ServerResponse{response}, nil
			},
		},
		{
			name: "get by name and version",
			read: func(ctx context.Context) ([]*models.ServerResponse, error) {
				response, err := registry.GetServerByNameAndVersion(ctx, serverName, "1.0.0")
				if err != nil {
					return nil, err
				}
				return []*models.ServerResponse{response}, nil
			},
		},
		{
			name: "list",
			read: func(ctx context.Context) ([]*models.ServerResponse, error) {
				responses, _, err := registry.ListServers(ctx, nil, "", 10)
				return responses, err
			},
		},
		{
			name: "list versions",
			read: func(ctx context.Context) ([]*models.ServerResponse, error) {
				return registry.GetAllVersionsByServerName(ctx, serverName)
			},
		},
	}

	tests := []struct {
		name      string
		ctx       context.Context
		canUpdate bool
		canDelete bool
		canDeploy bool
	}{
		{
			name: "owner with edit own",
			ctx: ownershipPermissionContext("capability-owner", "Owner", []auth.Permission{
				{Action: auth.PermissionActionRead, ResourcePattern: "*"},
				{Action: auth.PermissionActionEditOwn, ResourcePattern: "*"},
			}),
			canUpdate: true,
		},
		{
			name: "owner without edit own",
			ctx: ownershipPermissionContext("capability-owner", "Owner", []auth.Permission{
				{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			}),
		},
		{
			name: "non-owner with edit own",
			ctx: ownershipPermissionContext("other-subject", "Other", []auth.Permission{
				{Action: auth.PermissionActionRead, ResourcePattern: "*"},
				{Action: auth.PermissionActionEditOwn, ResourcePattern: "*"},
			}),
		},
		{
			name: "curator with edit and delete",
			ctx: ownershipPermissionContext("curator-subject", "Curator", []auth.Permission{
				{Action: auth.PermissionActionRead, ResourcePattern: "*"},
				{Action: auth.PermissionActionEdit, ResourcePattern: "*"},
				{Action: auth.PermissionActionDelete, ResourcePattern: "*"},
			}),
			canUpdate: true,
			canDelete: true,
		},
		{
			name: "delete only",
			ctx: ownershipPermissionContext("delete-subject", "Delete User", []auth.Permission{
				{Action: auth.PermissionActionRead, ResourcePattern: "*"},
				{Action: auth.PermissionActionDelete, ResourcePattern: "*"},
			}),
			canDelete: true,
		},
		{
			name: "deploy",
			ctx: ownershipPermissionContext("deploy-subject", "Deploy User", []auth.Permission{
				{Action: auth.PermissionActionRead, ResourcePattern: "*"},
				{Action: auth.PermissionActionDeploy, ResourcePattern: "*"},
			}),
			canDeploy: true,
		},
		{
			name: "neither",
			ctx: ownershipPermissionContext("read-only-subject", "Read Only", []auth.Permission{
				{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			}),
		},
	}

	for _, reader := range readers {
		for _, tt := range tests {
			t.Run(reader.name+"/"+tt.name, func(t *testing.T) {
				responses, err := reader.read(tt.ctx)
				require.NoError(t, err)
				require.Len(t, responses, 1)
				require.NotNil(t, responses[0].Meta.Capabilities)
				assert.Equal(t, tt.canUpdate, responses[0].Meta.Capabilities.CanUpdate)
				assert.Equal(t, tt.canDelete, responses[0].Meta.Capabilities.CanDelete)
				assert.Equal(t, tt.canDeploy, responses[0].Meta.Capabilities.CanDeploy)
			})
		}
	}
}

func TestServerCapabilitiesWithoutSessionAreFalse(t *testing.T) {
	service := &registryServiceImpl{authz: realCapabilityTestAuthorizer(t)}
	response := &models.ServerResponse{}

	service.annotateServerCapabilities(context.Background(), "com.example/server", response)

	require.NotNil(t, response.Meta.Capabilities)
	assert.False(t, response.Meta.Capabilities.CanUpdate)
	assert.False(t, response.Meta.Capabilities.CanDelete)
	assert.False(t, response.Meta.Capabilities.CanDeploy)
}

func TestServerCapabilitiesWithNilAuthorizerProviderCannotDelete(t *testing.T) {
	service := NewRegistryService(nil, nil, nil, auth.Authorizer{}).(*registryServiceImpl)
	response := &models.ServerResponse{
		Meta: models.ServerResponseMeta{
			Ownership: &models.OwnershipMeta{Subject: "capability-owner"},
		},
	}
	ctx := ownershipPermissionContext("capability-owner", "Owner", []auth.Permission{
		{Action: auth.PermissionActionRead, ResourcePattern: "*"},
		{Action: auth.PermissionActionEditOwn, ResourcePattern: "*"},
	})

	service.annotateServerCapabilities(ctx, "com.example/server", response)

	require.NotNil(t, response.Meta.Capabilities)
	assert.False(t, response.Meta.Capabilities.CanUpdate, "missing official metadata must fail closed")
	assert.False(t, response.Meta.Capabilities.CanDelete)
	assert.False(t, response.Meta.Capabilities.CanDeploy)
}

func realCapabilityTestAuthorizer(t *testing.T) auth.Authorizer {
	t.Helper()

	seed := make([]byte, ed25519.SeedSize)
	_, err := rand.Read(seed)
	require.NoError(t, err)

	cfg := &config.Config{JWTPrivateKey: hex.EncodeToString(seed)}
	jwtManager := auth.NewJWTManager(cfg)
	return auth.Authorizer{Authz: auth.NewPublicAuthzProvider(jwtManager)}
}
