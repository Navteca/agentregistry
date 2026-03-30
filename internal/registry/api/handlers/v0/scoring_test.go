package v0_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v0 "github.com/agentregistry-dev/agentregistry/internal/registry/api/handlers/v0"
	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	"github.com/agentregistry-dev/agentregistry/internal/registry/scoring"
	fakeregistry "github.com/agentregistry-dev/agentregistry/internal/registry/service/testing"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
)

func newScoringAPI(t *testing.T, registry *fakeregistry.FakeRegistry, scoringURL string) (*http.ServeMux, *config.Config) {
	t.Helper()
	cfg := &config.Config{
		MCPScoringURL:     scoringURL,
		MCPScoringTimeout: 5,
	}
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterScoringEndpoint(api, "/v0", registry, cfg)
	return mux, cfg
}

func sampleScoringResponse() scoring.AnalyzeResponse {
	return scoring.AnalyzeResponse{
		Scores: scoring.Scores{Security: 91, BestPractices: 88, Cost: 100, Total: 93},
		Rules: []scoring.Rule{
			{RuleID: "cost-01-minimal-dependencies", Outcome: "met", Evidence: "import logging", Rationale: "Limited imports"},
		},
		MCPSurface: scoring.MCPSurface{ToolCount: 2, ToolNames: []string{"search_publications"}, Transports: []string{"http", "stdio"}},
		Summary:    "MCP Server Analysis: Total Score 93/100",
		Analysis: scoring.Analysis{
			OverallAssessment: "Excellent (93/100)",
			Security:          scoring.CategoryDetail{Score: 91, RulesMet: 9, RulesNotMet: 1},
			BestPractices:     scoring.CategoryDetail{Score: 88, RulesMet: 3},
			Cost:              scoring.CategoryDetail{Score: 100, RulesMet: 6},
			KeyStrengths:      []string{"Protected against tool poisoning"},
		},
		EvalMeta:  scoring.EvalMeta{Provider: "openai", Model: "gpt-5.4-mini", RulesEvaluated: 22},
		CacheInfo: scoring.CacheInfo{Cached: false, CacheKey: "abc123", CacheTTL: 3600},
	}
}

func TestScoringEndpoint_Success(t *testing.T) {
	scoringSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req scoring.AnalyzeRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "https://github.com/testuser/test-server", req.SourceURL)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sampleScoringResponse())
	}))
	defer scoringSrv.Close()

	var updatedServerJSON *apiv0.ServerJSON
	fake := &fakeregistry.FakeRegistry{
		GetServerByNameAndVersionFn: func(ctx context.Context, serverName, version string) (*apiv0.ServerResponse, error) {
			return &apiv0.ServerResponse{
				Server: apiv0.ServerJSON{
					Schema:      model.CurrentSchemaURL,
					Name:        "io.github.testuser/test-server",
					Description: "Test",
					Version:     "1.0.0",
					Repository:  &model.Repository{URL: "https://github.com/testuser/test-server", Source: "git"},
				},
			}, nil
		},
		UpdateServerFn: func(ctx context.Context, serverName, version string, req *apiv0.ServerJSON, newStatus *string) (*apiv0.ServerResponse, error) {
			updatedServerJSON = req
			return &apiv0.ServerResponse{Server: *req}, nil
		},
	}

	mux, _ := newScoringAPI(t, fake, scoringSrv.URL)
	encodedName := url.PathEscape("io.github.testuser/test-server")
	req := httptest.NewRequest(http.MethodPost, "/v0/servers/"+encodedName+"/versions/1.0.0/score", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp v0.ScoreServerResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "io.github.testuser/test-server", resp.ServerName)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.Equal(t, 93, resp.Scores.Total)
	assert.Equal(t, 91, resp.Scores.Security)
	assert.Equal(t, 88, resp.Scores.BestPractices)
	assert.Equal(t, 100, resp.Scores.Cost)
	assert.Len(t, resp.Rules, 1)
	assert.Equal(t, "cost-01-minimal-dependencies", resp.Rules[0].RuleID)
	assert.Equal(t, 2, resp.MCPSurface.ToolCount)
	assert.Equal(t, "Excellent (93/100)", resp.Analysis.OverallAssessment)

	require.NotNil(t, updatedServerJSON)
	require.NotNil(t, updatedServerJSON.Meta)
	metadata, ok := updatedServerJSON.Meta.PublisherProvided["aregistry.ai/metadata"].(map[string]any)
	require.True(t, ok)
	mcpScoring, ok := metadata["mcp_scoring"].(map[string]any)
	require.True(t, ok)
	scores, ok := mcpScoring["scores"].(scoring.Scores)
	require.True(t, ok)
	assert.Equal(t, 93, scores.Total)
}

