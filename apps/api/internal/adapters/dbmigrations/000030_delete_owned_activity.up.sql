BEGIN;

ALTER TABLE public.activity_mutation_models
  DROP CONSTRAINT activity_mutation_models_operation_check,
  DROP CONSTRAINT activity_mutation_models_operation_state_check,
  DROP CONSTRAINT activity_mutation_models_saved_result_check;

ALTER TABLE public.activity_mutation_models
  ADD COLUMN result_activity_deleted boolean NOT NULL DEFAULT false,
  ADD COLUMN result_accumulated_seconds bigint;

ALTER TABLE public.activity_mutation_models
  ADD CONSTRAINT activity_mutation_models_operation_check CHECK (
    operation IN (
      'activity.timer.start',
      'activity.timer.stop',
      'activity.manual.create',
      'activity.update',
      'activity.delete'
    )
  ),
  ADD CONSTRAINT activity_mutation_models_operation_state_check CHECK (
    (
      operation = 'activity.timer.start'
      AND timer_id IS NOT NULL
      AND result_started_at IS NOT NULL
      AND result_time_zone <> ''
      AND NOT result_activity_saved
      AND NOT result_activity_deleted
      AND result_note IS NULL
      AND result_version IS NULL
      AND result_accumulated_seconds IS NULL
    )
    OR (
      operation = 'activity.timer.stop'
      AND timer_id IS NOT NULL
      AND result_note IS NULL
      AND result_version IS NULL
      AND result_accumulated_seconds IS NULL
    )
    OR (
      operation IN ('activity.manual.create', 'activity.update')
      AND timer_id IS NULL
      AND result_accumulated_seconds IS NULL
      AND (
        (result_activity_saved AND NOT result_activity_deleted)
        OR (NOT result_activity_saved AND result_activity_deleted)
      )
    )
    OR (
      operation = 'activity.delete'
      AND timer_id IS NULL
      AND NOT result_activity_saved
      AND result_activity_deleted
      AND result_activity_id IS NOT NULL
      AND result_accumulated_seconds IS NOT NULL
    )
  ),
  ADD CONSTRAINT activity_mutation_models_saved_result_check CHECK (
    NOT result_activity_saved
    OR (
      NOT result_activity_deleted
      AND result_activity_id IS NOT NULL
      AND result_started_at IS NOT NULL
      AND result_ended_at IS NOT NULL
      AND result_time_zone <> ''
      AND result_created_at IS NOT NULL
      AND result_updated_at IS NOT NULL
    )
  ),
  ADD CONSTRAINT activity_mutation_models_deleted_result_check CHECK (
    NOT result_activity_deleted
    OR (
      result_activity_id IS NOT NULL
      AND result_started_at IS NULL
      AND result_ended_at IS NULL
      AND result_time_zone = ''
      AND result_created_at IS NULL
      AND result_updated_at IS NULL
      AND result_note IS NULL
      AND result_version IS NULL
    )
  ),
  ADD CONSTRAINT activity_mutation_models_accumulated_seconds_check CHECK (
    result_accumulated_seconds IS NULL OR result_accumulated_seconds >= 0
  );

REVOKE ALL ON public.recorded_activity_models FROM app;
GRANT SELECT, INSERT ON public.recorded_activity_models TO app;
GRANT UPDATE (started_at, ended_at, occurrence_time_zone, note, updated_at)
  ON public.recorded_activity_models TO app;
GRANT DELETE ON public.recorded_activity_models TO app;

COMMIT;
