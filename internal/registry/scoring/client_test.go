package scoring

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errorReadCloser struct{}

func (errorReadCloser) Read(_ []byte) (int, error) { return 0, errors.New("read failed") }
func (errorReadCloser) Close() error               { return nil }

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:8000", 30*time.Second)
	assert.Equal(t, "http://localhost:8000", c.baseURL)
	assert.NotNil(t, c.httpClient)
	assert.Equal(t, 30*time.Second, c.httpClient.Timeout)
}

func TestNewClientTrimsTrailingSlash(t *testing.T) {
	c := NewClient("http://localhost:8000/", 10*time.Second)
	assert.Equal(t, "http://localhost:8000", c.baseURL)
}

func TestAnalyzeSuccess(t *testing.T) {
	expected := &AnalyzeResponse{
		Scores: Scores{Security: 91, BestPractices: 88, Cost: 100, Total: 93},
		Rules: []Rule{
			{RuleID: "cost-01-minimal-dependencies", Outcome: "met", Evidence: "import logging", Rationale: "Limited imports"},
		},
		MCPSurface: MCPSurface{ToolCount: 2, ToolNames: []string{"search_publications"}, PromptCount: 0, ResourceCount: 0, Transports: []string{"http", "sse", "stdio"}, Description: "USGS publications"},
		Summary:    "MCP Server Analysis: Total Score 93/100",
		Analysis: Analysis{
			OverallAssessment: "Excellent (93/100)",
			Security:          CategoryDetail{Score: 91, RulesMet: 9, RulesNotMet: 1},
			BestPractices:     CategoryDetail{Score: 88, RulesMet: 3},
			Cost:              CategoryDetail{Score: 100, RulesMet: 6},
			KeyStrengths:      []string{"Protected against tool poisoning"},
			CriticalIssues:    []Issue{{RuleID: "sec-03-tool-shadowing", Category: "security", Summary: "Generic name", Severity: "medium"}},
			Recommendations:   []string{"Continue following MCP best practices."},
		},
		EvalMeta:  EvalMeta{BatchCount: 3, Provider: "openai", Model: "gpt-5.4-mini", RulesEvaluated: 22},
		CacheInfo: CacheInfo{Cached: false, CacheKey: "abc123", CacheTTL: 3600},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/analyze", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req AnalyzeRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "https://github.com/navteca/hello-mcp", req.SourceURL)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	c := NewClient(server.URL, 5*time.Second)
	result, err := c.Analyze(context.Background(), "https://github.com/navteca/hello-mcp")
	require.NoError(t, err)

	assert.Equal(t, 93, result.Scores.Total)
	assert.Equal(t, 91, result.Scores.Security)
	assert.Equal(t, 88, result.Scores.BestPractices)
	assert.Equal(t, 100, result.Scores.Cost)
	assert.Len(t, result.Rules, 1)
	assert.Equal(t, "cost-01-minimal-dependencies", result.Rules[0].RuleID)
	assert.Equal(t, "met", result.Rules[0].Outcome)
	assert.Equal(t, 2, result.MCPSurface.ToolCount)
	assert.Equal(t, "Excellent (93/100)", result.Analysis.OverallAssessment)
	assert.Equal(t, 91, result.Analysis.Security.Score)
	assert.Equal(t, "openai", result.EvalMeta.Provider)
	assert.False(t, result.CacheInfo.Cached)
}

func TestAnalyzeNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	c := NewClient(server.URL, 5*time.Second)
	_, err := c.Analyze(context.Background(), "https://github.com/test/repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scoring service returned status 500")
	assert.Contains(t, err.Error(), "internal server error")
}

func TestAnalyzeBadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json"))
	}))
	defer server.Close()

	c := NewClient(server.URL, 5*time.Second)
	_, err := c.Analyze(context.Background(), "https://github.com/test/repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal scoring response")
}

func TestAnalyzeConnectionError(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", 1*time.Second)
	_, err := c.Analyze(context.Background(), "https://github.com/test/repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scoring service request failed")
}

func TestAnalyzeContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewClient(server.URL, 10*time.Second)
	_, err := c.Analyze(ctx, "https://github.com/test/repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scoring service request failed")
}

func TestAnalyzeBadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid source_url"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, 5*time.Second)
	_, err := c.Analyze(context.Background(), "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scoring service returned status 400")
}

func TestAnalyze422Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"detail":"Could not process the source"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, 5*time.Second)
	_, err := c.Analyze(context.Background(), "https://github.com/test/bad-repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scoring service returned status 422")
}

func TestAnalyzeEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(server.URL, 5*time.Second)
	_, err := c.Analyze(context.Background(), "https://github.com/test/repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal scoring response")
}

func TestAnalyzeInvalidBaseURL(t *testing.T) {
	c := NewClient("://invalid-url", 1*time.Second)
	_, err := c.Analyze(context.Background(), "https://github.com/test/repo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create analyze request")
}

func TestAnalyzeRequestPayload(t *testing.T) {
	var receivedReq AnalyzeRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AnalyzeResponse{})
	}))
	defer server.Close()

	c := NewClient(server.URL, 5*time.Second)
	_, err := c.Analyze(context.Background(), "https://github.com/navteca/my-server")
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/navteca/my-server", receivedReq.SourceURL)
}

func TestAnalyzeReadBodyError(t *testing.T) {
	c := NewClient("http://example.invalid", 5*time.Second)
	c.httpClient = &http.Client{
		Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       errorReadCloser{},
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, err := c.Analyze(context.Background(), "https://github.com/navteca/repo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read scoring response")
}

func TestAnalyzeEscapesSourceURLInPayload(t *testing.T) {
	var rawBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rawBody = body
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AnalyzeResponse{})
	}))
	defer server.Close()

	c := NewClient(server.URL, 5*time.Second)
	_, err := c.Analyze(context.Background(), `https://example.com/repo?x="quoted"&y=\slash`)
	require.NoError(t, err)
	assert.Contains(t, string(rawBody), `\"quoted\"`)
}
