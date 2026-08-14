package v0

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	"github.com/agentregistry-dev/agentregistry/internal/registry/scoring"
	"github.com/agentregistry-dev/agentregistry/internal/registry/service"
	"github.com/agentregistry-dev/agentregistry/pkg/models"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/database"
	"github.com/agentregistry-dev/agentregistry/pkg/types"
	"github.com/danielgtaylor/huma/v2"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
)

// ScoreServerInput represents the input for triggering MCP scoring.
type ScoreServerInput struct {
	ServerName string `path:"serverName" doc:"URL-encoded server name" example:"com.example%2Fmy-server"`
	Version    string `path:"version" doc:"URL-encoded server version" example:"1.0.0"`
}

// ScoreServerResponse wraps the scoring result.
type ScoreServerResponse struct {
	ServerName string             `json:"serverName"`
	Version    string             `json:"version"`
	Scores     scoring.Scores     `json:"scores"`
	Rules      []scoring.Rule     `json:"rules"`
	MCPSurface scoring.MCPSurface `json:"mcp_surface"`
	Summary    string             `json:"summary"`
	Analysis   scoring.Analysis   `json:"analysis"`
}

// RegisterScoringEndpoint registers POST /servers/{serverName}/versions/{version}/score.
func RegisterScoringEndpoint(api huma.API, pathPrefix string, registry service.RegistryService, cfg *config.Config) {
	if cfg.MCPScoringURL == "" {
		slog.Info("MCP scoring endpoint disabled (AGENT_REGISTRY_MCP_SCORING_URL not set)")
		return
	}

	scoringClient := scoring.NewClient(cfg.MCPScoringURL, time.Duration(cfg.MCPScoringTimeout)*time.Second)

	huma.Register(api, huma.Operation{
		OperationID: "score-server" + strings.ReplaceAll(pathPrefix, "/", "-"),
		Method:      http.MethodPost,
		Path:        pathPrefix + "/servers/{serverName}/versions/{version}/score",
		Summary:     "Trigger MCP scoring for a server version",
		Description: "Calls the external mcp-scoring service to analyze the server's repository and persists the results.",
		Tags:        []string{"servers", "scoring"},
	}, func(ctx context.Context, input *ScoreServerInput) (*types.Response[ScoreServerResponse], error) {
		return scoreServerHandler(ctx, input, registry, scoringClient)
	})
}

func scoreServerHandler(
	ctx context.Context,
	input *ScoreServerInput,
	registry service.RegistryService,
	scoringClient *scoring.Client,
) (*types.Response[ScoreServerResponse], error) {
	serverName, err := url.PathUnescape(input.ServerName)
	if err != nil {
		return nil, huma.Error400BadRequest("Invalid server name encoding", err)
	}
	version, err := url.PathUnescape(input.Version)
	if err != nil {
		return nil, huma.Error400BadRequest("Invalid version encoding", err)
	}

	server, err := registry.GetServerByNameAndVersion(ctx, serverName, version)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, huma.Error404NotFound("Server not found")
		}
		return nil, huma.Error500InternalServerError("Failed to get server", err)
	}

	repoURL := ""
	if server.Server.Repository != nil {
		repoURL = server.Server.Repository.URL
	}
	if repoURL == "" {
		return nil, huma.Error400BadRequest("Server has no repository URL configured")
	}

	result, err := scoringClient.Analyze(ctx, repoURL)
	if err != nil {
		slog.Error("mcp-scoring request failed", "server", serverName, "version", version, "error", err)
		return nil, huma.Error502BadGateway(fmt.Sprintf("Scoring service error: %v", err))
	}

	if err := persistScoringResult(ctx, registry, serverName, version, server, result); err != nil {
		slog.Error("failed to persist scoring result", "server", serverName, "version", version, "error", err)
		return nil, huma.Error500InternalServerError("Failed to persist scoring results", err)
	}

	return &types.Response[ScoreServerResponse]{
		Body: ScoreServerResponse{
			ServerName: serverName,
			Version:    version,
			Scores:     result.Scores,
			Rules:      result.Rules,
			MCPSurface: result.MCPSurface,
			Summary:    result.Summary,
			Analysis:   result.Analysis,
		},
	}, nil
}

func persistScoringResult(
	ctx context.Context,
	registry service.RegistryService,
	serverName, version string,
	server *models.ServerResponse,
	result *scoring.AnalyzeResponse,
) error {
	updatedServer := server.Server
	if updatedServer.Meta == nil {
		updatedServer.Meta = &apiv0.ServerMeta{}
	}
	if updatedServer.Meta.PublisherProvided == nil {
		updatedServer.Meta.PublisherProvided = map[string]any{}
	}

	existing, _ := updatedServer.Meta.PublisherProvided["aregistry.ai/metadata"].(map[string]any)
	if existing == nil {
		existing = map[string]any{}
	}

	existing["mcp_scoring"] = map[string]any{
		"scores":      result.Scores,
		"rules":       result.Rules,
		"summary":     result.Summary,
		"analysis":    result.Analysis,
		"mcp_surface": result.MCPSurface,
		"scored_at":   time.Now().UTC().Format(time.RFC3339),
	}

	updatedServer.Meta.PublisherProvided["aregistry.ai/metadata"] = existing

	_, err := registry.UpdateServer(ctx, serverName, version, &updatedServer, nil)
	return err
}
