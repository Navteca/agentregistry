package auth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/agentregistry-dev/agentregistry/pkg/registry/auth"
)

// InternalRole is the fixed internal role vocabulary that external OIDC
// roles are mapped to. Exactly three roles are supported by design; adding
// more (groups, tenancy, additional roles, etc.) is out of scope for AR-1 -
// see the ticket notes before extending this set.
type InternalRole string

const (
	InternalRoleUser    InternalRole = "user"
	InternalRoleCurator InternalRole = "curator"
	InternalRoleAdmin   InternalRole = "admin"
)

// roleRank orders the internal roles by privilege. Bundles are additive
// supersets (admin ⊇ curator ⊇ user), so when a caller's external roles map
// to more than one internal role, only the most privileged bundle is used.
var roleRank = map[InternalRole]int{
	InternalRoleUser:    1,
	InternalRoleCurator: 2,
	InternalRoleAdmin:   3,
}

// RoleMappingConfig configures how external OIDC roles resolve to registry
// permissions. It has no dependency on config.Config, HTTP, or any IdP
// client, so it can be constructed and tested without a request, router, or
// identity provider.
type RoleMappingConfig struct {
	// RoleClaimPath is a dotted path (e.g. "realm_access.roles") locating the
	// external role list within the OIDC claims. Empty disables role mapping
	// entirely; callers then fall back to the caller's default permission
	// bundle.
	RoleClaimPath string
	// DisplayNameClaimPath is a dotted path (e.g. "name") locating a
	// human-readable display name within the OIDC claims. Presentation only
	// - never used for authorization.
	DisplayNameClaimPath string
	// RoleMap maps external role strings to an InternalRole.
	RoleMap map[string]InternalRole
	// Patterns holds the resource-name patterns each internal role's
	// permission bundle is scoped to. A role with no configured patterns
	// cannot be granted (see buildRoleBundle).
	Patterns map[InternalRole][]string
}

// NewRoleMappingConfigFromStrings parses the comma-separated pattern lists
// and JSON role map used by configuration (see internal/registry/config)
// into a RoleMappingConfig. Returns an error if roleMapJSON is present but
// malformed, or maps to a role outside the fixed internal vocabulary.
func NewRoleMappingConfigFromStrings(roleClaimPath, displayNameClaimPath, roleMapJSON, userPatterns, curatorPatterns, adminPatterns string) (*RoleMappingConfig, error) {
	roleMap := map[string]InternalRole{}
	if strings.TrimSpace(roleMapJSON) != "" {
		var raw map[string]string
		if err := json.Unmarshal([]byte(roleMapJSON), &raw); err != nil {
			return nil, fmt.Errorf("invalid OIDC role map configuration: %w", err)
		}
		for external, internal := range raw {
			role := InternalRole(strings.TrimSpace(internal))
			if _, ok := roleRank[role]; !ok {
				return nil, fmt.Errorf("invalid internal role %q for external role %q: must be one of user, curator, admin", internal, external)
			}
			roleMap[external] = role
		}
	}

	return &RoleMappingConfig{
		RoleClaimPath:        roleClaimPath,
		DisplayNameClaimPath: displayNameClaimPath,
		RoleMap:              roleMap,
		Patterns: map[InternalRole][]string{
			InternalRoleUser:    splitPatterns(userPatterns),
			InternalRoleCurator: splitPatterns(curatorPatterns),
			InternalRoleAdmin:   splitPatterns(adminPatterns),
		},
	}, nil
}

