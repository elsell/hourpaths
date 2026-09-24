BEGIN;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM public.path_models
    WHERE interval_goal_target_seconds IS NOT NULL
      OR interval_goal_recurrence IS NOT NULL
      OR interval_goal_start_minute IS NOT NULL
      OR interval_goal_start_hour IS NOT NULL
      OR interval_goal_start_weekday IS NOT NULL
      OR interval_goal_start_day IS NOT NULL
      OR interval_goal_start_month IS NOT NULL
      OR overall_target_seconds IS NOT NULL
  ) THEN
    RAISE EXCEPTION 'cannot remove Path goal persistence while interval or overall goal data exists';
  END IF;
END
$$;

ALTER TABLE public.path_models DROP CONSTRAINT path_models_interval_goal_check;
ALTER TABLE public.path_models DROP CONSTRAINT path_models_overall_target_check;

ALTER TABLE public.path_models
  DROP COLUMN interval_goal_target_seconds,
  DROP COLUMN interval_goal_recurrence,
  DROP COLUMN interval_goal_start_minute,
  DROP COLUMN interval_goal_start_hour,
  DROP COLUMN interval_goal_start_weekday,
  DROP COLUMN interval_goal_start_day,
  DROP COLUMN interval_goal_start_month,
  DROP COLUMN overall_target_seconds;

COMMIT;
