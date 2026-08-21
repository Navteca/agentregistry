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

func TestNewConfig_CommunityLinksDefaultToShown(t *testing.T) {
	os.Unsetenv("AGENT_REGISTRY_SHOW_GITHUB_LINK")
	os.Unsetenv("AGENT_REGISTRY_SHOW_DISCORD_LINK")

	cfg := NewConfig()

	if !cfg.ShowGithubLink {
		t.Fatal("ShowGithubLink should default to true")
	}
	if !cfg.ShowDiscordLink {
		t.Fatal("ShowDiscordLink should default to true")
	}
}

func TestNewConfig_ExplicitReviewDefaults(t *testing.T) {
	t.Setenv("AGENT_REGISTRY_REVIEW_TYPES", "security,scientific")
	t.Setenv("AGENT_REGISTRY_REVIEW_OUTCOMES", "pass,fail")
	t.Setenv("AGENT_REGISTRY_REVIEW_OVERRIDE_OUTCOME", "override")

	cfg := NewConfig()
	if got := cfg.ReviewConfig().Types(); !equalStrings(got, []string{"security", "scientific"}) {
		t.Fatalf("review types default = %v, want [security scientific]", got)
	}
	if got := cfg.ReviewConfig().Outcomes(); !equalStrings(got, []string{"pass", "fail"}) {
		t.Fatalf("review outcomes default = %v, want [pass fail]", got)
	}
	if got := cfg.ReviewConfig().FailureOutcome(); got != "fail" {
		t.Fatalf("review failure outcome default = %q, want fail", got)
	}
	if got := cfg.ReviewConfig().OverrideOutcome(); got != "override" {
		t.Fatalf("review override outcome default = %q, want override", got)
	}
}

func TestReviewConfig_ConfiguredValuesPreserveOrder(t *testing.T) {
	t.Setenv("AGENT_REGISTRY_REVIEW_TYPES", "security,scientific,export-control")
	t.Setenv("AGENT_REGISTRY_REVIEW_OUTCOMES", "pass,fail,conditional,approved")
	t.Setenv("AGENT_REGISTRY_REVIEW_FAILURE_OUTCOME", "conditional")
	t.Setenv("AGENT_REGISTRY_REVIEW_OVERRIDE_OUTCOME", "override")

	cfg := NewConfig()
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	settings := cfg.ReviewConfig()
	if got := settings.Types(); !equalStrings(got, []string{"security", "scientific", "export-control"}) {
		t.Fatalf("review types = %v, want configured order", got)
	}
	if got := settings.Outcomes(); !equalStrings(got, []string{"pass", "fail", "conditional", "approved"}) {
		t.Fatalf("review outcomes = %v, want configured order", got)
	}
	if !settings.HasType("scientific") || settings.HasType("missing") {
		t.Fatal("HasType() did not match configured values")
	}
	if !settings.HasOutcome("pass") || settings.HasOutcome("pending") {
		t.Fatal("HasOutcome() did not match configured values")
	}
	if settings.FailureOutcome() != "conditional" {
		t.Fatalf("failure outcome = %q, want conditional", settings.FailureOutcome())
	}
	if settings.OverrideOutcome() != "override" {
		t.Fatalf("override outcome = %q, want override", settings.OverrideOutcome())
	}
}

func TestValidate_NormalizesReviewValuesInPlace(t *testing.T) {
	cfg := &Config{
		ReviewTypes:           []string{" security ", "scientific"},
		ReviewOutcomes:        []string{" pass ", "fail"},
		ReviewFailureOutcome:  " fail ",
		ReviewOverrideOutcome: " override ",
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if !equalStrings(cfg.ReviewTypes, []string{"security", "scientific"}) {
		t.Fatalf("normalized review types = %v", cfg.ReviewTypes)
	}
	if !equalStrings(cfg.ReviewOutcomes, []string{"pass", "fail"}) {
		t.Fatalf("normalized review outcomes = %v", cfg.ReviewOutcomes)
	}
	if cfg.ReviewFailureOutcome != "fail" {
		t.Fatalf("normalized review failure outcome = %q", cfg.ReviewFailureOutcome)
	}
	if cfg.ReviewOverrideOutcome != "override" {
		t.Fatalf("normalized review override outcome = %q", cfg.ReviewOverrideOutcome)
	}
	if got := cfg.ReviewConfig().Types(); !equalStrings(got, cfg.ReviewTypes) {
		t.Fatalf("accessor review types = %v, fields = %v", got, cfg.ReviewTypes)
	}
	if got := cfg.ReviewConfig().Outcomes(); !equalStrings(got, cfg.ReviewOutcomes) {
		t.Fatalf("accessor review outcomes = %v, fields = %v", got, cfg.ReviewOutcomes)
	}
}

func TestValidate_ReviewValues(t *testing.T) {
	tests := []struct {
		name     string
		types    []string
		outcomes []string
		failure  string
		override string
	}{
		{name: "empty review types", types: []string{}, outcomes: []string{"pass", "override"}, failure: "pass", override: "override"},
		{name: "empty outcomes", types: []string{"security"}, outcomes: []string{}, failure: "pass", override: "override"},
		{name: "duplicate review type", types: []string{"security", "security"}, outcomes: []string{"pass", "override"}, failure: "pass", override: "override"},
		{name: "duplicate outcome", types: []string{"security"}, outcomes: []string{"pass", "pass", "override"}, failure: "pass", override: "override"},
		{name: "whitespace-only review type", types: []string{"   "}, outcomes: []string{"pass", "override"}, failure: "pass", override: "override"},
		{name: "malformed outcome", types: []string{"security"}, outcomes: []string{"not valid", "override"}, failure: "not valid", override: "override"},
		{name: "unconfigured failure outcome", types: []string{"security"}, outcomes: []string{"pass", "override"}, failure: "fail", override: "override"},
		{name: "empty override outcome", types: []string{"security"}, outcomes: []string{"pass"}, failure: "pass", override: ""},
		{name: "override equals failure", types: []string{"security"}, outcomes: []string{"pass"}, failure: "pass", override: "pass"},
		{name: "override equals ordinary outcome", types: []string{"security"}, outcomes: []string{"pass", "override"}, failure: "fail", override: "override"},
		{name: "malformed override outcome", types: []string{"security"}, outcomes: []string{"pass"}, failure: "pass", override: "not valid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ReviewTypes: tt.types, ReviewOutcomes: tt.outcomes, ReviewFailureOutcome: tt.failure, ReviewOverrideOutcome: tt.override}
			if err := Validate(cfg); err == nil {
				t.Fatal("Validate() error = nil, want validation failure")
			}
		})
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
