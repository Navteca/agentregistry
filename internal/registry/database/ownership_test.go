package database_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	internaldb "github.com/agentregistry-dev/agentregistry/internal/registry/database"
	"github.com/agentregistry-dev/agentregistry/pkg/models"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/database"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerOwnershipSurvivesUpdates(t *testing.T) {
	db := internaldb.NewTestDB(t)
	ctx := internaldb.WithTestSession(context.Background())
	server := &apiv0.ServerJSON{
		Name:        "com.example/ownership-update",
		Description: "Original description",
		Version:     "1.0.0",
	}
	officialMeta := &apiv0.RegistryExtensions{
		Status:      model.StatusActive,
		PublishedAt: time.Now(),
		UpdatedAt:   time.Now(),
		IsLatest:    true,
	}
	ownership := models.OwnershipInput{
		Subject:     "creator-subject",
		DisplayName: "Creator Display Name",
		AuthMethod:  "oidc",
	}

	_, err := db.CreateServer(ctx, nil, server, officialMeta, ownership)
	require.NoError(t, err)

	updated := *server
	updated.Description = "Updated description"
	_, err = db.UpdateServer(ctx, nil, server.Name, server.Version, &updated)
	require.NoError(t, err)

	_, err = db.SetServerStatus(ctx, nil, server.Name, server.Version, string(model.StatusDeprecated))
	require.NoError(t, err)

	result, err := db.GetServerByNameAndVersion(ctx, nil, server.Name, server.Version)
	require.NoError(t, err)
	require.NotNil(t, result.Meta.Ownership)
	assert.Equal(t, ownership.Subject, result.Meta.Ownership.Subject)
	assert.Equal(t, ownership.DisplayName, result.Meta.Ownership.DisplayName)
	assert.Equal(t, ownership.AuthMethod, result.Meta.Ownership.AuthMethod)

	listed, _, err := db.ListServers(ctx, nil, &database.ServerFilter{Name: &server.Name}, "", 10)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.NotNil(t, listed[0].Meta.Ownership)
	assert.Equal(t, ownership.Subject, listed[0].Meta.Ownership.Subject)

	versions, err := db.GetAllVersionsByServerName(ctx, nil, server.Name)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	require.NotNil(t, versions[0].Meta.Ownership)
	assert.Equal(t, ownership.Subject, versions[0].Meta.Ownership.Subject)

	current, err := db.GetCurrentLatestVersion(ctx, nil, server.Name)
	require.NoError(t, err)
	require.NotNil(t, current.Meta.Ownership)
	assert.Equal(t, ownership.Subject, current.Meta.Ownership.Subject)
}

func TestServerWithoutOwnershipOmitsOwnershipJSON(t *testing.T) {
	db := internaldb.NewTestDB(t)
	ctx := internaldb.WithTestSession(context.Background())
	server := &apiv0.ServerJSON{
		Name:        "com.example/ownership-unowned",
		Description: "Unowned server",
		Version:     "1.0.0",
	}
	officialMeta := &apiv0.RegistryExtensions{
		Status:      model.StatusActive,
		PublishedAt: time.Now(),
		UpdatedAt:   time.Now(),
		IsLatest:    true,
	}

	_, err := db.CreateServer(ctx, nil, server, officialMeta, models.OwnershipInput{})
	require.NoError(t, err)

	result, err := db.GetServerByName(ctx, nil, server.Name)
	require.NoError(t, err)
	assert.Nil(t, result.Meta.Ownership)

	serialized, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(serialized), `"aregistry.ai/ownership"`)
}

func ownershipTestAgent(name string) *models.AgentJSON {
	return &models.AgentJSON{
		AgentManifest: models.AgentManifest{
			Name:        "ownership-" + name,
			Description: "Ownership test agent",
		},
		Version: "1.0.0",
	}
}

func ownershipTestAgentMeta() *models.AgentRegistryExtensions {
	now := time.Now()
	return &models.AgentRegistryExtensions{
		Status:      "active",
		PublishedAt: now,
		UpdatedAt:   now,
		IsLatest:    true,
	}
}

func ownershipTestSkill(name string) *models.SkillJSON {
	return &models.SkillJSON{
		Name:        name,
		Description: "Ownership test skill",
		Version:     "1.0.0",
	}
}

func ownershipTestSkillMeta() *models.SkillRegistryExtensions {
	now := time.Now()
	return &models.SkillRegistryExtensions{
		Status:      "active",
		PublishedAt: now,
		UpdatedAt:   now,
		IsLatest:    true,
	}
}

func ownershipTestPrompt(name, version string) *models.PromptJSON {
	return &models.PromptJSON{
		Name:    name,
		Version: version,
		Content: "Ownership test prompt",
	}
}

