package auth_test

import (
	"context"
	"testing"

	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	"github.com/stretchr/testify/assert"
)

type fakeSession struct {
	permissions []auth.Permission
}

func (s *fakeSession) Principal() auth.Principal {
	return auth.Principal{User: auth.User{Permissions: s.permissions}}
}

func TestPublicAuthzProvider_IsRegistryAdmin(t *testing.T) {
	provider := auth.NewPublicAuthzProvider(nil)
	ctx := context.Background()

	t.Run("nil session is never admin", func(t *testing.T) {
		assert.False(t, provider.IsRegistryAdmin(ctx, nil))
	})

	t.Run("system session is always admin", func(t *testing.T) {
		assert.True(t, provider.IsRegistryAdmin(ctx, &auth.SystemSession{}))
	})

	t.Run("read + wildcard does not grant the bypass", func(t *testing.T) {
		s := &fakeSession{permissions: []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
		}}
		assert.False(t, provider.IsRegistryAdmin(ctx, s))
	})

	t.Run("delete + wildcard does not grant the bypass", func(t *testing.T) {
		s := &fakeSession{permissions: []auth.Permission{
			{Action: auth.PermissionActionDelete, ResourcePattern: "*"},
		}}
		assert.False(t, provider.IsRegistryAdmin(ctx, s))
	})

	t.Run("admin action + wildcard grants the bypass", func(t *testing.T) {
		s := &fakeSession{permissions: []auth.Permission{
			{Action: auth.PermissionActionAdmin, ResourcePattern: "*"},
		}}
		assert.True(t, provider.IsRegistryAdmin(ctx, s))
	})

	t.Run("admin action with a scoped pattern does not grant the bypass", func(t *testing.T) {
		// The bypass is reserved for the true wildcard; a scoped admin grant
		// (e.g. "io.github.someorg/*") is not global and must go through
		// normal per-resource Check().
		s := &fakeSession{permissions: []auth.Permission{
			{Action: auth.PermissionActionAdmin, ResourcePattern: "io.github.someorg/*"},
		}}
		assert.False(t, provider.IsRegistryAdmin(ctx, s))
	})

	t.Run("mixed non-admin wildcard permissions never combine into a bypass", func(t *testing.T) {
		s := &fakeSession{permissions: []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "*"},
			{Action: auth.PermissionActionEdit, ResourcePattern: "*"},
			{Action: auth.PermissionActionDelete, ResourcePattern: "*"},
			{Action: auth.PermissionActionPublish, ResourcePattern: "*"},
			{Action: auth.PermissionActionDeploy, ResourcePattern: "*"},
		}}
		assert.False(t, provider.IsRegistryAdmin(ctx, s))
	})
}
