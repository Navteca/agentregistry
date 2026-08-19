package v0_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/stretchr/testify/require"

	v0 "github.com/agentregistry-dev/agentregistry/internal/registry/api/handlers/v0"
	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
)

func TestFrontendConfigEndpointReturnsPublicReviewVocabulary(t *testing.T) {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	cfg := &config.Config{
		KeycloakURL:           "https://keycloak.example",
		KeycloakRealm:         "registry",
		KeycloakClientID:      "frontend",
		ReviewTypes:           []string{"legal", "quality"},
		ReviewOutcomes:        []string{"approve", "reject", "needs-work"},
		ReviewFailureOutcome:  "reject",
		ReviewOverrideOutcome: "overridden",
		EnableAnonymousAuth:   true,
	}
	v0.RegisterFrontendConfigEndpoint(api, "/v0", cfg)

	req := httptest.NewRequest(http.MethodGet, "/v0/config/frontend", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, []string{
		"$schema",
		"anonymous_auth_enabled",
		"keycloak_client_id",
		"keycloak_realm",
		"keycloak_url",
		"review_failure_outcome",
		"review_outcomes",
		"review_override_outcome",
		"review_types",
	}, sortedKeys(body))

	var reviewTypes []string
	require.NoError(t, json.Unmarshal(body["review_types"], &reviewTypes))
	require.Equal(t, []string{"legal", "quality"}, reviewTypes)

	var reviewOutcomes []string
	require.NoError(t, json.Unmarshal(body["review_outcomes"], &reviewOutcomes))
	require.Equal(t, []string{"approve", "reject", "needs-work"}, reviewOutcomes)

	var reviewOverrideOutcome string
	require.NoError(t, json.Unmarshal(body["review_override_outcome"], &reviewOverrideOutcome))
	require.Equal(t, "overridden", reviewOverrideOutcome)

	var reviewFailureOutcome string
	require.NoError(t, json.Unmarshal(body["review_failure_outcome"], &reviewFailureOutcome))
	require.Equal(t, "reject", reviewFailureOutcome)
}

func sortedKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
