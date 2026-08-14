-- 012_add_ownership_auth_method.sql
-- AR-1: disambiguate ownership subjects across auth methods.
--
-- Subject strings are only unique within an auth method: generic OIDC yields an
-- IdP UUID, GitHub access-token auth yields a bare login, DNS/HTTP auth yields a
-- domain. Identity is therefore the pair (auth method, subject), not the subject
-- alone.
--
-- Recorded now so the pair is available if a deployment ever enables more than
-- one auth method. Authorization compares subject only; this column is not read
-- by AR-1.
--
-- Nullable, no backfill. Rows with no ownership have no auth method.

ALTER TABLE servers ADD COLUMN created_by_auth_method TEXT;
ALTER TABLE skills  ADD COLUMN created_by_auth_method TEXT;
ALTER TABLE agents  ADD COLUMN created_by_auth_method TEXT;
ALTER TABLE prompts ADD COLUMN created_by_auth_method TEXT;