func ownershipTestPromptMeta(publishedAt time.Time, isLatest bool) *models.PromptRegistryExtensions {
	return &models.PromptRegistryExtensions{
		Status:      "active",
		PublishedAt: publishedAt,
		UpdatedAt:   publishedAt,
		IsLatest:    isLatest,
	}
}

func assertAgentOwnership(t *testing.T, response *models.AgentResponse, ownership models.OwnershipInput) {
	t.Helper()
	require.NotNil(t, response.Meta.Ownership)
	assert.Equal(t, ownership.Subject, response.Meta.Ownership.Subject)
	assert.Equal(t, ownership.DisplayName, response.Meta.Ownership.DisplayName)
	assert.Equal(t, ownership.AuthMethod, response.Meta.Ownership.AuthMethod)
}

func TestAgentOwnershipSurvivesUpdatesAndAllReads(t *testing.T) {
	db := internaldb.NewTestDB(t)
	ctx := internaldb.WithTestSession(context.Background())
	agent := ownershipTestAgent("ownership-agent-update")
	ownership := models.OwnershipInput{
		Subject:     "agent-creator-subject",
		DisplayName: "Agent Creator",
		AuthMethod:  "oidc",
	}

	_, err := db.CreateAgent(ctx, nil, agent, ownershipTestAgentMeta(), ownership)
	require.NoError(t, err)

	updated := *agent
	updated.Description = "Updated description"
	_, err = db.UpdateAgent(ctx, nil, agent.Name, agent.Version, &updated)
	require.NoError(t, err)

	_, err = db.SetAgentStatus(ctx, nil, agent.Name, agent.Version, "deprecated")
	require.NoError(t, err)

	latest, err := db.GetAgentByName(ctx, nil, agent.Name)
	require.NoError(t, err)
	assertAgentOwnership(t, latest, ownership)

	byVersion, err := db.GetAgentByNameAndVersion(ctx, nil, agent.Name, agent.Version)
	require.NoError(t, err)
	assertAgentOwnership(t, byVersion, ownership)

	versions, err := db.GetAllVersionsByAgentName(ctx, nil, agent.Name)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	assertAgentOwnership(t, versions[0], ownership)

	listed, _, err := db.ListAgents(ctx, nil, &database.AgentFilter{Name: &agent.Name}, "", 10)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assertAgentOwnership(t, listed[0], ownership)

	current, err := db.GetCurrentLatestAgentVersion(ctx, nil, agent.Name)
	require.NoError(t, err)
	assertAgentOwnership(t, current, ownership)
}

func TestAgentWithoutOwnershipOmitsOwnershipJSON(t *testing.T) {
	db := internaldb.NewTestDB(t)
	ctx := internaldb.WithTestSession(context.Background())
	agent := ownershipTestAgent("ownership-agent-unowned")

	_, err := db.CreateAgent(ctx, nil, agent, ownershipTestAgentMeta(), models.OwnershipInput{})
	require.NoError(t, err)

	result, err := db.GetAgentByName(ctx, nil, agent.Name)
	require.NoError(t, err)
	assert.Nil(t, result.Meta.Ownership)

	serialized, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(serialized), `"aregistry.ai/ownership"`)
}

func assertSkillOwnership(t *testing.T, response *models.SkillResponse, ownership models.OwnershipInput) {
	t.Helper()
	require.NotNil(t, response.Meta.Ownership)
	assert.Equal(t, ownership.Subject, response.Meta.Ownership.Subject)
	assert.Equal(t, ownership.DisplayName, response.Meta.Ownership.DisplayName)
	assert.Equal(t, ownership.AuthMethod, response.Meta.Ownership.AuthMethod)
}

func TestSkillOwnershipSurvivesUpdatesAndAllReads(t *testing.T) {
	db := internaldb.NewTestDB(t)
	ctx := internaldb.WithTestSession(context.Background())
	skill := ownershipTestSkill("ownership-skill-update")
	ownership := models.OwnershipInput{
		Subject:     "skill-creator-subject",
		DisplayName: "Skill Creator",
		AuthMethod:  "oidc",
	}

	_, err := db.CreateSkill(ctx, nil, skill, ownershipTestSkillMeta(), ownership)
	require.NoError(t, err)

	updated := *skill
	updated.Description = "Updated description"
	_, err = db.UpdateSkill(ctx, nil, skill.Name, skill.Version, &updated)
	require.NoError(t, err)

	_, err = db.SetSkillStatus(ctx, nil, skill.Name, skill.Version, "deprecated")
	require.NoError(t, err)

	latest, err := db.GetSkillByName(ctx, nil, skill.Name)
	require.NoError(t, err)
	assertSkillOwnership(t, latest, ownership)

	byVersion, err := db.GetSkillByNameAndVersion(ctx, nil, skill.Name, skill.Version)
	require.NoError(t, err)
	assertSkillOwnership(t, byVersion, ownership)

	versions, err := db.GetAllVersionsBySkillName(ctx, nil, skill.Name)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	assertSkillOwnership(t, versions[0], ownership)

	listed, _, err := db.ListSkills(ctx, nil, &database.SkillFilter{Name: &skill.Name}, "", 10)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assertSkillOwnership(t, listed[0], ownership)

	current, err := db.GetCurrentLatestSkillVersion(ctx, nil, skill.Name)
	require.NoError(t, err)
	assertSkillOwnership(t, current, ownership)
}

