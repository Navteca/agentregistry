package auth_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/agentregistry-dev/agentregistry/internal/registry/api/handlers/v0/auth"
	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	registryauth "github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockGenericOIDCValidator for testing
type MockGenericOIDCValidator struct {
	validateFunc func(ctx context.Context, token string) (*auth.OIDCClaims, error)
}

func (m *MockGenericOIDCValidator) ValidateToken(ctx context.Context, token string) (*auth.OIDCClaims, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, token)
	}
	return nil, fmt.Errorf("not implemented")
}

func TestOIDCHandler_ExchangeToken(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.Config
		mockValidator *MockGenericOIDCValidator
		token         string
		expectedError bool
	}{
		{
			name: "successful token exchange with publish permissions",
			config: &config.Config{
				OIDCEnabled:      true,
				OIDCIssuer:       "https://accounts.google.com",
				OIDCClientID:     "test-client-id",
				OIDCExtraClaims:  `[{"hd":"modelcontextprotocol.io"}]`,
				OIDCPublishPerms: "*",
				JWTPrivateKey:    "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", // 32 byte hex
			},
			mockValidator: &MockGenericOIDCValidator{
				validateFunc: func(_ context.Context, _ string) (*auth.OIDCClaims, error) {
					return &auth.OIDCClaims{
						Subject: "test-subject",
						ExtraClaims: map[string]any{
							"email":              "admin@modelcontextprotocol.io",
							"email_verified":     true,
							"hd":                 "modelcontextprotocol.io",
							"preferred_username": "admin",
						},
					}, nil
				},
			},
			token:         "valid-oidc-token",
			expectedError: false,
		},
		{
			name: "failed validation with invalid hosted domain",
			config: &config.Config{
				OIDCEnabled:      true,
				OIDCIssuer:       "https://accounts.google.com",
				OIDCClientID:     "test-client-id",
				OIDCExtraClaims:  `[{"hd":"modelcontextprotocol.io"}]`,
				OIDCPublishPerms: "*",
				JWTPrivateKey:    "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			},
			mockValidator: &MockGenericOIDCValidator{
				validateFunc: func(_ context.Context, _ string) (*auth.OIDCClaims, error) {
					return &auth.OIDCClaims{
						ExtraClaims: map[string]any{
							"email":              "user@example.com",
							"email_verified":     true,
							"hd":                 "example.com", // Wrong domain
							"preferred_username": "user",
						},
					}, nil
				},
			},
			token:         "invalid-domain-token",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := auth.NewOIDCHandler(tt.config)
			if tt.mockValidator != nil {
				handler.SetValidator(tt.mockValidator)
			}

			ctx := context.Background()
			response, err := handler.ExchangeToken(ctx, tt.token)

			if tt.expectedError {
				require.Error(t, err)
				assert.Nil(t, response)
			} else {
				require.NoError(t, err)
				require.NotNil(t, response)
				assert.NotEmpty(t, response.RegistryToken)
				assert.Positive(t, response.ExpiresAt)
			}
		})
	}
}

func TestOIDCHandler_ExchangeTokenRolePermissions(t *testing.T) {
	cfg := &config.Config{
		OIDCEnabled:              true,
		OIDCIssuer:               "https://accounts.google.com",
		OIDCClientID:             "test-client-id",
		OIDCRoleClaimPath:        "realm_access.roles",
		OIDCRoleMap:              `{"registry-user":"user"}`,
		OIDCUserPatterns:         "io.example.*",
		OIDCDisplayNameClaimPath: "preferred_username",
		OIDCReadPerms:            "static-read",
		OIDCPublishPerms:         "static-publish",
		OIDCPushPerms:            "static-push",
		OIDCDeployPerms:          "static-deploy",
		OIDCEditPerms:            "static-edit",
		OIDCDeletePerms:          "static-delete",
		JWTPrivateKey:            "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	}
	handler := auth.NewOIDCHandler(cfg)
	jwtManager := registryauth.NewJWTManager(cfg)

	tests := []struct {
		name                string
		subject             string
		extraClaims         map[string]any
		expectedPerm        []registryauth.Permission
		expectedDisplayName string
	}{
		{
			name:    "mapped role gets role bundle and display name",
			subject: "subject-mapped",
			extraClaims: map[string]any{
				"realm_access":       map[string]any{"roles": []any{"registry-user"}},
				"preferred_username": "Ada",
			},
			expectedPerm: []registryauth.Permission{
				{Action: registryauth.PermissionActionRead, ResourcePattern: "io.example.*"},
				{Action: registryauth.PermissionActionPublish, ResourcePattern: "io.example.*"},
				{Action: registryauth.PermissionActionEditOwn, ResourcePattern: "io.example.*"},
			},
			expectedDisplayName: "Ada",
		},
		{
			name:    "unknown role gets static fallback and display name",
			subject: "subject-unmatched",
			extraClaims: map[string]any{
				"realm_access":       map[string]any{"roles": []any{"unrecognized"}},
				"preferred_username": "Grace",
			},
			expectedPerm:        staticOIDCPermissions(),
			expectedDisplayName: "Grace",
		},
		{
			name:                "absent display name falls back to subject",
			subject:             "subject-no-name",
			extraClaims:         map[string]any{"email": "user@example.com"},
			expectedPerm:        staticOIDCPermissions(),
			expectedDisplayName: "subject-no-name",
		},
		{
			name:    "empty display name falls back to subject",
			subject: "subject-empty-name",
			extraClaims: map[string]any{
				"realm_access":       "not-a-map",
				"preferred_username": "",
			},
			expectedPerm:        staticOIDCPermissions(),
			expectedDisplayName: "subject-empty-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler.SetValidator(&MockGenericOIDCValidator{
				validateFunc: func(_ context.Context, _ string) (*auth.OIDCClaims, error) {
					return &auth.OIDCClaims{
						Subject:     tt.subject,
						ExtraClaims: tt.extraClaims,
					}, nil
				},
			})

			response, err := handler.ExchangeToken(context.Background(), "valid-oidc-token")
			require.NoError(t, err)

			claims, err := jwtManager.ValidateToken(context.Background(), response.RegistryToken)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.expectedPerm, claims.Permissions)
			assert.Equal(t, tt.expectedDisplayName, claims.AuthMethodDisplayName)
		})
	}
}

func staticOIDCPermissions() []registryauth.Permission {
	return []registryauth.Permission{
		{Action: registryauth.PermissionActionRead, ResourcePattern: "static-read"},
		{Action: registryauth.PermissionActionPublish, ResourcePattern: "static-push"},
		{Action: registryauth.PermissionActionDeploy, ResourcePattern: "static-deploy"},
		{Action: registryauth.PermissionActionPublish, ResourcePattern: "static-publish"},
		{Action: registryauth.PermissionActionEdit, ResourcePattern: "static-edit"},
		{Action: registryauth.PermissionActionDelete, ResourcePattern: "static-delete"},
	}
}
