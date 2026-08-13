-- 011_add_ownership_columns.sql
-- AR-1: record who registered each artifact.
--
-- created_by_subject is the stable identity from the auth token (OIDC `sub`).
-- created_by_display_name is a snapshot of the human-readable name at creation
-- time, for display only -- it is not an identity and must never be used for
-- authorization. Subject authorizes; name displays.
--
-- Both are nullable. Rows that predate this migration remain NULL and are
-- treated as unowned. No backfill.

ALTER TABLE servers
    ADD COLUMN created_by_subject TEXT,
    ADD COLUMN created_by_display_name TEXT;

ALTER TABLE skills
    ADD COLUMN created_by_subject TEXT,
    ADD COLUMN created_by_display_name TEXT;

ALTER TABLE agents
    ADD COLUMN created_by_subject TEXT,
    ADD COLUMN created_by_display_name TEXT;

ALTER TABLE prompts
    ADD COLUMN created_by_subject TEXT,
    ADD COLUMN created_by_display_name TEXT;

CREATE INDEX idx_servers_created_by_subject ON servers (created_by_subject);
CREATE INDEX idx_skills_created_by_subject ON skills (created_by_subject);
CREATE INDEX idx_agents_created_by_subject ON agents (created_by_subject);
CREATE INDEX idx_prompts_created_by_subject ON prompts (created_by_subject);