func TestScoringEndpoint_ServerNotFound(t *testing.T) {
	scoringSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("scoring service should not be called")
	}))
	defer scoringSrv.Close()

	fake := &fakeregistry.FakeRegistry{}

	mux, _ := newScoringAPI(t, fake, scoringSrv.URL)
	encodedName := url.PathEscape("io.github.testuser/nonexistent")
	req := httptest.NewRequest(http.MethodPost, "/v0/servers/"+encodedName+"/versions/1.0.0/score", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Server not found")
}

func TestScoringEndpoint_NoRepository(t *testing.T) {
	scoringSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("scoring service should not be called")
	}))
	defer scoringSrv.Close()

	fake := &fakeregistry.FakeRegistry{
		GetServerByNameAndVersionFn: func(ctx context.Context, serverName, version string) (*apiv0.ServerResponse, error) {
			return &apiv0.ServerResponse{
				Server: apiv0.ServerJSON{
					Schema:      model.CurrentSchemaURL,
					Name:        "io.github.testuser/no-repo",
					Description: "No repo",
					Version:     "1.0.0",
				},
			}, nil
		},
	}

	mux, _ := newScoringAPI(t, fake, scoringSrv.URL)
	encodedName := url.PathEscape("io.github.testuser/no-repo")
	req := httptest.NewRequest(http.MethodPost, "/v0/servers/"+encodedName+"/versions/1.0.0/score", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "no repository URL")
}

func TestScoringEndpoint_ScoringServiceError(t *testing.T) {
	scoringSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer scoringSrv.Close()

	fake := &fakeregistry.FakeRegistry{
		GetServerByNameAndVersionFn: func(ctx context.Context, serverName, version string) (*apiv0.ServerResponse, error) {
			return &apiv0.ServerResponse{
				Server: apiv0.ServerJSON{
					Schema:      model.CurrentSchemaURL,
					Name:        "io.github.testuser/test-server",
					Description: "Test",
					Version:     "1.0.0",
					Repository:  &model.Repository{URL: "https://github.com/testuser/test-server", Source: "git"},
				},
			}, nil
		},
	}

	mux, _ := newScoringAPI(t, fake, scoringSrv.URL)
	encodedName := url.PathEscape("io.github.testuser/test-server")
	req := httptest.NewRequest(http.MethodPost, "/v0/servers/"+encodedName+"/versions/1.0.0/score", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "Scoring service error")
}

