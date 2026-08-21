package service

import (
	"context"
	"testing"

	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	internaldb "github.com/agentregistry-dev/agentregistry/internal/registry/database"
	"github.com/agentregistry-dev/agentregistry/pkg/models"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type artifactCapabilityReader struct {
	name string
	read func(context.Context) ([]*models.CapabilitiesMeta, error)
}

func assertArtifactCapabilitiesForReadPaths(t *testing.T, artifactCanDeploy bool, readers []artifactCapabilityReader) {
	t.Helper()

	tests := []struct {
		name      string
		ctx       context.Context
		canDelete bool
		canDeploy bool
	}{
		{
			name: "principal with delete",
			ctx: ownershipPermissionContext("delete-subject", "Delete User", []auth.Permission{
				{Action: auth.PermissionActionRead, ResourcePattern: "*"},
				{Action: auth.PermissionActionDelete, ResourcePattern: "*"},
			}),
			canDelete: true,
		},
		{
			name: "principal without delete",
			ctx: ownershipPermissionContext("read-only-subject", "Read Only", []auth.Permission{
				{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			}),
		},
		{
			name: "principal with deploy",
			ctx: ownershipPermissionContext("deploy-subject", "Deploy User", []auth.Permission{
				{Action: auth.PermissionActionRead, ResourcePattern: "*"},
				{Action: auth.PermissionActionDeploy, ResourcePattern: "*"},
			}),
			canDeploy: true,
		},
		{
			name: "owner with edit own",
			ctx: ownershipPermissionContext("artifact-owner", "Owner", []auth.Permission{
				{Action: auth.PermissionActionRead, ResourcePattern: "*"},
				{Action: auth.PermissionActionEditOwn, ResourcePattern: "*"},
			}),
		},
	}

	for _, reader := range readers {
		reader := reader
		for _, tt := range tests {
			tt := tt
			t.Run(reader.name+"/"+tt.name, func(t *testing.T) {
				responses, err := reader.read(tt.ctx)
				require.NoError(t, err)
				require.Len(t, responses, 1)
				require.NotNil(t, responses[0])
				assert.False(t, responses[0].CanUpdate)
				assert.Equal(t, tt.canDelete, responses[0].CanDelete)
				if artifactCanDeploy {
					assert.Equal(t, tt.canDeploy, responses[0].CanDeploy)
				} else {
					assert.False(t, responses[0].CanDeploy)
				}
			})
		}
	}
}

func TestAgentCapabilitiesForAllReadPaths(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil, realCapabilityTestAuthorizer(t))
	name := "capability-agent"

	_, err := registry.CreateAgent(ownershipTestContext("artifact-owner", "Owner", auth.MethodOIDC), newOwnershipTestAgent(name))
	require.NoError(t, err)

	assertArtifactCapabilitiesForReadPaths(t, true, []artifactCapabilityReader{
		{
			name: "list",
			read: func(ctx context.Context) ([]*models.CapabilitiesMeta, error) {
				agents, _, err := registry.ListAgents(ctx, nil, "", 10)
				if err != nil {
					return nil, err
				}
				responses := make([]*models.CapabilitiesMeta, 0, len(agents))
				for _, agent := range agents {
					responses = append(responses, agent.Meta.Capabilities)
				}
				return responses, nil
			},
		},
		{
			name: "get by name",
			read: func(ctx context.Context) ([]*models.CapabilitiesMeta, error) {
				agent, err := registry.GetAgentByName(ctx, name)
				if err != nil {
					return nil, err
				}
				return []*models.CapabilitiesMeta{agent.Meta.Capabilities}, nil
			},
		},
		{
			name: "get by name and version",
			read: func(ctx context.Context) ([]*models.CapabilitiesMeta, error) {
				agent, err := registry.GetAgentByNameAndVersion(ctx, name, "1.0.0")
				if err != nil {
					return nil, err
				}
				return []*models.CapabilitiesMeta{agent.Meta.Capabilities}, nil
			},
		},
		{
			name: "list versions",
			read: func(ctx context.Context) ([]*models.CapabilitiesMeta, error) {
				agents, err := registry.GetAllVersionsByAgentName(ctx, name)
				if err != nil {
					return nil, err
				}
				responses := make([]*models.CapabilitiesMeta, 0, len(agents))
				for _, agent := range agents {
					responses = append(responses, agent.Meta.Capabilities)
				}
				return responses, nil
			},
		},
	})
}

func TestSkillCapabilitiesForAllReadPaths(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil, realCapabilityTestAuthorizer(t))
	name := "capability-skill"

	_, err := registry.CreateSkill(ownershipTestContext("artifact-owner", "Owner", auth.MethodOIDC), newOwnershipTestSkill(name))
	require.NoError(t, err)

	assertArtifactCapabilitiesForReadPaths(t, false, []artifactCapabilityReader{
		{
			name: "list",
			read: func(ctx context.Context) ([]*models.CapabilitiesMeta, error) {
				skills, _, err := registry.ListSkills(ctx, nil, "", 10)
				if err != nil {
					return nil, err
				}
				responses := make([]*models.CapabilitiesMeta, 0, len(skills))
				for _, skill := range skills {
					responses = append(responses, skill.Meta.Capabilities)
				}
				return responses, nil
			},
		},
		{
			name: "get by name",
			read: func(ctx context.Context) ([]*models.CapabilitiesMeta, error) {
				skill, err := registry.GetSkillByName(ctx, name)
				if err != nil {
					return nil, err
				}
				return []*models.CapabilitiesMeta{skill.Meta.Capabilities}, nil
			},
		},
		{
			name: "get by name and version",
			read: func(ctx context.Context) ([]*models.CapabilitiesMeta, error) {
				skill, err := registry.GetSkillByNameAndVersion(ctx, name, "1.0.0")
				if err != nil {
					return nil, err
				}
				return []*models.CapabilitiesMeta{skill.Meta.Capabilities}, nil
			},
		},
		{
			name: "list versions",
			read: func(ctx context.Context) ([]*models.CapabilitiesMeta, error) {
				skills, err := registry.GetAllVersionsBySkillName(ctx, name)
				if err != nil {
					return nil, err
				}
				responses := make([]*models.CapabilitiesMeta, 0, len(skills))
				for _, skill := range skills {
					responses = append(responses, skill.Meta.Capabilities)
				}
				return responses, nil
			},
		},
	})
}

