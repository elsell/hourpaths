CREATE TABLE path_models (
    id text PRIMARY KEY,
    owner_user_id text NOT NULL REFERENCES user_models(id),
    "name" text NOT NULL CHECK (
        name = btrim(name)
        AND char_length(name) BETWEEN 1 AND 100
        AND name !~ '[[:cntrl:]]'
    ),
    "visibility" text NOT NULL CHECK (visibility IN ('private', 'followers', 'public')),

    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX path_models_owner_page_idx ON path_models(owner_user_id, created_at, id);

CREATE TABLE path_membership_models (
    path_id text NOT NULL REFERENCES path_models(id) ON DELETE CASCADE,
    user_id text NOT NULL REFERENCES user_models(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('administrator', 'participant', 'supporter')),
    PRIMARY KEY (path_id, user_id)
);
CREATE INDEX path_membership_models_user_path_idx ON path_membership_models(user_id, path_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON path_models TO app;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON path_membership_models FROM app;
GRANT SELECT ON path_membership_models TO app;
