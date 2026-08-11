package v0

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
)

// FrontendConfigBody holds public frontend bootstrap configuration values.
type FrontendConfigBody struct {
	KeycloakURL      string `json:"keycloak_url"`
	KeycloakRealm    string `json:"keycloak_realm"`
	KeycloakClientID string `json:"keycloak_client_id"`
	APIBaseURL       string `json:"api_base_url,omitempty"`
	GatewayBaseURL   string `json:"gateway_base_url,omitempty"`
	AnonymousAuth    bool   `json:"anonymous_auth_enabled"`
}

// frontendConfigOutput wraps FrontendConfigBody and adds a Cache-Control response header
// so browsers and CDNs may cache the largely-static OIDC config for up to 5 minutes.
type frontendConfigOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         FrontendConfigBody
}

// RegisterFrontendConfigEndpoint registers GET /v0/config/frontend.
// The endpoint is unauthenticated; it exposes only the three public OIDC values
// that the browser requires to initialise keycloak-js at runtime.
func RegisterFrontendConfigEndpoint(api huma.API, pathPrefix string, cfg *config.Config) {
	huma.Register(api, huma.Operation{
		OperationID: "get-frontend-config",
		Method:      http.MethodGet,
		Path:        pathPrefix + "/config/frontend",
		Summary:     "Frontend OIDC configuration",
		Description: "Returns the Keycloak URL, realm, and client ID required by the browser to initialise OIDC authentication. This endpoint is intentionally unauthenticated.",
		Tags:        []string{"auth"},
		Security:    []map[string][]string{},
	}, func(_ context.Context, _ *struct{}) (*frontendConfigOutput, error) {
		return &frontendConfigOutput{
			CacheControl: "public, max-age=300",
			Body: FrontendConfigBody{
				KeycloakURL:      cfg.KeycloakURL,
				KeycloakRealm:    cfg.KeycloakRealm,
				KeycloakClientID: cfg.KeycloakClientID,
				APIBaseURL:       cfg.FrontendAPIURL,
				GatewayBaseURL:   cfg.FrontendGatewayURL,
				AnonymousAuth:    cfg.EnableAnonymousAuth,
			},
		}, nil
	})
}
