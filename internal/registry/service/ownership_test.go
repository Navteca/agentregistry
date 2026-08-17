package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	internaldb "github.com/agentregistry-dev/agentregistry/internal/registry/database"
	"github.com/agentregistry-dev/agentregistry/pkg/models"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
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

func ownershipPermissionContext(subject, displayName string, permissions []auth.Permission) context.Context {
	return auth.AuthSessionTo(context.Background(), &ownershipTestSession{
		user: auth.User{
			Permissions: permissions,
			AuthMethod:  auth.MethodOIDC,
			Subject:     subject,
			DisplayName: displayName,
		},
	})
}

func ownerEditPermissions() []auth.Permission {
	return []auth.Permission{
		{Action: auth.PermissionActionRead, ResourcePattern: "*"},
		{Action: auth.PermissionActionPublish, ResourcePattern: "*"},
		{Action: auth.PermissionActionEditOwn, ResourcePattern: "*"},
	}
}

func curatorEditPermissions() []auth.Permission {
	return []auth.Permission{
		{Action: auth.PermissionActionRead, ResourcePattern: "*"},
		{Action: auth.PermissionActionPublish, ResourcePattern: "*"},
		{Action: auth.PermissionActionEdit, ResourcePattern: "*"},
		{Action: auth.PermissionActionDelete, ResourcePattern: "*"},
	}
}

func newOwnershipTestServer(name string) *apiv0.ServerJSON {
	return &apiv0.ServerJSON{
		Name:        "com.example/" + name,
		Description: "Ownership test server",
		Schema:      model.CurrentSchemaURL,
		Version:     "1.0.0",
		Repository: &model.Repository{
			URL:    "https://github.com/owner/repo",
			Source: "git",
		},
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

func newOwnershipTestAgent(name string) *models.AgentJSON {
	return &models.AgentJSON{
		AgentManifest: models.AgentManifest{
			Name:        name,
			Description: "Ownership test agent",
		},
		Version: "1.0.0",
	}
}

func newOwnershipTestSkill(name string) *models.SkillJSON {
	return &models.SkillJSON{
		Name:        name,
		Description: "Ownership test skill",
		Version:     "1.0.0",
	}
}

func newOwnershipTestPrompt(name, version string) *models.PromptJSON {
	return &models.PromptJSON{
		Name:    name,
		Version: version,
		Content: "Ownership test prompt",
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
	server := newOwnershipTestServer("ownership-anonymous")

	created, err := registry.CreateServer(ctx, server)
	require.NoError(t, err)
	assert.Nil(t, created.Meta.Ownership)

	fetched, err := registry.GetServerByName(ctx, "com.example/ownership-anonymous")
	require.NoError(t, err)
	assert.Nil(t, fetched.Meta.Ownership)
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
}

func TestCreateAgentResolvesAndPersistsOwnership(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)
	ctx := ownershipTestContext("agent-oidc-subject", "Ada Lovelace", auth.MethodOIDC)

	created, err := registry.CreateAgent(ctx, newOwnershipTestAgent("ownership-agent-create"))
	require.NoError(t, err)
	require.NotNil(t, created.Meta.Ownership)
	assert.Equal(t, "agent-oidc-subject", created.Meta.Ownership.Subject)
	assert.Equal(t, "Ada Lovelace", created.Meta.Ownership.DisplayName)
	assert.Equal(t, "oidc", created.Meta.Ownership.AuthMethod)

	fetched, err := registry.GetAgentByName(ctx, "ownership-agent-create")
	require.NoError(t, err)
	require.NotNil(t, fetched.Meta.Ownership)
	assert.Equal(t, "agent-oidc-subject", fetched.Meta.Ownership.Subject)
}

func TestCreateAgentIgnoresOwnershipShapedRequestData(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)
	ctx := ownershipTestContext("authenticated-subject", "Authenticated User", auth.MethodOIDC)

	var agent models.AgentJSON
	err := json.Unmarshal([]byte(`{
		"name": "ownership-agent-request",
		"version": "1.0.0",
		"_meta": {
			"aregistry.ai/ownership": {
				"subject": "request-subject",
				"displayName": "Request Display Name",
				"authMethod": "request-method"
			}
		}

	}`), &agent)
	require.NoError(t, err)

	created, err := registry.CreateAgent(ctx, &agent)
	require.NoError(t, err)
	require.NotNil(t, created.Meta.Ownership)
	assert.Equal(t, "authenticated-subject", created.Meta.Ownership.Subject)
	assert.NotEqual(t, "request-subject", created.Meta.Ownership.Subject)
}

func TestCreateAgentSubjectsRemainDistinctWithSameDisplayName(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)

	createdAgents := make(map[string]*models.AgentResponse, 2)
	for _, subject := range []string{"agent-subject-one", "agent-subject-two"} {
		ctx := ownershipTestContext(subject, "Shared Display Name", auth.MethodOIDC)
		created, err := registry.CreateAgent(ctx, newOwnershipTestAgent("ownership-agent-"+subject))
		require.NoError(t, err)
		require.NotNil(t, created)
		require.Equal(t, "ownership-agent-"+subject, created.Agent.Name)
		require.Equal(t, "1.0.0", created.Agent.Version)
		require.NotNil(t, created.Meta.Official)
		t.Logf("created agent name=%q version=%q is_latest=%t", created.Agent.Name, created.Agent.Version, created.Meta.Official.IsLatest)
		createdAgents[subject] = created
	}

	firstCreated := createdAgents["agent-subject-one"]
	first, err := registry.GetAgentByNameAndVersion(
		ownershipTestContext("agent-subject-one", "Shared Display Name", auth.MethodOIDC),
		firstCreated.Agent.Name,
		firstCreated.Agent.Version,
	)
	require.NoError(t, err)
	secondCreated := createdAgents["agent-subject-two"]
	second, err := registry.GetAgentByNameAndVersion(
		ownershipTestContext("agent-subject-two", "Shared Display Name", auth.MethodOIDC),
		secondCreated.Agent.Name,
		secondCreated.Agent.Version,
	)
	require.NoError(t, err)
	require.NotNil(t, first.Meta.Ownership)
	require.NotNil(t, second.Meta.Ownership)
	assert.Equal(t, "agent-subject-one", first.Meta.Ownership.Subject)
	assert.Equal(t, "agent-subject-two", second.Meta.Ownership.Subject)
	assert.Equal(t, first.Meta.Ownership.DisplayName, second.Meta.Ownership.DisplayName)
}

