package validators

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRepositoryReachability(t *testing.T) {
	t.Run("accepts reachable repository URL", func(t *testing.T) {
		repoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer repoServer.Close()

		previousClient := repositoryHTTPClient
		repositoryHTTPClient = repoServer.Client()
		t.Cleanup(func() {
			repositoryHTTPClient = previousClient
		})

		err := ValidateRepositoryReachability(context.Background(), repoServer.URL+"/owner/repo")
		require.NoError(t, err)
	})

	t.Run("rejects unreachable repository URL", func(t *testing.T) {
		err := ValidateRepositoryReachability(context.Background(), "http://127.0.0.1:1/owner/repo")
		require.Error(t, err)
		assert.Contains(t, err.Error(), ErrRepositoryUnreachable.Error())
	})

	t.Run("falls back to get when head is not allowed", func(t *testing.T) {
		repoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer repoServer.Close()

		previousClient := repositoryHTTPClient
		repositoryHTTPClient = repoServer.Client()
		t.Cleanup(func() {
			repositoryHTTPClient = previousClient
		})

		err := ValidateRepositoryReachability(context.Background(), repoServer.URL+"/owner/repo")
		require.NoError(t, err)
	})
}

func TestValidatePublishRequest_RepositoryReachabilityEnabled(t *testing.T) {
	repoChecked := false
	repoServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repoChecked = true
		w.WriteHeader(http.StatusOK)
	}))
	defer repoServer.Close()

	previousClient := repositoryHTTPClient
	repositoryHTTPClient = repoServer.Client()
	t.Cleanup(func() {
		repositoryHTTPClient = previousClient
	})

	serverJSON := apiv0.ServerJSON{
		Schema:      model.CurrentSchemaURL,
		Name:        "com.example/test-server",
		Description: "A test server",
		Repository: &model.Repository{
			URL:    "https://github.com/owner/repo",
			Source: "git",
		},
		Version: "1.0.0",
	}

	// Rewrite outbound requests to the local TLS test server while keeping the
	// original GitHub URL for structural validation.
	repositoryHTTPClient.Transport = rewriteTransport{
		targetURL: repoServer.URL,
		base:      repoServer.Client().Transport,
	}

	err := ValidatePublishRequest(context.Background(), serverJSON, &config.Config{
		ValidateRepositoryReachability: true,
	})
	require.NoError(t, err)
	assert.True(t, repoChecked)
}

type rewriteTransport struct {
	targetURL string
	base      http.RoundTripper
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	targetReq := req.Clone(req.Context())
	targetReq.URL.Scheme = "https"
	targetReq.URL.Host = reqHostFromURL(t.targetURL)
	targetReq.Host = reqHostFromURL(t.targetURL)

	return t.base.RoundTrip(targetReq)
}

func reqHostFromURL(rawURL string) string {
	req, _ := http.NewRequest(http.MethodGet, rawURL, nil)
	return req.URL.Host
}
