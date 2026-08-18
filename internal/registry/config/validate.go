package config

import (
	"fmt"
	"regexp"
	"strings"
)

// Excludes dots and slashes deliberately: these values appear in JSON keys and
// URL segments, so namespaced names like "nasa.export-control" are not allowed.
var reviewValuePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// Validate performs runtime validations on the loaded configuration.
// It is intentionally strict for embeddings to avoid runtime pgvector errors.
func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	cfg.ReviewTypes = normalizeReviewValues(cfg.ReviewTypes)
	cfg.ReviewOutcomes = normalizeReviewValues(cfg.ReviewOutcomes)
	if err := validateReviewValues("review types", cfg.ReviewTypes); err != nil {
		return err
	}
	if err := validateReviewValues("review outcomes", cfg.ReviewOutcomes); err != nil {
		return err
	}
	if cfg.Embeddings.Enabled {
		if cfg.Embeddings.Dimensions <= 0 {
			return fmt.Errorf("embeddings dimensions must be positive (got %d)", cfg.Embeddings.Dimensions)
		}
		// Database schema currently provisions vector(1536). Reject mismatches early.
		if cfg.Embeddings.Dimensions != 1536 {
			return fmt.Errorf("embeddings dimensions must equal 1536 to match database schema (got %d)", cfg.Embeddings.Dimensions)
		}
		if cfg.Embeddings.Model == "" {
			return fmt.Errorf("embeddings model must be specified when embeddings are enabled")
		}
		if cfg.Embeddings.Provider == "" {
			return fmt.Errorf("embeddings provider must be specified when embeddings are enabled")
		}
	}
	return nil
}

func validateReviewValues(label string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s must not be empty", label)
	}

	seen := make(map[string]struct{}, len(values))
	for _, rawValue := range values {
		value := strings.TrimSpace(rawValue)
		if value == "" {
			return fmt.Errorf("%s contains an empty entry", label)
		}
		if !reviewValuePattern.MatchString(value) {
			return fmt.Errorf("%s entry %q must match %q", label, rawValue, reviewValuePattern.String())
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s contains duplicate entry %q", label, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}
