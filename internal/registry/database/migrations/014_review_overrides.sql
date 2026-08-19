-- AR-2B: append-only administrative overrides of failed reviews.

ALTER TABLE reviews
    ADD COLUMN overrides_review_id BIGINT;

CREATE INDEX idx_reviews_overrides_review
    ON reviews (overrides_review_id)
    WHERE overrides_review_id IS NOT NULL;
