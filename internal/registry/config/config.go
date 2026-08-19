package config

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"slices"
	"strings"

	env "github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config holds the application configuration
// See .env.example for more documentation
type Config struct {
	ServerAddress                  string `env:"SERVER_ADDRESS" envDefault:":8080"`
	MCPPort                        uint16 `env:"MCP_PORT" envDefault:"0"`
	DatabaseURL                    string `env:"DATABASE_URL" envDefault:"postgres://agentregistry:agentregistry@localhost:5432/agentregistry?sslmode=disable"`
	DatabaseVectorEnabled          bool   `env:"DATABASE_VECTOR_ENABLED" envDefault:"false"`
	SeedFrom                       string `env:"SEED_FROM" envDefault:""`
	EnrichServerData               bool   `env:"ENRICH_SERVER_DATA" envDefault:"false"`
	DisableBuiltinSeed             bool   `env:"DISABLE_BUILTIN_SEED" envDefault:"true"`
	Version                        string `env:"VERSION" envDefault:"dev"`
	GithubClientID                 string `env:"GITHUB_CLIENT_ID" envDefault:""`
	GithubClientSecret             string `env:"GITHUB_CLIENT_SECRET" envDefault:""`
	JWTPrivateKey                  string `env:"JWT_PRIVATE_KEY" envDefault:""`
	EnableAnonymousAuth            bool   `env:"ENABLE_ANONYMOUS_AUTH" envDefault:"false"`
	EnableRegistryValidation       bool   `env:"ENABLE_REGISTRY_VALIDATION" envDefault:"true"`
	ValidateRepositoryReachability bool   `env:"VALIDATE_REPOSITORY_REACHABILITY" envDefault:"true"`
	LogLevel                       string `env:"LOG_LEVEL" envDefault:"info"`

	// Review configuration. Values are ASCII identifiers beginning with a
	// letter and continuing with letters, digits, hyphens, or underscores.
	ReviewTypes           []string `env:"REVIEW_TYPES" envDefault:"security,scientific"`
	ReviewOutcomes        []string `env:"REVIEW_OUTCOMES" envDefault:"pass,fail"`
	ReviewFailureOutcome  string   `env:"REVIEW_FAILURE_OUTCOME" envDefault:"fail"`
	ReviewOverrideOutcome string   `env:"REVIEW_OVERRIDE_OUTCOME" envDefault:"override"`

	// Frontend OIDC Configuration (served at runtime via GET /v0/config/frontend)
	KeycloakURL        string `env:"KEYCLOAK_URL" envDefault:""`
	KeycloakRealm      string `env:"KEYCLOAK_REALM" envDefault:""`
	KeycloakClientID   string `env:"KEYCLOAK_CLIENT_ID" envDefault:""`
	FrontendAPIURL     string `env:"FRONTEND_API_URL" envDefault:""`
	FrontendGatewayURL string `env:"FRONTEND_GATEWAY_URL" envDefault:""`

	// OIDC Configuration
	OIDCEnabled      bool   `env:"OIDC_ENABLED" envDefault:"false"`
	OIDCIssuer       string `env:"OIDC_ISSUER" envDefault:""`
	OIDCClientID     string `env:"OIDC_CLIENT_ID" envDefault:""`
	OIDCExtraClaims  string `env:"OIDC_EXTRA_CLAIMS" envDefault:""`
	OIDCEditPerms    string `env:"OIDC_EDIT_PERMISSIONS" envDefault:""`
	OIDCPublishPerms string `env:"OIDC_PUBLISH_PERMISSIONS" envDefault:""`
	OIDCReadPerms    string `env:"OIDC_READ_PERMISSIONS" envDefault:""`
	OIDCPushPerms    string `env:"OIDC_PUSH_PERMISSIONS" envDefault:""`
	OIDCDeletePerms  string `env:"OIDC_DELETE_PERMISSIONS" envDefault:""`
	OIDCDeployPerms  string `env:"OIDC_DEPLOY_PERMISSIONS" envDefault:""`

	// OIDC Role Mapping (Optional)
	// When OIDCRoleClaimPath is empty, role-based permissions are disabled and
	// callers fall through to the static OIDC_*_PERMISSIONS bundle above.
	//
	// OIDCRoleClaimPath is a dotted path (e.g. "realm_access.roles") locating
	// the external role list within the OIDC claims.
	OIDCRoleClaimPath string `env:"OIDC_ROLE_CLAIM_PATH" envDefault:""`
	// OIDCDisplayNameClaimPath is a dotted path (e.g. "name") locating a
	// human-readable display name within the OIDC claims. Presentation only;
	// never used for authorization.
	OIDCDisplayNameClaimPath string `env:"OIDC_DISPLAY_NAME_CLAIM_PATH" envDefault:""`
	// OIDCRoleMap is a JSON object mapping external role strings to the
	// internal vocabulary (user, curator, admin), e.g.
	// {"registry-admin": "admin", "registry-curator": "curator"}.
	OIDCRoleMap string `env:"OIDC_ROLE_MAP" envDefault:""`
	// OIDC{User,Curator,Admin}Patterns are comma-separated resource-name
	// patterns (same glob rules as auth.Permission.ResourcePattern) that the
	// corresponding internal role's permission bundle is scoped to. A single
	// flat list per role, applied across all artifact types, so operators
	// must include both dotted (server/agent) and flat (skill/prompt) name
	// prefixes where relevant, e.g. "io.example.*,example-*".
	OIDCUserPatterns    string `env:"OIDC_USER_PATTERNS" envDefault:""`
	OIDCCuratorPatterns string `env:"OIDC_CURATOR_PATTERNS" envDefault:""`
	OIDCAdminPatterns   string `env:"OIDC_ADMIN_PATTERNS" envDefault:""`

	// Platform mode: "docker" or "kubernetes". Controls which deployment
	// provider IDs are available in the UI. Defaults to "kubernetes" so
	// Helm/K8s deployments work without extra config; docker-compose.yml
	// explicitly sets this to "docker".
	PlatformMode string `env:"PLATFORM_MODE" envDefault:"kubernetes"`

	// Agent Gateway Configuration
	AgentGatewayPort uint16 `env:"AGENT_GATEWAY_PORT" envDefault:"8081"`

	// Runtime Configuration
	RuntimeDir string `env:"RUNTIME_DIR" envDefault:"/tmp/arctl-runtime"`
	Verbose    bool   `env:"VERBOSE" envDefault:"false"`

	// MCP Scoring — external service for analyzing MCP server quality.
	// When empty, scoring is disabled.
	MCPScoringURL     string `env:"MCP_SCORING_URL" envDefault:""`
	MCPScoringTimeout int    `env:"MCP_SCORING_TIMEOUT" envDefault:"120"`

	// Embeddings / Semantic Search
	Embeddings EmbeddingsConfig
}