func TestCreateSkillResolvesAndPersistsOwnership(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)
	ctx := ownershipTestContext("skill-oidc-subject", "Ada Lovelace", auth.MethodOIDC)

	created, err := registry.CreateSkill(ctx, newOwnershipTestSkill("ownership-skill-create"))
	require.NoError(t, err)
	require.NotNil(t, created.Meta.Ownership)
	assert.Equal(t, "skill-oidc-subject", created.Meta.Ownership.Subject)
	assert.Equal(t, "Ada Lovelace", created.Meta.Ownership.DisplayName)
	assert.Equal(t, "oidc", created.Meta.Ownership.AuthMethod)

	fetched, err := registry.GetSkillByName(ctx, "ownership-skill-create")
	require.NoError(t, err)
	require.NotNil(t, fetched.Meta.Ownership)
	assert.Equal(t, "skill-oidc-subject", fetched.Meta.Ownership.Subject)
}

func TestCreateSkillIgnoresOwnershipShapedRequestData(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)
	ctx := ownershipTestContext("authenticated-subject", "Authenticated User", auth.MethodOIDC)

	var skill models.SkillJSON
	err := json.Unmarshal([]byte(`{
		"name": "ownership-skill-request",
		"description": "Request fixture",
		"version": "1.0.0",
		"_meta": {
			"aregistry.ai/ownership": {
				"subject": "request-subject",
				"displayName": "Request Display Name",
				"authMethod": "request-method"
			}
		}
	}`), &skill)
	require.NoError(t, err)

	created, err := registry.CreateSkill(ctx, &skill)
	require.NoError(t, err)
	require.NotNil(t, created.Meta.Ownership)
	assert.Equal(t, "authenticated-subject", created.Meta.Ownership.Subject)
	assert.NotEqual(t, "request-subject", created.Meta.Ownership.Subject)
}

