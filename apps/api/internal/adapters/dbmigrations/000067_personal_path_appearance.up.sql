BEGIN;
CREATE TABLE public.path_appearance_models (
 user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
 color text NOT NULL CHECK (color IN ('coral','lavender','gold','mint','blue','pink')),
 emoji text NOT NULL CHECK (char_length(emoji) BETWEEN 1 AND 32),
 revision bigint NOT NULL CHECK (revision > 0),
 updated_at timestamptz NOT NULL,
 PRIMARY KEY (user_id, path_id)
);
CREATE TABLE public.path_appearance_mutation_models (
 user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
 idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 16 AND 128),
 request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
 color text NOT NULL,
 emoji text NOT NULL,
 revision bigint NOT NULL CHECK (revision > 0),
 updated_at timestamptz NOT NULL,
 PRIMARY KEY (user_id, path_id, idempotency_key)
);
CREATE FUNCTION public.delete_path_appearance_after_membership() RETURNS trigger LANGUAGE plpgsql SET search_path = pg_catalog, public AS $$
BEGIN
 DELETE FROM public.path_appearance_models WHERE path_id=OLD.path_id AND user_id=OLD.user_id;
 DELETE FROM public.path_appearance_mutation_models WHERE path_id=OLD.path_id AND user_id=OLD.user_id;
 RETURN OLD;
END;
$$;
REVOKE ALL ON FUNCTION public.delete_path_appearance_after_membership() FROM PUBLIC;
CREATE TRIGGER delete_path_appearance_after_membership AFTER DELETE ON public.path_membership_models FOR EACH ROW EXECUTE FUNCTION public.delete_path_appearance_after_membership();
REVOKE ALL ON public.path_appearance_models, public.path_appearance_mutation_models FROM app;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.path_appearance_models TO app;
GRANT SELECT, INSERT, DELETE ON public.path_appearance_mutation_models TO app;
COMMIT;
