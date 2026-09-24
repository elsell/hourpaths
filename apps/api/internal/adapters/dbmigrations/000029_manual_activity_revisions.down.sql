BEGIN;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM public.recorded_activity_revision_models
  ) OR EXISTS (
    SELECT 1
    FROM public.recorded_activity_models
    WHERE note IS NOT NULL
  ) OR EXISTS (
    SELECT 1
    FROM public.activity_mutation_models
    WHERE operation IN ('activity.manual.create', 'activity.update')
  ) THEN
    RAISE EXCEPTION 'cannot remove manual activity persistence while note, revision, create, or update state exists';
  END IF;
END
$$;

REVOKE ALL ON public.recorded_activity_revision_models FROM app;
DROP TABLE public.recorded_activity_revision_models;

ALTER TABLE public.activity_mutation_models
  DROP CONSTRAINT activity_mutation_models_operation_check,
  DROP CONSTRAINT activity_mutation_models_operation_state_check,
  DROP CONSTRAINT activity_mutation_models_saved_result_check,
  DROP CONSTRAINT activity_mutation_models_result_note_check;
ALTER TABLE public.activity_mutation_models DROP COLUMN result_version;
ALTER TABLE public.activity_mutation_models DROP COLUMN result_note;
ALTER TABLE public.activity_mutation_models ALTER COLUMN timer_id SET NOT NULL;
ALTER TABLE public.activity_mutation_models
  ADD CONSTRAINT activity_mutation_models_operation_check CHECK (
    operation IN ('activity.timer.start', 'activity.timer.stop')
  ),
  ADD CONSTRAINT activity_mutation_models_check CHECK (
    (operation = 'activity.timer.start' AND result_started_at IS NOT NULL AND result_time_zone <> '' AND NOT result_activity_saved)
    OR operation = 'activity.timer.stop'
  ),
  ADD CONSTRAINT activity_mutation_models_check1 CHECK (
    NOT result_activity_saved
    OR (
      result_activity_id IS NOT NULL
      AND result_started_at IS NOT NULL
      AND result_ended_at IS NOT NULL
      AND result_time_zone <> ''
      AND result_created_at IS NOT NULL
      AND result_updated_at IS NOT NULL
    )
  );

ALTER TABLE public.recorded_activity_models DROP CONSTRAINT recorded_activity_models_note_check;
DROP INDEX public.recorded_activity_models_path_created_idx;
ALTER TABLE public.recorded_activity_models DROP COLUMN note;
REVOKE ALL ON public.recorded_activity_models FROM app;
GRANT SELECT, INSERT ON public.recorded_activity_models TO app;
REVOKE ALL ON public.activity_mutation_models FROM app;
GRANT SELECT, INSERT, UPDATE ON public.activity_mutation_models TO app;

COMMIT;
