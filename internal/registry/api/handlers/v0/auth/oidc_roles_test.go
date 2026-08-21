package auth_test

import (
	"bytes"
	"log/slog"
	"testing"

	v0auth "github.com/agentregistry-dev/agentregistry/internal/registry/api/handlers/v0/auth"
	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupDottedPath(t *testing.T) {
	claims := map[string]any{
		"realm_access": map[string]any{
			"roles": []any{"registry-admin", "other-role"},
		},
		"name": "Ada Lovelace",
		"flat": "value",
	}

	tests := []struct {
		name    string
		path    string
		wantVal any
		wantOK  bool
	}{
		{"nested path found", "realm_access.roles", []any{"registry-admin", "other-role"}, true},
		{"flat path found", "flat", "value", true},
		{"single segment name", "name", "Ada Lovelace", true},
		{"missing top-level key", "missing", nil, false},
		{"missing nested key", "realm_access.missing", nil, false},
		{"path traverses through non-map", "flat.sub", nil, false},
		{"empty path", "", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := v0auth.LookupDottedPath(claims, tt.path)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantVal, got)
			}
		})
	}
}

func TestNewRoleMappingConfigFromStrings(t *testing.T) {
	t.Run("valid config parses role map and patterns", func(t *testing.T) {
		cfg, err := v0auth.NewRoleMappingConfigFromStrings(
			"realm_access.roles",
			"name",
			`{"registry-admin":"admin","registry-curator":"curator","registry-user":"user"}`,
			"io.example.*, example-*",
			"io.example.curated.*",
			"io.example.admin.*",
		)
		require.NoError(t, err)
		assert.Equal(t, v0auth.InternalRoleAdmin, cfg.RoleMap["registry-admin"])
		assert.Equal(t, v0auth.InternalRoleCurator, cfg.RoleMap["registry-curator"])
		assert.Equal(t, v0auth.InternalRoleUser, cfg.RoleMap["registry-user"])
		assert.Equal(t, []string{"io.example.*", "example-*"}, cfg.Patterns[v0auth.InternalRoleUser])
		assert.Equal(t, []string{"io.example.curated.*"}, cfg.Patterns[v0auth.InternalRoleCurator])
		assert.Equal(t, []string{"io.example.admin.*"}, cfg.Patterns[v0auth.InternalRoleAdmin])
	})

	t.Run("empty role map JSON yields empty map, no error", func(t *testing.T) {
		cfg, err := v0auth.NewRoleMappingConfigFromStrings("", "", "", "", "", "")
		require.NoError(t, err)
		assert.Empty(t, cfg.RoleMap)
	})

	t.Run("malformed JSON errors", func(t *testing.T) {
		_, err := v0auth.NewRoleMappingConfigFromStrings("realm_access.roles", "", "{not-json", "", "", "")
		require.Error(t, err)
	})

	t.Run("unknown internal role errors", func(t *testing.T) {
		_, err := v0auth.NewRoleMappingConfigFromStrings("realm_access.roles", "", `{"ext":"superuser"}`, "", "", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid internal role")
	})
}

func TestResolveRolePermissions(t *testing.T) {
	baseCfg := func() *v0auth.RoleMappingConfig {
		cfg, err := v0auth.NewRoleMappingConfigFromStrings(
			"realm_access.roles",
			"name",
			`{"registry-admin":"admin","registry-curator":"curator","registry-user":"user"}`,
			"io.example.*",
			"io.example.*",
			"io.example.*",
		)
		require.NoError(t, err)
		return cfg
	}

	t.Run("user role yields read+publish+edit_own", func(t *testing.T) {
		claims := map[string]any{
			"realm_access": map[string]any{"roles": []any{"registry-user"}},
			"name":         "Ada Lovelace",
		}
		perms, displayName, matched := v0auth.ResolveRolePermissions(claims, baseCfg())
		require.True(t, matched)
		assert.Equal(t, "Ada Lovelace", displayName)
		assert.ElementsMatch(t, []auth.Permission{
			{Action: auth.PermissionActionRead, ResourcePattern: "io.example.*"},
			{Action: auth.PermissionActionPublish, ResourcePattern: "io.example.*"},
			{Action: auth.PermissionActionEditOwn, ResourcePattern: "io.example.*"},
		}, perms)
	})

	t.Run("curator role adds edit+delete on top of edit_own", func(t *testing.T) {
		claims := map[string]any{
			"realm_access": map[string]any{"roles": []any{"registry-curator"}},
		}
		perms, _, matched := v0auth.ResolveRolePermissions(claims, baseCfg())
		require.True(t, matched)
		actions := actionSet(perms)
		assert.ElementsMatch(t, []auth.PermissionAction{
			auth.PermissionActionRead, auth.PermissionActionPublish,
			auth.PermissionActionEditOwn, auth.PermissionActionEdit, auth.PermissionActionDelete,
		}, actions)
	})

	t.Run("admin role adds deploy on top of curator", func(t *testing.T) {
		claims := map[string]any{
			"realm_access": map[string]any{"roles": []any{"registry-admin"}},
		}
		perms, _, matched := v0auth.ResolveRolePermissions(claims, baseCfg())
		require.True(t, matched)
		actions := actionSet(perms)
		assert.ElementsMatch(t, []auth.PermissionAction{
			auth.PermissionActionRead, auth.PermissionActionPublish,
			auth.PermissionActionEditOwn, auth.PermissionActionEdit,
			auth.PermissionActionDelete, auth.PermissionActionDeploy,
		}, actions)
	})

	t.Run("admin bundle never grants bare wildcard bypass pattern", func(t *testing.T) {
		claims := map[string]any{
			"realm_access": map[string]any{"roles": []any{"registry-admin"}},
		}
		perms, _, matched := v0auth.ResolveRolePermissions(claims, baseCfg())
		require.True(t, matched)
		for _, p := range perms {
			assert.NotEqual(t, "*", p.ResourcePattern, "role bundles must never grant bare '*' (would trip IsRegistryAdmin's unbounded bypass)")
		}
	})

	t.Run("multiple roles: most privileged bundle wins", func(t *testing.T) {
		claims := map[string]any{
			"realm_access": map[string]any{"roles": []any{"registry-user", "registry-admin"}},
		}
		perms, _, matched := v0auth.ResolveRolePermissions(claims, baseCfg())
		require.True(t, matched)
		actions := actionSet(perms)
		assert.Contains(t, actions, auth.PermissionActionDeploy, "admin (highest ranked matched role) should win over user")
	})

	t.Run("unrecognized external role falls through unmatched", func(t *testing.T) {
		var logBuf bytes.Buffer
		prevLogger := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, nil)))
		defer slog.SetDefault(prevLogger)

		claims := map[string]any{
			"realm_access": map[string]any{"roles": []any{"some-other-role"}},
		}
		perms, _, matched := v0auth.ResolveRolePermissions(claims, baseCfg())
		assert.False(t, matched)
		assert.Nil(t, perms)
		assert.Empty(t, logBuf.String(), "an ordinary unrecognized-role caller (e.g. a caller with no registry role at all) must not log a warning - only a matched-but-unconfigured role is a misconfiguration")
	})

	t.Run("missing role claim falls through unmatched", func(t *testing.T) {
		perms, _, matched := v0auth.ResolveRolePermissions(map[string]any{}, baseCfg())
		assert.False(t, matched)
		assert.Nil(t, perms)
	})

	t.Run("nil config falls through unmatched", func(t *testing.T) {
		perms, displayName, matched := v0auth.ResolveRolePermissions(map[string]any{"name": "X"}, nil)
		assert.False(t, matched)
		assert.Nil(t, perms)
		assert.Empty(t, displayName)
	})

	t.Run("role claim path unset disables role mapping entirely", func(t *testing.T) {
		cfg, err := v0auth.NewRoleMappingConfigFromStrings("", "", `{"registry-admin":"admin"}`, "io.example.*", "io.example.*", "io.example.*")
		require.NoError(t, err)
		claims := map[string]any{
			"realm_access": map[string]any{"roles": []any{"registry-admin"}},
		}
		perms, _, matched := v0auth.ResolveRolePermissions(claims, cfg)
		assert.False(t, matched)
		assert.Nil(t, perms)
	})

	t.Run("matched role with no configured patterns falls back unmatched and logs a warning", func(t *testing.T) {
		var logBuf bytes.Buffer
		prevLogger := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, nil)))
		defer slog.SetDefault(prevLogger)

		cfg, err := v0auth.NewRoleMappingConfigFromStrings(
			"realm_access.roles", "", `{"registry-admin":"admin"}`,
			"", "", "", // no patterns configured for any role
		)
		require.NoError(t, err)
		claims := map[string]any{
			"realm_access": map[string]any{"roles": []any{"registry-admin"}},
		}
		perms, _, matched := v0auth.ResolveRolePermissions(claims, cfg)
		assert.False(t, matched, "a matched role with an empty pattern list must not silently grant zero permissions as a match")
		assert.Nil(t, perms)

		logged := logBuf.String()
		assert.Contains(t, logged, "level=WARN", "misconfiguration (matched role, no patterns) must be logged, not silent")
		assert.Contains(t, logged, "no configured resource patterns")
		assert.Contains(t, logged, "role=admin")
	})

	t.Run("display name resolved independently of role match", func(t *testing.T) {
		cfg, err := v0auth.NewRoleMappingConfigFromStrings("realm_access.roles", "name", "", "", "", "")
		require.NoError(t, err)
		claims := map[string]any{
			"realm_access": map[string]any{"roles": []any{"unrecognized"}},
			"name":         "Grace Hopper",
		}
		_, displayName, matched := v0auth.ResolveRolePermissions(claims, cfg)
		assert.False(t, matched)
		assert.Equal(t, "Grace Hopper", displayName, "display name should resolve even when role matching fails")
	})
}

func actionSet(perms []auth.Permission) []auth.PermissionAction {
	actions := make([]auth.PermissionAction, 0, len(perms))
	for _, p := range perms {
		actions = append(actions, p.Action)
	}
	return actions
}
