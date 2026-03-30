// Package scoring provides an HTTP client for the mcp-scoring service.
package scoring

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// AnalyzeRequest is the payload sent to mcp-scoring POST /analyze.
type AnalyzeRequest struct {
	SourceURL string `json:"source_url"`
}

// AnalyzeResponse is the top-level response from mcp-scoring POST /analyze.
type AnalyzeResponse struct {
	Scores     Scores     `json:"scores"`
	Rules      []Rule     `json:"rules"`
	MCPSurface MCPSurface `json:"mcp_surface"`
	Summary    string     `json:"summary"`
	Analysis   Analysis   `json:"analysis"`
	EvalMeta   EvalMeta   `json:"evaluation_meta"`
	CacheInfo  CacheInfo  `json:"cache_info"`
}

// Scores holds the category scores returned by mcp-scoring.
type Scores struct {
	Security      int `json:"security"`
	BestPractices int `json:"best-practices"`
	Cost          int `json:"cost"`
	Total         int `json:"total"`
}

// Rule represents a single evaluated rule in the analysis.
type Rule struct {
	RuleID    string `json:"rule_id"`
	Outcome   string `json:"outcome"`
	Evidence  string `json:"evidence"`
	Rationale string `json:"rationale"`
}

// MCPSurface describes the MCP server surface area discovered during analysis.
type MCPSurface struct {
	ToolCount     int      `json:"tool_count"`
	ToolNames     []string `json:"tool_names"`
	PromptCount   int      `json:"prompt_count"`
	ResourceCount int      `json:"resource_count"`
	Transports    []string `json:"transports"`
	Description   string   `json:"description"`
}

// CategoryDetail holds score and rule stats for a single category.
type CategoryDetail struct {
	Score       int      `json:"score"`
	RulesMet    int      `json:"rules_met"`
	RulesNotMet int      `json:"rules_not_met"`
	RulesNA     int      `json:"rules_na"`
	Strengths   []string `json:"strengths"`
	Issues      []Issue  `json:"issues"`
}

// Issue is a scored finding within a category.
type Issue struct {
	RuleID   string `json:"rule_id"`
	Category string `json:"category"`
	Summary  string `json:"summary"`
	Severity string `json:"severity"`
}

// Analysis holds the high-level breakdown.
type Analysis struct {
	OverallAssessment string         `json:"overall_assessment"`
	Security          CategoryDetail `json:"security"`
	BestPractices     CategoryDetail `json:"best-practices"`
	Cost              CategoryDetail `json:"cost"`
	KeyStrengths      []string       `json:"key_strengths"`
	CriticalIssues    []Issue        `json:"critical_issues"`
	Recommendations   []string       `json:"recommendations"`
}

// EvalMeta captures evaluation metadata.
type EvalMeta struct {
	BatchCount        int    `json:"batch_count"`
	Provider          string `json:"provider"`
	Model             string `json:"model"`
	TotalInputTokens  int    `json:"total_input_tokens"`
	TotalOutputTokens int    `json:"total_output_tokens"`
	RulesEvaluated    int    `json:"rules_evaluated"`
	RulesError        int    `json:"rules_error"`
}

// CacheInfo tells whether the result was served from cache.
type CacheInfo struct {
	Cached   bool   `json:"cached"`
	CacheKey string `json:"cache_key"`
	CacheTTL int    `json:"cache_ttl_seconds"`
}

// Client is a thin HTTP client for the mcp-scoring service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a scoring client with the given base URL and timeout.
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Analyze calls POST /analyze on the mcp-scoring service.
func (c *Client) Analyze(ctx context.Context, sourceURL string) (*AnalyzeResponse, error) {
	// Build a compact JSON payload with proper string escaping.
	payload := []byte(`{"source_url":` + strconv.Quote(sourceURL) + `}`)

	var err error
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/analyze", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create analyze request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scoring service request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read scoring response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scoring service returned status %d: %s", resp.StatusCode, string(body))
	}

	var result AnalyzeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal scoring response: %w", err)
	}

	return &result, nil
}