func TestSkillWithoutOwnershipOmitsOwnershipJSON(t *testing.T) {
	db := internaldb.NewTestDB(t)
	ctx := internaldb.WithTestSession(context.Background())
	skill := ownershipTestSkill("ownership-skill-unowned")

	_, err := db.CreateSkill(ctx, nil, skill, ownershipTestSkillMeta(), models.OwnershipInput{})
	require.NoError(t, err)

	result, err := db.GetSkillByName(ctx, nil, skill.Name)
	require.NoError(t, err)
	assert.Nil(t, result.Meta.Ownership)

	serialized, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(serialized), `"aregistry.ai/ownership"`)
}

func assertPromptOwnership(t *testing.T, response *models.PromptResponse, ownership models.OwnershipInput) {
	t.Helper()
	require.NotNil(t, response.Meta.Ownership)
	assert.Equal(t, ownership.Subject, response.Meta.Ownership.Subject)
	assert.Equal(t, ownership.DisplayName, response.Meta.Ownership.DisplayName)
	assert.Equal(t, ownership.AuthMethod, response.Meta.Ownership.AuthMethod)
}

func TestPromptOwnershipSurvivesLatestPromotionAndAllReads(t *testing.T) {
	db := internaldb.NewTestDB(t)
	ctx := internaldb.WithTestSession(context.Background())
	promptName := "ownership-prompt-promotion"
	first := ownershipTestPrompt(promptName, "1.0.0")
	firstOwnership := models.OwnershipInput{
		Subject:     "prompt-subject-one",
		DisplayName: "Shared Display Name",
		AuthMethod:  "oidc",
	}
	now := time.Now()

	_, err := db.CreatePrompt(ctx, nil, first, ownershipTestPromptMeta(now, true), firstOwnership)
	require.NoError(t, err)

	latest, err := db.GetPromptByName(ctx, nil, promptName)
	require.NoError(t, err)
	assertPromptOwnership(t, latest, firstOwnership)

	byVersion, err := db.GetPromptByNameAndVersion(ctx, nil, promptName, "1.0.0")
	require.NoError(t, err)
	assertPromptOwnership(t, byVersion, firstOwnership)

	versions, err := db.GetAllVersionsByPromptName(ctx, nil, promptName)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	assertPromptOwnership(t, versions[0], firstOwnership)

	listed, _, err := db.ListPrompts(ctx, nil, &database.PromptFilter{Name: &promptName}, "", 10)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assertPromptOwnership(t, listed[0], firstOwnership)

	current, err := db.GetCurrentLatestPromptVersion(ctx, nil, promptName)
	require.NoError(t, err)
	assertPromptOwnership(t, current, firstOwnership)

	err = db.UnmarkPromptAsLatest(ctx, nil, promptName)
	require.NoError(t, err)

	second := ownershipTestPrompt(promptName, "2.0.0")
	secondOwnership := models.OwnershipInput{
		Subject:     "prompt-subject-two",
		DisplayName: "Shared Display Name",
		AuthMethod:  "oidc",
	}
	_, err = db.CreatePrompt(ctx, nil, second, ownershipTestPromptMeta(now.Add(time.Second), true), secondOwnership)
	require.NoError(t, err)

	versions, err = db.GetAllVersionsByPromptName(ctx, nil, promptName)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	for _, version := range versions {
		switch version.Prompt.Version {
		case "1.0.0":
			assertPromptOwnership(t, version, firstOwnership)
		case "2.0.0":
			assertPromptOwnership(t, version, secondOwnership)
		default:
			t.Fatalf("unexpected prompt version %q", version.Prompt.Version)
		}
	}
}

func TestPromptWithoutOwnershipOmitsOwnershipJSON(t *testing.T) {
	db := internaldb.NewTestDB(t)
	ctx := internaldb.WithTestSession(context.Background())
	prompt := ownershipTestPrompt("ownership-prompt-unowned", "1.0.0")

	_, err := db.CreatePrompt(ctx, nil, prompt, ownershipTestPromptMeta(time.Now(), true), models.OwnershipInput{})
	require.NoError(t, err)

	result, err := db.GetPromptByName(ctx, nil, prompt.Name)
	require.NoError(t, err)
	assert.Nil(t, result.Meta.Ownership)

	serialized, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(serialized), `"aregistry.ai/ownership"`)
}
