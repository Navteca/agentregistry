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
