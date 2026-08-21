package auth

import (
	"context"
	"net/http"
	"strings"

	registryauth "github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	"github.com/agentregistry-dev/agentregistry/pkg/types"
	"github.com/danielgtaylor/huma/v2"
)

// CurrentPrincipalResponse contains the authenticated caller's identity.
type CurrentPrincipalResponse struct {
	Subject     string              `json:"subject"`
	DisplayName string              `json:"display_name"`
	AuthMethod  registryauth.Method `json:"auth_method"`
}

// RegisterCurrentPrincipalEndpoint registers the endpoint for the authenticated caller's identity.
func RegisterCurrentPrincipalEndpoint(api huma.API, pathPrefix string) {
	huma.Register(api, huma.Operation{
		OperationID: "get-current-principal" + strings.ReplaceAll(pathPrefix, "/", "-"),
		Method:      http.MethodGet,
		Path:        pathPrefix + "/auth/me",
		Summary:     "Get the current principal",
		Description: "Returns the authenticated caller's subject, display name, and authentication method.",
		Tags:        []string{"auth"},
	}, func(ctx context.Context, _ *struct{}) (*types.Response[CurrentPrincipalResponse], error) {
		session, ok := registryauth.AuthSessionFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("Authentication required")
		}

		user := session.Principal().User
		if user.AuthMethod == registryauth.MethodNone {
			// Anonymous callers share the "anonymous" subject, so it is not identifying.
			user.Subject = ""
			user.DisplayName = ""
		}

		return &types.Response[CurrentPrincipalResponse]{
			Body: CurrentPrincipalResponse{
				Subject:     user.Subject,
				DisplayName: user.DisplayName,
				AuthMethod:  user.AuthMethod,
			},
		}, nil
	})
}
