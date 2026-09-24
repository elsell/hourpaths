BEGIN;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM public.activity_mutation_models
    WHERE operation = 'activity.delete'
       OR result_activity_deleted
       OR result_accumulated_seconds IS NOT NULL
  ) THEN
    RAISE EXCEPTION 'cannot remove activity deletion replay state while deletion evidence exists';
  END IF;
END
$$;

REVOKE ALL ON public.recorded_activity_models FROM app;

ALTER TABLE public.activity_mutation_models
  DROP CONSTRAINT activity_mutation_models_operation_check,
  DROP CONSTRAINT activity_mutation_models_operation_state_check,
  DROP CONSTRAINT activity_mutation_models_saved_result_check,
  DROP CONSTRAINT activity_mutation_models_deleted_result_check,
  DROP CONSTRAINT activity_mutation_models_accumulated_seconds_check;

ALTER TABLE public.activity_mutation_models
  DROP COLUMN result_accumulated_seconds,
  DROP COLUMN result_activity_deleted;

ALTER TABLE public.activity_mutation_models
  ADD CONSTRAINT activity_mutation_models_operation_check CHECK (
    operation IN (
      'activity.timer.start',
      'activity.timer.stop',
      'activity.manual.create',
      'activity.update'
    )
  ),
  ADD CONSTRAINT activity_mutation_models_operation_state_check CHECK (
    (
      operation = 'activity.timer.start'
      AND timer_id IS NOT NULL
      AND result_started_at IS NOT NULL
      AND result_time_zone <> ''
      AND NOT result_activity_saved
      AND result_note IS NULL
      AND result_version IS NULL
    )
    OR (
      operation = 'activity.timer.stop'
      AND timer_id IS NOT NULL
      AND result_note IS NULL
      AND result_version IS NULL
    )
    OR (
      operation = 'activity.manual.create'
      AND timer_id IS NULL
      AND result_activity_saved
      AND result_version = 1
    )
    OR (
      operation = 'activity.update'
      AND timer_id IS NULL
      AND result_activity_saved
      AND result_version > 1
    )
  ),
  ADD CONSTRAINT activity_mutation_models_saved_result_check CHECK (
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

GRANT SELECT, INSERT ON public.recorded_activity_models TO app;
GRANT UPDATE (started_at, ended_at, occurrence_time_zone, note, updated_at)
  ON public.recorded_activity_models TO app;

COMMIT;
