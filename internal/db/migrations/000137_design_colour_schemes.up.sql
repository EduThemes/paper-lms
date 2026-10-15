-- 000137_design_colour_schemes.up.sql
--
-- Page Designer: a teacher's saved palettes ("My colour schemes").
-- Owner-only rows; account_id is the owner's tenant. At most 50 per user
-- (enforced by the API).

CREATE TABLE IF NOT EXISTS design_colour_schemes (
    id bigserial,
    account_id bigint NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    palette jsonb NOT NULL,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_design_colour_schemes_user_id ON public.design_colour_schemes USING btree (user_id);
