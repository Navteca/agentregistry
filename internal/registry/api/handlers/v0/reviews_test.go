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
	"github.com/agentregistry-dev/agentregistry/pkg/registry/database"
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

func reviewGetRequest(t *testing.T, path string, ctx context.Context) *http.Request {
	t.Helper()

	return httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
}

func TestListReviewsEndpointReturnsAllRowsAndMarkers(t *testing.T) {
	cfg := &config.Config{
		ReviewTypes:    []string{"security"},
		ReviewOutcomes: []string{"pass"},
	}
	fake := servicetesting.NewFakeRegistry()
	current := true
	stale := false
	fake.GetReviewsFn = func(ctx context.Context, artifactType, artifactName, artifactVersion string) ([]models.Review, error) {
		assert.Equal(t, "server", artifactType)
		assert.Equal(t, "com.example/review-target", artifactName)
		assert.Equal(t, "1.0.0", artifactVersion)
		return []models.Review{{
			ID:                  1,
			ArtifactType:        artifactType,
			ArtifactName:        artifactName,
			ArtifactVersion:     artifactVersion,
			ReviewType:          "security",
			Outcome:             "pass",
			ReviewerSubject:     "subject",
			ReviewerAuthMethod:  "oidc",
			ReviewerDisplayName: "Reviewer",
			Notes:               "finding",
			IsCurrent:           &current,
			IsStale:             &stale,
		}}, nil
	}

	mux := newReviewEndpoint(t, cfg, fake)
	req := reviewGetRequest(
		t,
		"/v0/reviews/server/"+url.PathEscape("com.example/review-target")+"/versions/1.0.0",
		context.Background(),
	)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	var reviews []models.Review
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&reviews))
	require.Len(t, reviews, 1)
	assert.Equal(t, "finding", reviews[0].Notes)
	assert.True(t, *reviews[0].IsCurrent)
}

func TestListReviewsEndpointMapsMissingArtifactToNotFound(t *testing.T) {
	cfg := &config.Config{
		ReviewTypes:    []string{"security"},
		ReviewOutcomes: []string{"pass"},
	}
	fake := servicetesting.NewFakeRegistry()
	fake.GetReviewsFn = func(context.Context, string, string, string) ([]models.Review, error) {
		return nil, database.ErrNotFound
	}

	mux := newReviewEndpoint(t, cfg, fake)
	req := reviewGetRequest(t, "/v0/reviews/server/missing/versions/1.0.0", context.Background())
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
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

func TestCreateReviewOverrideEndpointUsesServerIdentityAndReason(t *testing.T) {
	cfg := &config.Config{
		ReviewTypes:           []string{"security"},
		ReviewOutcomes:        []string{"pass", "fail", "override"},
		ReviewFailureOutcome:  "fail",
		ReviewOverrideOutcome: "override",
	}
	fake := servicetesting.NewFakeRegistry()
	fake.CreateReviewOverrideFn = func(ctx context.Context, artifactType, artifactName, artifactVersion string, targetReviewID int64, reason string) (*models.Review, error) {
		session, ok := auth.AuthSessionFrom(ctx)
		require.True(t, ok)
		user := session.Principal().User
		return &models.Review{
			ID:                  2,
			ArtifactType:        artifactType,
			ArtifactName:        artifactName,
			ArtifactVersion:     artifactVersion,
			ReviewType:          "security",
			Outcome:             "override",
			ReviewerSubject:     user.Subject,
			ReviewerAuthMethod:  string(user.AuthMethod),
			ReviewerDisplayName: user.DisplayName,
			Notes:               reason,
			OverridesReviewID:   &targetReviewID,
		}, nil
	}

	mux := newReviewEndpoint(t, cfg, fake)
	req := reviewRequest(
		t,
		"/v0/reviews/server/"+url.PathEscape("com.example/review-target")+"/versions/1.0.0/overrides",
		`{"review_id":1,"reason":"accepted risk"}`,
		reviewEndpointContext("admin-subject", "Admin", []auth.Permission{
			{Action: auth.PermissionActionOverride, ResourcePattern: "*"},
		}),
	)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	var got models.Review
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.True(t, got.IsOverride())
	assert.Equal(t, int64(1), *got.OverridesReviewID)
	assert.Equal(t, "admin-subject", got.ReviewerSubject)
	assert.Equal(t, "accepted risk", got.Notes)
}

func TestCreateReviewOverrideEndpointRejectsEmptyReason(t *testing.T) {
	cfg := &config.Config{
		ReviewTypes:           []string{"security"},
		ReviewOutcomes:        []string{"pass", "fail", "override"},
		ReviewFailureOutcome:  "fail",
		ReviewOverrideOutcome: "override",
	}
	fake := servicetesting.NewFakeRegistry()
	called := false
	fake.CreateReviewOverrideFn = func(context.Context, string, string, string, int64, string) (*models.Review, error) {
		called = true
		return &models.Review{}, nil
	}

	mux := newReviewEndpoint(t, cfg, fake)
	req := reviewRequest(
		t,
		"/v0/reviews/server/"+url.PathEscape("com.example/review-target")+"/versions/1.0.0/overrides",
		`{"review_id":1,"reason":"   "}`,
		context.Background(),
	)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.False(t, called)
}
