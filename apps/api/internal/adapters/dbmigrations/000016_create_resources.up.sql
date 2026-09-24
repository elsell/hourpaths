CREATE TABLE resource_models (
    id text PRIMARY KEY,
    domain text NOT NULL,
    owner_user_id text NOT NULL REFERENCES user_models(id),
    name text NOT NULL,
    created_at timestamptz NOT NULL
);
CREATE INDEX resource_models_owner_page_idx
    ON resource_models(domain, owner_user_id, created_at, id);
GRANT SELECT, INSERT, UPDATE, DELETE ON resource_models TO app;
