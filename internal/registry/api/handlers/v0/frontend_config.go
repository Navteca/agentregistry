package v0

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
)

// FrontendConfigBody holds public frontend bootstrap configuration values.
type FrontendConfigBody struct {
	KeycloakURL           string   `json:"keycloak_url"`
	KeycloakRealm         string   `json:"keycloak_realm"`
	KeycloakClientID      string   `json:"keycloak_client_id"`
	APIBaseURL            string   `json:"api_base_url,omitempty"`
	GatewayBaseURL        string   `json:"gateway_base_url,omitempty"`
	AnonymousAuth         bool     `json:"anonymous_auth_enabled"`
	ShowGithubLink        bool     `json:"show_github_link"`
	ShowDiscordLink       bool     `json:"show_discord_link"`
	ReviewTypes           []string `json:"review_types"`
	ReviewOutcomes        []string `json:"review_outcomes"`
	ReviewFailureOutcome  string   `json:"review_failure_outcome"`
	ReviewOverrideOutcome string   `json:"review_override_outcome"`
}

// frontendConfigOutput wraps FrontendConfigBody and adds a Cache-Control response header
// so browsers and CDNs may cache the largely-static OIDC config for up to 5 minutes.
type frontendConfigOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         FrontendConfigBody
}

// RegisterFrontendConfigEndpoint registers GET /v0/config/frontend.
// The endpoint is unauthenticated; it exposes only deployment-wide values that
// the browser requires to initialise keycloak-js and render review controls.
func RegisterFrontendConfigEndpoint(api huma.API, pathPrefix string, cfg *config.Config) {
	huma.Register(api, huma.Operation{
		OperationID: "get-frontend-config",
		Method:      http.MethodGet,
		Path:        pathPrefix + "/config/frontend",
		Summary:     "Frontend OIDC and review configuration",
		Description: "Returns deployment-wide configuration required by the browser to initialise OIDC authentication and render review controls. This endpoint is intentionally unauthenticated; it contains no identity-dependent data.",
		Tags:        []string{"auth"},
		Security:    []map[string][]string{},
	}, func(_ context.Context, _ *struct{}) (*frontendConfigOutput, error) {
		reviewConfig := cfg.ReviewConfig()
		return &frontendConfigOutput{
			CacheControl: "public, max-age=300",
			Body: FrontendConfigBody{
				KeycloakURL:           cfg.KeycloakURL,
				KeycloakRealm:         cfg.KeycloakRealm,
				KeycloakClientID:      cfg.KeycloakClientID,
				APIBaseURL:            cfg.FrontendAPIURL,
				GatewayBaseURL:        cfg.FrontendGatewayURL,
				AnonymousAuth:         cfg.EnableAnonymousAuth,
				ShowGithubLink:        cfg.ShowGithubLink,
				ShowDiscordLink:       cfg.ShowDiscordLink,
				ReviewTypes:           reviewConfig.Types(),
				ReviewOutcomes:        reviewConfig.Outcomes(),
				ReviewFailureOutcome:  reviewConfig.FailureOutcome(),
				ReviewOverrideOutcome: reviewConfig.OverrideOutcome(),
			},
		}, nil
	})
}