func TestCreateSkillAnonymousHasNoOwnership(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)
	ctx := ownershipTestContext("anonymous", "Anonymous", auth.MethodNone)

	created, err := registry.CreateSkill(ctx, newOwnershipTestSkill("ownership-skill-anonymous"))
	require.NoError(t, err)
	assert.Nil(t, created.Meta.Ownership)
}

func TestCreateSkillSubjectsRemainDistinctWithSameDisplayName(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)

	for _, subject := range []string{"skill-subject-one", "skill-subject-two"} {
		ctx := ownershipTestContext(subject, "Shared Display Name", auth.MethodOIDC)
		_, err := registry.CreateSkill(ctx, newOwnershipTestSkill("ownership-"+subject))
		require.NoError(t, err)
	}

	first, err := registry.GetSkillByName(ownershipTestContext("skill-subject-one", "Shared Display Name", auth.MethodOIDC), "ownership-skill-subject-one")
	require.NoError(t, err)
	second, err := registry.GetSkillByName(ownershipTestContext("skill-subject-two", "Shared Display Name", auth.MethodOIDC), "ownership-skill-subject-two")
	require.NoError(t, err)
	require.NotNil(t, first.Meta.Ownership)
	require.NotNil(t, second.Meta.Ownership)
	assert.Equal(t, "skill-subject-one", first.Meta.Ownership.Subject)
	assert.Equal(t, "skill-subject-two", second.Meta.Ownership.Subject)
	assert.Equal(t, first.Meta.Ownership.DisplayName, second.Meta.Ownership.DisplayName)
}

func TestCreatePromptResolvesAndPersistsOwnership(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)
	ctx := ownershipTestContext("prompt-oidc-subject", "Ada Lovelace", auth.MethodOIDC)

	created, err := registry.CreatePrompt(ctx, newOwnershipTestPrompt("ownership-prompt-create", "1.0.0"))
	require.NoError(t, err)
	require.NotNil(t, created.Meta.Ownership)
	assert.Equal(t, "prompt-oidc-subject", created.Meta.Ownership.Subject)
	assert.Equal(t, "Ada Lovelace", created.Meta.Ownership.DisplayName)
	assert.Equal(t, "oidc", created.Meta.Ownership.AuthMethod)

	fetched, err := registry.GetPromptByName(ctx, "ownership-prompt-create")
	require.NoError(t, err)
	require.NotNil(t, fetched.Meta.Ownership)
	assert.Equal(t, "prompt-oidc-subject", fetched.Meta.Ownership.Subject)
}

func TestCreatePromptIgnoresOwnershipShapedRequestData(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)
	ctx := ownershipTestContext("authenticated-subject", "Authenticated User", auth.MethodOIDC)

	var prompt models.PromptJSON
	err := json.Unmarshal([]byte(`{
		"name": "ownership-prompt-request",
		"version": "1.0.0",
		"content": "Request fixture",
		"_meta": {
			"aregistry.ai/ownership": {
				"subject": "request-subject",
				"displayName": "Request Display Name",
				"authMethod": "request-method"
			}
		}
	}`), &prompt)
	require.NoError(t, err)

	created, err := registry.CreatePrompt(ctx, &prompt)
	require.NoError(t, err)
	require.NotNil(t, created.Meta.Ownership)
	assert.Equal(t, "authenticated-subject", created.Meta.Ownership.Subject)
	assert.NotEqual(t, "request-subject", created.Meta.Ownership.Subject)
}

func TestCreatePromptAnonymousHasNoOwnership(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)
	ctx := ownershipTestContext("anonymous", "Anonymous", auth.MethodNone)

	created, err := registry.CreatePrompt(ctx, newOwnershipTestPrompt("ownership-prompt-anonymous", "1.0.0"))
	require.NoError(t, err)
	assert.Nil(t, created.Meta.Ownership)
}