func TestPromptCapabilitiesForAllReadPaths(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil, realCapabilityTestAuthorizer(t))
	name := "capability-prompt"

	_, err := registry.CreatePrompt(ownershipTestContext("artifact-owner", "Owner", auth.MethodOIDC), newOwnershipTestPrompt(name, "1.0.0"))
	require.NoError(t, err)

	assertArtifactCapabilitiesForReadPaths(t, false, []artifactCapabilityReader{
		{
			name: "list",
			read: func(ctx context.Context) ([]*models.CapabilitiesMeta, error) {
				prompts, _, err := registry.ListPrompts(ctx, nil, "", 10)
				if err != nil {
					return nil, err
				}
				responses := make([]*models.CapabilitiesMeta, 0, len(prompts))
				for _, prompt := range prompts {
					responses = append(responses, prompt.Meta.Capabilities)
				}
				return responses, nil
			},
		},
		{
			name: "get by name",
			read: func(ctx context.Context) ([]*models.CapabilitiesMeta, error) {
				prompt, err := registry.GetPromptByName(ctx, name)
				if err != nil {
					return nil, err
				}
				return []*models.CapabilitiesMeta{prompt.Meta.Capabilities}, nil
			},
		},
		{
			name: "get by name and version",
			read: func(ctx context.Context) ([]*models.CapabilitiesMeta, error) {
				prompt, err := registry.GetPromptByNameAndVersion(ctx, name, "1.0.0")
				if err != nil {
					return nil, err
				}
				return []*models.CapabilitiesMeta{prompt.Meta.Capabilities}, nil
			},
		},
		{
			name: "list versions",
			read: func(ctx context.Context) ([]*models.CapabilitiesMeta, error) {
				prompts, err := registry.GetAllVersionsByPromptName(ctx, name)
				if err != nil {
					return nil, err
				}
				responses := make([]*models.CapabilitiesMeta, 0, len(prompts))
				for _, prompt := range prompts {
					responses = append(responses, prompt.Meta.Capabilities)
				}
				return responses, nil
			},
		},
	})
}

