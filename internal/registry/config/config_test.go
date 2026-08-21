package config

import (
	"os"
	"strings"
	"testing"
)

func TestNewConfig_RuntimeDirHasRandomSuffix(t *testing.T) {
	// Ensure the env var is unset so the default path is used.
	os.Unsetenv("AGENT_REGISTRY_RUNTIME_DIR")

	cfg := NewConfig()

	base := "/tmp/arctl-runtime-"
	if !strings.HasPrefix(cfg.RuntimeDir, base) {
		t.Fatalf("RuntimeDir should start with %q, got %q", base, cfg.RuntimeDir)
	}

	suffix := strings.TrimPrefix(cfg.RuntimeDir, base)
	if len(suffix) != 16 { // 8 bytes = 16 hex chars
		t.Fatalf("RuntimeDir suffix should be 16 hex chars, got %q (len %d)", suffix, len(suffix))
	}
}

func TestNewConfig_RuntimeDirUniqueBetweenCalls(t *testing.T) {
	os.Unsetenv("AGENT_REGISTRY_RUNTIME_DIR")

	cfg1 := NewConfig()
	cfg2 := NewConfig()

	if cfg1.RuntimeDir == cfg2.RuntimeDir {
		t.Fatalf("two NewConfig() calls should produce different RuntimeDir values, both got %q", cfg1.RuntimeDir)
	}
}

func TestNewConfig_RuntimeDirRespectsEnvOverride(t *testing.T) {
	custom := "/custom/runtime/path"
	t.Setenv("AGENT_REGISTRY_RUNTIME_DIR", custom)

	cfg := NewConfig()

	if cfg.RuntimeDir != custom {
		t.Fatalf("RuntimeDir should be %q when env var is set, got %q", custom, cfg.RuntimeDir)
	}
}

func TestNewConfig_OIDCRolePatternsDefaultToEmpty(t *testing.T) {
	for _, key := range []string{
		"AGENT_REGISTRY_OIDC_ROLE_CLAIM_PATH",
		"AGENT_REGISTRY_OIDC_DISPLAY_NAME_CLAIM_PATH",
		"AGENT_REGISTRY_OIDC_ROLE_MAP",
		"AGENT_REGISTRY_OIDC_USER_PATTERNS",
		"AGENT_REGISTRY_OIDC_CURATOR_PATTERNS",
		"AGENT_REGISTRY_OIDC_ADMIN_PATTERNS",
	} {
		os.Unsetenv(key)
	}

	cfg := NewConfig()

	if cfg.OIDCRoleClaimPath != "" {
		t.Fatalf("OIDCRoleClaimPath should default to empty (role mapping disabled), got %q", cfg.OIDCRoleClaimPath)
	}
	if cfg.OIDCDisplayNameClaimPath != "" {
		t.Fatalf("OIDCDisplayNameClaimPath should default to empty, got %q", cfg.OIDCDisplayNameClaimPath)
	}
	if cfg.OIDCRoleMap != "" {
		t.Fatalf("OIDCRoleMap should default to empty, got %q", cfg.OIDCRoleMap)
	}
	if cfg.OIDCUserPatterns != "" || cfg.OIDCCuratorPatterns != "" || cfg.OIDCAdminPatterns != "" {
		t.Fatalf("OIDC role pattern lists should default to empty, got user=%q curator=%q admin=%q",
			cfg.OIDCUserPatterns, cfg.OIDCCuratorPatterns, cfg.OIDCAdminPatterns)
	}
}
