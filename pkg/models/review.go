package models

import "time"

// Review records one append-only review of a specific artifact version.
type Review struct {
	ID                  int64     `json:"id"`
	ArtifactType        string    `json:"artifact_type"`
	ArtifactName        string    `json:"artifact_name"`
	ArtifactVersion     string    `json:"artifact_version"`
	ReviewType          string    `json:"review_type"`
	Outcome             string    `json:"outcome"`
	ReviewerSubject     string    `json:"reviewer_subject"`
	ReviewerAuthMethod  string    `json:"reviewer_auth_method"`
	ReviewerDisplayName string    `json:"reviewer_display_name"`
	Notes               string    `json:"notes"`
	CreatedAt           time.Time `json:"created_at"`
}
