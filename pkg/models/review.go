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

const (
	ReviewStatusPending = "pending"
	ReviewStatusPass    = "pass"
	ReviewStatusFail    = "fail"
)

// ReviewTypeStatus summarizes the current reviews for one configured type.
type ReviewTypeStatus struct {
	ReviewType     string   `json:"review_type"`
	Status         string   `json:"status"`
	Outcome        string   `json:"outcome"`
	CurrentReviews []Review `json:"current_reviews"`
}

// ReviewState contains the derived review state for one artifact version.
type ReviewState struct {
	Certified      bool               `json:"certified"`
	Rejected       bool               `json:"rejected"`
	Pending        bool               `json:"pending"`
	CurrentReviews []Review           `json:"current_reviews"`
	PerType        []ReviewTypeStatus `json:"per_type"`
}
