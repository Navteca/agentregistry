package v0

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentregistry-dev/agentregistry/internal/registry/scoring"
	fakeregistry "github.com/agentregistry-dev/agentregistry/internal/registry/service/testing"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScoreServerHandler_BadServerNameEncoding(t *testing.T) {
	registry := &fakeregistry.FakeRegistry{}
	client := scoring.NewClient("http://127.0.0.1:1", time.Second)

	_, err := scoreServerHandler(context.Background(), &ScoreServerInput{
		ServerName: "%",
		Version:    "1.0.0",
	}, registry, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid server name encoding")
}

func TestScoreServerHandler_BadVersionEncoding(t *testing.T) {
	registry := &fakeregistry.FakeRegistry{}
	client := scoring.NewClient("http://127.0.0.1:1", time.Second)

	_, err := scoreServerHandler(context.Background(), &ScoreServerInput{
		ServerName: "io.github.test/server",
		Version:    "%",
	}, registry, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid version encoding")
}

func TestScoreServerHandler_GetServerInternalError(t *testing.T) {
	registry := &fakeregistry.FakeRegistry{
		GetServerByNameAndVersionFn: func(context.Context, string, string) (*apiv0.ServerResponse, error) {
			return nil, errors.New("db down")
		},
	}
	client := scoring.NewClient("http://127.0.0.1:1", time.Second)

	_, err := scoreServerHandler(context.Background(), &ScoreServerInput{
		ServerName: "io.github.test/server",
		Version:    "1.0.0",
	}, registry, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to get server")
}
