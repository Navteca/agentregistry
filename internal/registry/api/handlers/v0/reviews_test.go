package v0_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	v0 "github.com/agentregistry-dev/agentregistry/internal/registry/api/handlers/v0"
	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	internaldb "github.com/agentregistry-dev/agentregistry/internal/registry/database"
	"github.com/agentregistry-dev/agentregistry/internal/registry/service"
	servicetesting "github.com/agentregistry-dev/agentregistry/internal/registry/service/testing"
	"github.com/agentregistry-dev/agentregistry/pkg/models"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reviewEndpointSession struct {
	user auth.User
}

func (s *reviewEndpointSession) Principal() auth.Principal {
	return auth.Principal{User: s.user}
}

func reviewEndpointContext(subject, displayName string, permissions []auth.Permission) context.Context {
	return auth.AuthSessionTo(context.Background(), &reviewEndpointSession{
		user: auth.User{
			Permissions: permissions,
			AuthMethod:  auth.MethodOIDC,
			Subject:     subject,
			DisplayName: displayName,
		},
	})
}

func reviewEndpointAuthorizer(t *testing.T) auth.Authorizer {
	t.Helper()

	seed := make([]byte, ed25519.SeedSize)
	_, err := rand.Read(seed)
	require.NoError(t, err)

	jwtManager := auth.NewJWTManager(&config.Config{
		JWTPrivateKey: hex.EncodeToString(seed),
	})
	return auth.Authorizer{Authz: auth.NewPublicAuthzProvider(jwtManager)}
}

func newReviewEndpoint(t *testing.T, cfg *config.Config, registry service.RegistryService) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Review API", "1.0.0"))
	v0.RegisterReviewsEndpoint(api, "/v0", cfg, registry)
	return mux
}

func reviewRequest(t *testing.T, path, body string, ctx context.Context) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(ctx)
}

func TestCreateReviewEndpointRejectsUnconfiguredValues(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "review type",
			body: `{"review_type":"scientific","outcome":"pass","notes":"finding"}`,
		},
		{
			name: "outcome",
			body: `{"review_type":"security","outcome":"conditional","notes":"finding"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				ReviewTypes:    []string{"security"},
				ReviewOutcomes: []string{"pass"},
			}
			fake := servicetesting.NewFakeRegistry()
			called := false
			fake.CreateReviewFn = func(context.Context, string, string, string, string, string, string) (*models.Review, error) {
				called = true
				return &models.Review{}, nil
			}

			mux := newReviewEndpoint(t, cfg, fake)
			req := reviewRequest(
				t,
				"/v0/reviews/server/"+url.PathEscape("com.example/review-target")+"/versions/1.0.0",
				tt.body,
				context.Background(),
			)
			resp := httptest.NewRecorder()
			mux.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusBadRequest, resp.Code)
			assert.False(t, called, "service must not be called for invalid configured values")
		})
	}
}

func TestCreateReviewEndpointUsesTokenIdentity(t *testing.T) {
	cfg := &config.Config{
		ReviewTypes:    []string{"security"},
		ReviewOutcomes: []string{"pass"},
	}
	fake := servicetesting.NewFakeRegistry()
	fake.CreateReviewFn = func(ctx context.Context, artifactType, artifactName, artifactVersion, reviewType, outcome, notes string) (*models.Review, error) {
		session, ok := auth.AuthSessionFrom(ctx)
		if !ok {
			return nil, auth.ErrUnauthenticated
		}
		user := session.Principal().User
		return &models.Review{
			ArtifactType:        artifactType,
			ArtifactName:        artifactName,
			ArtifactVersion:     artifactVersion,
			ReviewType:          reviewType,
			Outcome:             outcome,
			ReviewerSubject:     user.Subject,
			ReviewerAuthMethod:  string(user.AuthMethod),
			ReviewerDisplayName: user.DisplayName,
			Notes:               notes,
		}, nil
	}

	mux := newReviewEndpoint(t, cfg, fake)
	req := reviewRequest(
		t,
		"/v0/reviews/server/"+url.PathEscape("com.example/review-target")+"/versions/1.0.0",
		`{"review_type":"security","outcome":"pass","notes":"all clear","reviewer_subject":"attacker","reviewer_auth_method":"fake","reviewer_display_name":"Attacker"}`,
		reviewEndpointContext("token-subject", "Token Reviewer", []auth.Permission{
			{Action: auth.PermissionActionReview, ResourcePattern: "*"},
		}),
	)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	var got models.Review
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, "token-subject", got.ReviewerSubject)
	assert.Equal(t, "oidc", got.ReviewerAuthMethod)
	assert.Equal(t, "Token Reviewer", got.ReviewerDisplayName)
	assert.NotEqual(t, "attacker", got.ReviewerSubject)
}

func TestCreateReviewEndpointRefusesUserPermission(t *testing.T) {
	db := internaldb.NewTestDB(t)
	cfg := &config.Config{
		ReviewTypes:    []string{"security"},
		ReviewOutcomes: []string{"pass"},
	}
	registry := service.NewRegistryService(db, cfg, nil, reviewEndpointAuthorizer(t))
	mux := newReviewEndpoint(t, cfg, registry)

	req := reviewRequest(
		t,
		"/v0/reviews/server/"+url.PathEscape("com.example/review-target")+"/versions/1.0.0",
		`{"review_type":"security","outcome":"pass","notes":"should be forbidden"}`,
		reviewEndpointContext("user-subject", "User", []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionEditOwn, ResourcePattern: "*"},
		}),
	)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code)
}
