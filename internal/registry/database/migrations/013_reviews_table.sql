-- AR-2: append-only typed reviews for specific artifact versions.
--
-- Reviews intentionally have no foreign key because artifact identity spans
-- four tables. Orphaned review rows after an artifact deletion are accepted.

CREATE TABLE reviews (
    id BIGSERIAL PRIMARY KEY,
    artifact_type VARCHAR(50) NOT NULL,
    artifact_name VARCHAR(255) NOT NULL,
    artifact_version VARCHAR(255) NOT NULL,
    review_type VARCHAR(100) NOT NULL,
    outcome VARCHAR(50) NOT NULL,
    reviewer_subject TEXT NOT NULL,
    reviewer_auth_method TEXT NOT NULL,
    reviewer_display_name TEXT NOT NULL,
    notes TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT reviews_artifact_type_valid
        CHECK (artifact_type IN ('server', 'agent', 'skill', 'prompt'))
);

-- Supports a curator's review history across artifact types and versions.
CREATE INDEX idx_reviews_reviewer
    ON reviews (reviewer_subject, reviewer_auth_method, created_at DESC);

-- Supports artifact review state and latest-row resolution. The id tie-breaker
-- makes rows with equal timestamps deterministic.
CREATE INDEX idx_reviews_current
    ON reviews (
        artifact_type,
        artifact_name,
        artifact_version,
        review_type,
        reviewer_subject,
        created_at DESC,
        id DESC
    );