func TestPromptOwnershipSurvivesLatestPromotion(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)

	firstCtx := ownershipTestContext("prompt-subject-one", "Shared Display Name", auth.MethodOIDC)
	_, err := registry.CreatePrompt(firstCtx, newOwnershipTestPrompt("ownership-prompt-promotion", "1.0.0"))
	require.NoError(t, err)

	secondCtx := ownershipTestContext("prompt-subject-two", "Shared Display Name", auth.MethodOIDC)
	_, err = registry.CreatePrompt(secondCtx, newOwnershipTestPrompt("ownership-prompt-promotion", "2.0.0"))
	require.NoError(t, err)

	versions, err := registry.GetAllVersionsByPromptName(secondCtx, "ownership-prompt-promotion")
	require.NoError(t, err)
	require.Len(t, versions, 2)
	for _, version := range versions {
		switch version.Prompt.Version {
		case "1.0.0":
			assert.Equal(t, "prompt-subject-one", version.Meta.Ownership.Subject)
		case "2.0.0":
			assert.Equal(t, "prompt-subject-two", version.Meta.Ownership.Subject)
		default:
			t.Fatalf("unexpected prompt version %q", version.Prompt.Version)
		}
	}
}

func TestUpdateServerOwnerScopedEdit(t *testing.T) {
	db := internaldb.NewTestDB(t)
	registry := NewRegistryService(db, &config.Config{EnableRegistryValidation: false}, nil)

	tests := []struct {
		name          string
		ownerSubject  string
		ownerName     string
		editorSubject string
		editorName    string
		editorPerms   []auth.Permission
		wantErr       error
	}{
		{
			name:          "user updates own unreviewed artifact",
			ownerSubject:  "owner-subject",
			ownerName:     "Owner",
			editorSubject: "owner-subject",
			editorName:    "Owner",
			editorPerms:   ownerEditPermissions(),
		},
		{
			name:          "another subject is refused",
			ownerSubject:  "owner-subject",
			ownerName:     "Owner",
			editorSubject: "other-subject",
			editorName:    "Other",
			editorPerms:   ownerEditPermissions(),
			wantErr:       auth.ErrForbidden,
		},
		{
			name:          "unowned artifact is refused",
			ownerSubject:  "",
			ownerName:     "",
			editorSubject: "owner-subject",
			editorName:    "Owner",
			editorPerms:   ownerEditPermissions(),
			wantErr:       auth.ErrForbidden,
		},
		{
			name:          "curator updates another subject's artifact",
			ownerSubject:  "owner-subject",
			ownerName:     "Owner",
			editorSubject: "curator-subject",
			editorName:    "Curator",
			editorPerms:   curatorEditPermissions(),
		},
		{
			name:          "same display name with different subjects is refused",
			ownerSubject:  "owner-subject",
			ownerName:     "Shared Name",
			editorSubject: "other-subject",
			editorName:    "Shared Name",
			editorPerms:   ownerEditPermissions(),
			wantErr:       auth.ErrForbidden,
		},
		{
			name:          "display name change does not lose owner rights",
			ownerSubject:  "owner-subject",
			ownerName:     "Old Name",
			editorSubject: "owner-subject",
			editorName:    "New Name",
			editorPerms:   ownerEditPermissions(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverSuffix := "owner-edit-" + strings.ToLower(strings.NewReplacer(" ", "-", "'", "").Replace(tt.name))
			serverName := "com.example/" + serverSuffix
			ownerContext := ownershipPermissionContext(tt.ownerSubject, tt.ownerName, ownerEditPermissions())
			if tt.ownerSubject == "" {
				ownerContext = ownershipTestContext("anonymous", "Anonymous", auth.MethodNone)
			}

			_, err := registry.CreateServer(ownerContext, newOwnershipTestServer(serverSuffix))
			require.NoError(t, err)

			update := newOwnershipTestServer(serverSuffix)
			update.Description = "Updated ownership test server"
			editorContext := ownershipPermissionContext(tt.editorSubject, tt.editorName, tt.editorPerms)
			updated, err := registry.UpdateServer(editorContext, serverName, "1.0.0", update, nil)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, updated)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, updated)
			assert.Equal(t, "Updated ownership test server", updated.Server.Description)
		})
	}
}
