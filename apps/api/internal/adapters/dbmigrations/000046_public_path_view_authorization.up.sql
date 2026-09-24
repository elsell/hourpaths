BEGIN;

-- Serialize setup with Path and authorization writers. Once installed, the
-- trigger bridges public Paths created by the still-running prior API across
-- the deployment cutover without relying on a timing window.
LOCK TABLE public.path_models IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE public.authorization_outbox_models IN SHARE ROW EXCLUSIVE MODE;

CREATE FUNCTION public.publish_public_path_viewer_authorization()
RETURNS trigger
LANGUAGE plpgsql
SECURITY INVOKER
SET search_path = pg_catalog, public
AS $$
BEGIN
  IF NEW.resource_type = 'path'
     AND NEW.relation = 'creator'
     AND NEW.operation = 'touch'
     AND EXISTS (
       SELECT 1
       FROM public.path_models AS path
       WHERE path.id = NEW.resource_id
         AND path.owner_user_id = NEW.subject_id
         AND path.visibility = 'public'
     ) THEN
    INSERT INTO public.authorization_outbox_models (
      id, resource_type, resource_id, relation, subject_type, subject_id,
      owner_user_id, actor_user_id, operation, created_at
    ) VALUES (
      NEW.id || '-public-viewer', 'path', NEW.resource_id, 'public_viewer', 'user', '*',
      NEW.owner_user_id, NEW.actor_user_id, 'touch', NEW.created_at
    )
    ON CONFLICT (id) DO NOTHING;
    IF NOT EXISTS (
      SELECT 1
      FROM public.authorization_outbox_models AS audience
      WHERE audience.id = NEW.id || '-public-viewer'
        AND audience.resource_type = 'path'
        AND audience.resource_id = NEW.resource_id
        AND audience.relation = 'public_viewer'
        AND audience.subject_type = 'user'
        AND audience.subject_id = '*'
        AND audience.owner_user_id = NEW.owner_user_id
        AND audience.actor_user_id = NEW.actor_user_id
        AND audience.operation = 'touch'
    ) THEN
      RAISE EXCEPTION 'public Path authorization cutover identifier conflicts with another relationship'
        USING ERRCODE = '23505';
    END IF;
  END IF;
  RETURN NEW;
END
$$;

REVOKE ALL ON FUNCTION public.publish_public_path_viewer_authorization() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.publish_public_path_viewer_authorization() TO app;

CREATE TRIGGER authorization_outbox_publish_public_path_viewer
AFTER INSERT ON public.authorization_outbox_models
FOR EACH ROW EXECUTE FUNCTION public.publish_public_path_viewer_authorization();

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM public.path_models AS path
    WHERE path.visibility = 'public'
      AND NOT EXISTS (
        SELECT 1
        FROM public.authorization_outbox_models AS creator
        WHERE creator.resource_type = 'path'
          AND creator.resource_id = path.id
          AND creator.relation = 'creator'
          AND creator.subject_type = 'user'
          AND creator.subject_id = path.owner_user_id
          AND creator.operation = 'touch'
      )
  ) THEN
    RAISE EXCEPTION 'migration 46 requires creator authorization evidence for every public Path'
      USING ERRCODE = '55000';
  END IF;
END;
$$;

INSERT INTO public.authorization_outbox_models (
  id, resource_type, resource_id, relation, subject_type, subject_id,
  owner_user_id, actor_user_id, operation, created_at
)
SELECT creator.id || '-public-viewer', 'path', path.id, 'public_viewer', 'user', '*',
       path.owner_user_id, path.owner_user_id, 'touch', clock_timestamp()
FROM public.path_models AS path
JOIN LATERAL (
  SELECT evidence.id
  FROM public.authorization_outbox_models AS evidence
  WHERE evidence.resource_type = 'path'
    AND evidence.resource_id = path.id
    AND evidence.relation = 'creator'
    AND evidence.subject_type = 'user'
    AND evidence.subject_id = path.owner_user_id
    AND evidence.operation = 'touch'
  ORDER BY evidence.created_at DESC, evidence.id DESC
  LIMIT 1
) AS creator ON TRUE
WHERE path.visibility = 'public'
ON CONFLICT (id) DO NOTHING;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM public.path_models AS path
    JOIN LATERAL (
      SELECT evidence.id
      FROM public.authorization_outbox_models AS evidence
      WHERE evidence.resource_type = 'path'
        AND evidence.resource_id = path.id
        AND evidence.relation = 'creator'
        AND evidence.subject_type = 'user'
        AND evidence.subject_id = path.owner_user_id
        AND evidence.operation = 'touch'
      ORDER BY evidence.created_at DESC, evidence.id DESC
      LIMIT 1
    ) AS creator ON TRUE
    WHERE path.visibility = 'public'
      AND NOT EXISTS (
        SELECT 1
        FROM public.authorization_outbox_models AS audience
        WHERE audience.id = creator.id || '-public-viewer'
          AND audience.resource_type = 'path'
          AND audience.resource_id = path.id
          AND audience.relation = 'public_viewer'
          AND audience.subject_type = 'user'
          AND audience.subject_id = '*'
          AND audience.owner_user_id = path.owner_user_id
          AND audience.operation = 'touch'
      )
  ) THEN
    RAISE EXCEPTION 'migration 46 public Path authorization backfill did not converge'
      USING ERRCODE = '23505';
  END IF;
END;
$$;

COMMIT;