func TestScoringEndpoint_ScoringServiceUnreachable(t *testing.T) {
	fake := &fakeregistry.FakeRegistry{
		GetServerByNameAndVersionFn: func(ctx context.Context, serverName, version string) (*apiv0.ServerResponse, error) {
			return &apiv0.ServerResponse{
				Server: apiv0.ServerJSON{
					Schema:      model.CurrentSchemaURL,
					Name:        "io.github.testuser/test-server",
					Description: "Test",
					Version:     "1.0.0",
					Repository:  &model.Repository{URL: "https://github.com/testuser/test-server", Source: "git"},
				},
			}, nil
		},
	}

	mux, _ := newScoringAPI(t, fake, "http://127.0.0.1:1")
	encodedName := url.PathEscape("io.github.testuser/test-server")
	req := httptest.NewRequest(http.MethodPost, "/v0/servers/"+encodedName+"/versions/1.0.0/score", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestScoringEndpoint_Disabled(t *testing.T) {
	cfg := &config.Config{MCPScoringURL: ""}
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterScoringEndpoint(api, "/v0", &fakeregistry.FakeRegistry{}, cfg)

	encodedName := url.PathEscape("io.github.testuser/test-server")
	req := httptest.NewRequest(http.MethodPost, "/v0/servers/"+encodedName+"/versions/1.0.0/score", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestScoringEndpoint_URLEncoded(t *testing.T) {
	scoringSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sampleScoringResponse())
	}))
	defer scoringSrv.Close()

	fake := &fakeregistry.FakeRegistry{
		GetServerByNameAndVersionFn: func(ctx context.Context, serverName, version string) (*apiv0.ServerResponse, error) {
			assert.Equal(t, "io.github.testuser/my-server", serverName)
			assert.Equal(t, "1.0.0+build.123", version)
			return &apiv0.ServerResponse{
				Server: apiv0.ServerJSON{
					Schema:      model.CurrentSchemaURL,
					Name:        serverName,
					Description: "Test",
					Version:     version,
					Repository:  &model.Repository{URL: "https://github.com/testuser/my-server", Source: "git"},
				},
			}, nil
		},
		UpdateServerFn: func(ctx context.Context, serverName, version string, req *apiv0.ServerJSON, newStatus *string) (*apiv0.ServerResponse, error) {
			return &apiv0.ServerResponse{Server: *req}, nil
		},
	}

	mux, _ := newScoringAPI(t, fake, scoringSrv.URL)
	encodedName := url.PathEscape("io.github.testuser/my-server")
	encodedVersion := url.PathEscape("1.0.0+build.123")
	req := httptest.NewRequest(http.MethodPost, "/v0/servers/"+encodedName+"/versions/"+encodedVersion+"/score", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestScoringEndpoint_PreservesExistingMetadata(t *testing.T) {
	scoringSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sampleScoringResponse())
	}))
	defer scoringSrv.Close()

	var updatedServerJSON *apiv0.ServerJSON
	fake := &fakeregistry.FakeRegistry{
		GetServerByNameAndVersionFn: func(ctx context.Context, serverName, version string) (*apiv0.ServerResponse, error) {
			return &apiv0.ServerResponse{
				Server: apiv0.ServerJSON{
					Schema:      model.CurrentSchemaURL,
					Name:        "io.github.testuser/test-server",
					Description: "Test",
					Version:     "1.0.0",
					Repository:  &model.Repository{URL: "https://github.com/testuser/test-server", Source: "git"},
					Meta: &apiv0.ServerMeta{
						PublisherProvided: map[string]any{
							"aregistry.ai/metadata": map[string]any{
								"stars": 42,
								"score": 3.14,
							},
						},
					},
				},
			}, nil
		},
		UpdateServerFn: func(ctx context.Context, serverName, version string, req *apiv0.ServerJSON, newStatus *string) (*apiv0.ServerResponse, error) {
			updatedServerJSON = req
			return &apiv0.ServerResponse{Server: *req}, nil
		},
	}

	mux, _ := newScoringAPI(t, fake, scoringSrv.URL)
	encodedName := url.PathEscape("io.github.testuser/test-server")
	req := httptest.NewRequest(http.MethodPost, "/v0/servers/"+encodedName+"/versions/1.0.0/score", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	require.NotNil(t, updatedServerJSON)
	metadata := updatedServerJSON.Meta.PublisherProvided["aregistry.ai/metadata"].(map[string]any)
	assert.Equal(t, 42, metadata["stars"])
	assert.Equal(t, 3.14, metadata["score"])
	assert.NotNil(t, metadata["mcp_scoring"])
}

func TestScoringEndpoint_PersistFailure(t *testing.T) {
	scoringSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sampleScoringResponse())
	}))
	defer scoringSrv.Close()

	fake := &fakeregistry.FakeRegistry{
		GetServerByNameAndVersionFn: func(ctx context.Context, serverName, version string) (*apiv0.ServerResponse, error) {
			return &apiv0.ServerResponse{
				Server: apiv0.ServerJSON{
					Schema:      model.CurrentSchemaURL,
					Name:        "io.github.testuser/test-server",
					Description: "Test",
					Version:     "1.0.0",
					Repository:  &model.Repository{URL: "https://github.com/testuser/test-server", Source: "git"},
				},
			}, nil
		},
		UpdateServerFn: func(ctx context.Context, serverName, version string, req *apiv0.ServerJSON, newStatus *string) (*apiv0.ServerResponse, error) {
			return nil, assert.AnError
		},
	}

	mux, _ := newScoringAPI(t, fake, scoringSrv.URL)
	encodedName := url.PathEscape("io.github.testuser/test-server")
	req := httptest.NewRequest(http.MethodPost, "/v0/servers/"+encodedName+"/versions/1.0.0/score", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to persist scoring results")
}

func TestScoringEndpoint_EmptyRepositoryURL(t *testing.T) {
	scoringSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("scoring service should not be called")
	}))
	defer scoringSrv.Close()

	fake := &fakeregistry.FakeRegistry{
		GetServerByNameAndVersionFn: func(ctx context.Context, serverName, version string) (*apiv0.ServerResponse, error) {
			return &apiv0.ServerResponse{
				Server: apiv0.ServerJSON{
					Schema:      model.CurrentSchemaURL,
					Name:        "io.github.testuser/empty-url",
					Description: "Empty URL",
					Version:     "1.0.0",
					Repository:  &model.Repository{URL: "", Source: "git"},
				},
			}, nil
		},
	}

	mux, _ := newScoringAPI(t, fake, scoringSrv.URL)
	encodedName := url.PathEscape("io.github.testuser/empty-url")
	req := httptest.NewRequest(http.MethodPost, "/v0/servers/"+encodedName+"/versions/1.0.0/score", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "no repository URL")
}
