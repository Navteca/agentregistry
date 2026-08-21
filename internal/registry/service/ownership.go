package service

import (
	"context"

	"github.com/agentregistry-dev/agentregistry/pkg/models"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
)

func resolveOwnership(ctx context.Context) models.OwnershipInput {
	session, ok := auth.AuthSessionFrom(ctx)
	if !ok {
		return models.OwnershipInput{}
	}

	claims := session.Principal().User
	if claims.AuthMethod == auth.MethodNone {
		return models.OwnershipInput{}
	}

	return models.OwnershipInput{
		Subject:     claims.Subject,
		DisplayName: claims.DisplayName,
		AuthMethod:  string(claims.AuthMethod),
	}
}