func splitPatterns(csv string) []string {
	var patterns []string
	for p := range strings.SplitSeq(csv, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

// LookupDottedPath resolves a dotted path (e.g. "realm_access.roles") within
// a nested claims map. It supports exactly dotted-path traversal of nested
// map[string]any values - no wildcards, array indexing, or expression
// syntax. Returns ok=false if any segment is missing or the path traverses
// through a non-map value.
func LookupDottedPath(claims map[string]any, path string) (value any, ok bool) {
	if path == "" {
		return nil, false
	}
	segments := strings.Split(path, ".")
	var current any = claims
	for _, segment := range segments {
		m, isMap := current.(map[string]any)
		if !isMap {
			return nil, false
		}
		current, ok = m[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// extractStringList converts a claims value that may be []any of strings,
// []string, or a single string into a []string, ignoring non-string
// entries.
func extractStringList(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	default:
		return nil
	}
}

// resolveInternalRole extracts the external role list from claims via the
// configured dotted path, maps each entry to an internal role, and returns
// the most privileged matched role. ok is false when role mapping is not
// configured, the claim is absent, or none of the external roles map to a
// known internal role.
func resolveInternalRole(claims map[string]any, cfg *RoleMappingConfig) (InternalRole, bool) {
	if cfg == nil || cfg.RoleClaimPath == "" || len(cfg.RoleMap) == 0 {
		return "", false
	}

	value, found := LookupDottedPath(claims, cfg.RoleClaimPath)
	if !found {
		return "", false
	}

	var best InternalRole
	matched := false
	for _, external := range extractStringList(value) {
		internal, ok := cfg.RoleMap[external]
		if !ok {
			continue
		}
		if !matched || roleRank[internal] > roleRank[best] {
			best = internal
			matched = true
		}
	}
	return best, matched
}

// resolveDisplayName extracts a presentation-only display name from claims
// via the configured dotted path. Returns "" if not configured, absent, or
// not a string. The result is never used for authorization.
func resolveDisplayName(claims map[string]any, cfg *RoleMappingConfig) string {
	if cfg == nil || cfg.DisplayNameClaimPath == "" {
		return ""
	}
	value, found := LookupDottedPath(claims, cfg.DisplayNameClaimPath)
	if !found {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

// buildRoleBundle expands an internal role into its permission bundle,
// scoped to the role's configured resource patterns:
//
//	user    = read + publish + edit_own
//	curator = user + edit + delete + review
// admin   = curator + deploy + review + override
//
// A role with no configured patterns yields an empty bundle - it is never
// granted the bare "*" resource pattern, which would trip
// auth.IsRegistryAdmin's unbounded, action-agnostic bypass regardless of
// which action was requested. Admin is therefore bounded to whatever
// resource patterns are configured for it, not a global bypass.
func buildRoleBundle(role InternalRole, patterns []string) []auth.Permission {
	if len(patterns) == 0 {
		return nil
	}

	actions := []auth.PermissionAction{
		auth.PermissionActionRead,
		auth.PermissionActionPublish,
		auth.PermissionActionEditOwn,
	}
	switch role {
	case InternalRoleUser:
		// read + publish + edit_own
	case InternalRoleCurator:
		actions = append(actions, auth.PermissionActionEdit, auth.PermissionActionDelete, auth.PermissionActionReview)
	case InternalRoleAdmin:
		actions = append(actions, auth.PermissionActionEdit, auth.PermissionActionDelete, auth.PermissionActionDeploy, auth.PermissionActionReview, auth.PermissionActionOverride)
	default:
		return nil
	}

	permissions := make([]auth.Permission, 0, len(actions)*len(patterns))
	for _, action := range actions {
		for _, pattern := range patterns {
			permissions = append(permissions, auth.Permission{
				Action:          action,
				ResourcePattern: pattern,
			})
		}
	}
	return permissions
}

// ResolveRolePermissions is the top-level entry point for role-aware OIDC
// permissions: it extracts the caller's internal role and a presentation
// display name from raw OIDC claims, and returns the role's permission
// bundle. matched is false when role mapping is not configured, no external
// role was recognized, or the matched role has no resource patterns
// configured - in every such case the caller should fall back to its
// existing default (static) permission bundle rather than receiving no
// permissions at all. The last case (a role matched but left unconfigured)
// is logged as a warning: it is a misconfiguration, not an absence of role
// mapping, and would otherwise silently downgrade the caller to the default
// bundle with no operator-visible signal.
func ResolveRolePermissions(claims map[string]any, cfg *RoleMappingConfig) (permissions []auth.Permission, displayName string, matched bool) {
	displayName = resolveDisplayName(claims, cfg)

	role, ok := resolveInternalRole(claims, cfg)
	if !ok {
		return nil, displayName, false
	}

	bundle := buildRoleBundle(role, cfg.Patterns[role])
	if len(bundle) == 0 {
		slog.Warn("OIDC role matched but has no configured resource patterns; falling back to default permission bundle",
			"role", string(role))
		return nil, displayName, false
	}

	return bundle, displayName, true
}
