BEGIN;

ALTER TABLE public.recorded_activity_models ADD COLUMN note text;
ALTER TABLE public.recorded_activity_models
  ADD CONSTRAINT recorded_activity_models_note_check CHECK (
    note IS NULL
    OR (
      char_length(note) BETWEEN 1 AND 2000
      AND btrim(note) <> ''
      AND note !~ E'[\\x01-\\x09\\x0B-\\x0C\\x0E-\\x1F\\x7F]'
    )
  );

CREATE INDEX recorded_activity_models_path_created_idx
  ON public.recorded_activity_models(path_id, created_at DESC, id DESC);

CREATE TABLE public.recorded_activity_revision_models (
  activity_id text NOT NULL REFERENCES public.recorded_activity_models(id) ON DELETE CASCADE,
  version bigint NOT NULL CHECK (version > 0),
  started_at timestamptz NOT NULL,
  ended_at timestamptz NOT NULL,
  occurrence_time_zone text NOT NULL CHECK (
    occurrence_time_zone = btrim(occurrence_time_zone)
    AND occurrence_time_zone <> ''
    AND occurrence_time_zone <> 'Local'
  ),
  note text,
  public_changed boolean NOT NULL,
  updated_at timestamptz NOT NULL,
  replaced_at timestamptz NOT NULL,
  PRIMARY KEY (activity_id, version),
  CHECK (ended_at >= started_at + interval '1 second'),
  CHECK (replaced_at >= updated_at),
  CONSTRAINT recorded_activity_revision_models_note_check CHECK (
    note IS NULL
    OR (
      char_length(note) BETWEEN 1 AND 2000
      AND btrim(note) <> ''
      AND note !~ E'[\\x01-\\x09\\x0B-\\x0C\\x0E-\\x1F\\x7F]'
    )
  )
);

CREATE INDEX recorded_activity_revision_models_activity_replaced_idx
  ON public.recorded_activity_revision_models(activity_id, replaced_at, version);

ALTER TABLE public.activity_mutation_models
  DROP CONSTRAINT activity_mutation_models_operation_check,
  DROP CONSTRAINT activity_mutation_models_check,
  DROP CONSTRAINT activity_mutation_models_check1;

ALTER TABLE public.activity_mutation_models ALTER COLUMN timer_id DROP NOT NULL;
ALTER TABLE public.activity_mutation_models
  ADD COLUMN result_note text,
  ADD COLUMN result_version bigint;

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
  ),
  ADD CONSTRAINT activity_mutation_models_result_note_check CHECK (
    result_note IS NULL
    OR (
      char_length(result_note) BETWEEN 1 AND 2000
      AND btrim(result_note) <> ''
      AND result_note !~ E'[\\x01-\\x09\\x0B-\\x0C\\x0E-\\x1F\\x7F]'
    )
  );

REVOKE ALL ON public.recorded_activity_models FROM app;
GRANT SELECT, INSERT ON public.recorded_activity_models TO app;
GRANT UPDATE (started_at, ended_at, occurrence_time_zone, note, updated_at)
  ON public.recorded_activity_models TO app;
REVOKE ALL ON public.recorded_activity_revision_models FROM app;
GRANT SELECT, INSERT ON public.recorded_activity_revision_models TO app;
REVOKE ALL ON public.activity_mutation_models FROM app;
GRANT SELECT, INSERT, UPDATE ON public.activity_mutation_models TO app;

COMMIT;