// EmbeddingsConfig captures configuration needed to generate embeddings
type EmbeddingsConfig struct {
	Enabled       bool   `env:"EMBEDDINGS_ENABLED" envDefault:"false"`
	Provider      string `env:"EMBEDDINGS_PROVIDER" envDefault:"openai"`
	Model         string `env:"EMBEDDINGS_MODEL" envDefault:"text-embedding-3-small"`
	Dimensions    int    `env:"EMBEDDINGS_DIMENSIONS" envDefault:"1536"`
	OpenAIAPIKey  string `env:"OPENAI_API_KEY" envDefault:""`
	OpenAIBaseURL string `env:"OPENAI_BASE_URL" envDefault:"https://api.openai.com/v1"`
	OpenAIOrg     string `env:"OPENAI_ORG" envDefault:""`
	OnPublish     bool   `env:"EMBEDDINGS_ON_PUBLISH" envDefault:"false"`
}

// ReviewSettings exposes the validated review vocabulary to downstream
// services without exposing the raw configuration slices.
type ReviewSettings struct {
	reviewTypes     []string
	outcomes        []string
	failureOutcome  string
	overrideOutcome string
}

// Types returns the configured review types in their configured order.
func (s ReviewSettings) Types() []string {
	return slices.Clone(s.reviewTypes)
}

// Outcomes returns the configured outcomes in their configured order.
func (s ReviewSettings) Outcomes() []string {
	return slices.Clone(s.outcomes)
}

// HasType reports whether reviewType is configured.
func (s ReviewSettings) HasType(reviewType string) bool {
	return slices.Contains(s.reviewTypes, reviewType)
}

// HasOutcome reports whether outcome is configured.
func (s ReviewSettings) HasOutcome(outcome string) bool {
	return slices.Contains(s.outcomes, outcome)
}

// FailureOutcome returns the configured outcome that rejects an artifact.
func (s ReviewSettings) FailureOutcome() string {
	return s.failureOutcome
}

// OverrideOutcome returns the configured outcome used for administrative
// overrides.
func (s ReviewSettings) OverrideOutcome() string {
	return s.overrideOutcome
}

// ReviewConfig returns the configured review vocabulary in stable order.
func (cfg *Config) ReviewConfig() ReviewSettings {
	return ReviewSettings{
		reviewTypes:     slices.Clone(cfg.ReviewTypes),
		outcomes:        slices.Clone(cfg.ReviewOutcomes),
		failureOutcome:  strings.TrimSpace(cfg.ReviewFailureOutcome),
		overrideOutcome: strings.TrimSpace(cfg.ReviewOverrideOutcome),
	}
}

func normalizeReviewValues(values []string) []string {
	normalized := make([]string, len(values))
	for i, value := range values {
		normalized[i] = strings.TrimSpace(value)
	}
	return normalized
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		slog.Info("no .env file found or error loading .env file", "error", err)
	}
	var cfg Config
	err = env.ParseWithOptions(&cfg, env.Options{
		Prefix: "AGENT_REGISTRY_",
	})
	if err != nil {
		slog.Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	// Append a random suffix to RuntimeDir when the user has not set an
	// explicit override via the AGENT_REGISTRY_RUNTIME_DIR env var. This
	// prevents concurrent runs from sharing the same directory.
	if os.Getenv("AGENT_REGISTRY_RUNTIME_DIR") == "" {
		suffix, err := randomHex(8)
		if err != nil {
			slog.Error("failed to generate random runtime dir suffix", "error", err)
			os.Exit(1)
		}
		cfg.RuntimeDir = cfg.RuntimeDir + "-" + suffix
	}

	return &cfg
}

// randomHex returns a hex-encoded string of n random bytes.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