func TestArtifactCapabilitiesWithoutSessionAreFalse(t *testing.T) {
	registry := &registryServiceImpl{authz: realCapabilityTestAuthorizer(t)}

	agent := &models.AgentResponse{}
	registry.annotateAgentCapabilities(context.Background(), "com.example/agent", agent)
	require.NotNil(t, agent.Meta.Capabilities)
	assert.False(t, agent.Meta.Capabilities.CanUpdate)
	assert.False(t, agent.Meta.Capabilities.CanDelete)
	assert.False(t, agent.Meta.Capabilities.CanDeploy)

	skill := &models.SkillResponse{}
	registry.annotateSkillCapabilities(context.Background(), "com.example/skill", skill)
	require.NotNil(t, skill.Meta.Capabilities)
	assert.False(t, skill.Meta.Capabilities.CanUpdate)
	assert.False(t, skill.Meta.Capabilities.CanDelete)
	assert.False(t, skill.Meta.Capabilities.CanDeploy)

	prompt := &models.PromptResponse{}
	registry.annotatePromptCapabilities(context.Background(), "com.example/prompt", prompt)
	require.NotNil(t, prompt.Meta.Capabilities)
	assert.False(t, prompt.Meta.Capabilities.CanUpdate)
	assert.False(t, prompt.Meta.Capabilities.CanDelete)
	assert.False(t, prompt.Meta.Capabilities.CanDeploy)
}

func TestUpdateServerResponseHasCapabilities(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{
		EnableRegistryValidation:       false,
		ValidateRepositoryReachability: false,
		ReviewTypes:                    []string{"security", "scientific"},
		ReviewOutcomes:                 []string{"pass", "fail"},
		ReviewFailureOutcome:           "fail",
	}, nil, realCapabilityTestAuthorizer(t))
	name := "com.example/capability-update-server"
	server := newOwnershipTestServer("capability-update-server")

	_, err := registry.CreateServer(
		ownershipTestContext("artifact-owner", "Owner", auth.MethodOIDC),
		server,
	)
	require.NoError(t, err)

	_, err = registry.CreateReview(
		ownershipPermissionContext("reviewer", "Reviewer", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionReview, ResourcePattern: "*"},
		}),
		"server",
		name,
		"1.0.0",
		"security",
		"pass",
		"baseline review",
	)
	require.NoError(t, err)

	updateContext := ownershipPermissionContext("artifact-owner", "Owner", curatorEditPermissions())
	update := newOwnershipTestServer("capability-update-server")
	update.Description = "Updated description"
	updated, err := registry.UpdateServer(updateContext, name, "1.0.0", update, nil)
	require.NoError(t, err)
	require.NotNil(t, updated.Meta.Capabilities)
	require.NotNil(t, updated.Meta.Review)
	assert.Equal(t, "pending", updated.Meta.Review.Status)
	require.Len(t, updated.Meta.Review.PerType, 2)
	for _, reviewType := range updated.Meta.Review.PerType {
		assert.Equal(t, "pending", reviewType.Status)
	}

	read, err := registry.GetServerByName(updateContext, name)
	require.NoError(t, err)
	require.NotNil(t, read.Meta.Capabilities)
	require.NotNil(t, read.Meta.Review)
	assert.Equal(t, read.Meta.Capabilities, updated.Meta.Capabilities)
	assert.Equal(t, read.Meta.Review, updated.Meta.Review)
	assert.True(t, updated.Meta.Capabilities.CanUpdate)
	assert.True(t, updated.Meta.Capabilities.CanDelete)
}
