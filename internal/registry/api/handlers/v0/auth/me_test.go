package auth_test

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v0auth "github.com/agentregistry-dev/agentregistry/internal/registry/api/handlers/v0/auth"
	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	registryauth "github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentPrincipalEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		claims         *registryauth.JWTClaims
		withToken      bool
		expectedStatus int
		assertBody     func(t *testing.T, body map[string]any)
	}{
		{
			name: "authenticated caller receives identity",
			claims: &registryauth.JWTClaims{
				AuthMethod:            registryauth.MethodOIDC,
				AuthMethodSubject:     "user-subject",
				AuthMethodDisplayName: "User Display Name",
			},
			withToken:      true,
			expectedStatus: http.StatusOK,
			assertBody: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "user-subject", body["subject"])
				assert.Equal(t, "User Display Name", body["display_name"])
				assert.Equal(t, string(registryauth.MethodOIDC), body["auth_method"])
				assert.NotContains(t, body, "permissions")
				assert.NotContains(t, body, "role")
			},
		},
		{
			name:           "unauthenticated caller receives unauthorized",
			expectedStatus: http.StatusUnauthorized,
			assertBody: func(t *testing.T, body map[string]any) {
				assert.Empty(t, body)
			},
		},
		{
			name: "anonymous caller receives an anonymous identity",
			claims: &registryauth.JWTClaims{
				AuthMethod:        registryauth.MethodNone,
				AuthMethodSubject: "anonymous",
			},
			withToken:      true,
			expectedStatus: http.StatusOK,
			assertBody: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "", body["subject"])
				assert.Equal(t, "", body["display_name"])
				assert.Equal(t, string(registryauth.MethodNone), body["auth_method"])
				assert.NotContains(t, body, "permissions")
				assert.NotContains(t, body, "role")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
			jwtManager := registryauth.NewJWTManager(&config.Config{
				JWTPrivateKey: hex.EncodeToString(make([]byte, ed25519.SeedSize)),
			})
			api.UseMiddleware(registryauth.AuthnMiddleware(jwtManager))
			v0auth.RegisterCurrentPrincipalEndpoint(api, "/v0")

			request := httptest.NewRequest(http.MethodGet, "/v0/auth/me", nil)
			if tt.withToken {
				tokenResponse, err := jwtManager.GenerateTokenResponse(context.Background(), *tt.claims)
				require.NoError(t, err)
				request.Header.Set("Authorization", "Bearer "+tokenResponse.RegistryToken)
			}

			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			assert.Equal(t, tt.expectedStatus, response.Code)

			if tt.expectedStatus != http.StatusOK {
				return
			}

			var body map[string]any
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			tt.assertBody(t, body)
		})
	}
}

func TestCurrentPrincipalEndpointUsesSignedTokenClaims(t *testing.T) {
	cfg := &config.Config{JWTPrivateKey: hex.EncodeToString(make([]byte, ed25519.SeedSize))}
	jwtManager := registryauth.NewJWTManager(cfg)
	claims := registryauth.JWTClaims{
		AuthMethod:            registryauth.MethodGitHubOIDC,
		AuthMethodSubject:     "github-subject",
		AuthMethodDisplayName: "GitHub User",
	}
	tokenResponse, err := jwtManager.GenerateTokenResponse(context.Background(), claims)
	require.NoError(t, err)

	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	api.UseMiddleware(registryauth.AuthnMiddleware(jwtManager))
	v0auth.RegisterCurrentPrincipalEndpoint(api, "/v0")

	request := httptest.NewRequest(http.MethodGet, "/v0/auth/me", nil)
	request.Header.Set("Authorization", "Bearer "+tokenResponse.RegistryToken)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	var body v0auth.CurrentPrincipalResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, claims.AuthMethodSubject, body.Subject)
	assert.Equal(t, claims.AuthMethodDisplayName, body.DisplayName)
	assert.Equal(t, claims.AuthMethod, body.AuthMethod)
}
