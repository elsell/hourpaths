BEGIN;

ALTER TABLE public.path_models
  ADD COLUMN interval_goal_target_seconds bigint,
  ADD COLUMN interval_goal_recurrence text,
  ADD COLUMN interval_goal_start_minute smallint,
  ADD COLUMN interval_goal_start_hour smallint,
  ADD COLUMN interval_goal_start_weekday smallint,
  ADD COLUMN interval_goal_start_day smallint,
  ADD COLUMN interval_goal_start_month smallint,
  ADD COLUMN overall_target_seconds bigint;

ALTER TABLE public.path_models
  ADD CONSTRAINT path_models_interval_goal_check CHECK (
    (
      interval_goal_target_seconds IS NULL
      AND interval_goal_recurrence IS NULL
      AND interval_goal_start_minute IS NULL
      AND interval_goal_start_hour IS NULL
      AND interval_goal_start_weekday IS NULL
      AND interval_goal_start_day IS NULL
      AND interval_goal_start_month IS NULL
    )
    OR (
      interval_goal_target_seconds IS NOT NULL
      AND interval_goal_target_seconds > 0
      AND interval_goal_recurrence IS NOT NULL
      AND (
        (
          interval_goal_recurrence = 'hourly'
          AND interval_goal_start_minute IS NOT NULL
          AND interval_goal_start_minute BETWEEN 0 AND 59
          AND interval_goal_start_hour IS NULL
          AND interval_goal_start_weekday IS NULL
          AND interval_goal_start_day IS NULL
          AND interval_goal_start_month IS NULL
        )
        OR (
          interval_goal_recurrence = 'daily'
          AND interval_goal_start_minute IS NULL
          AND interval_goal_start_hour IS NOT NULL
          AND interval_goal_start_hour BETWEEN 0 AND 23
          AND interval_goal_start_weekday IS NULL
          AND interval_goal_start_day IS NULL
          AND interval_goal_start_month IS NULL
        )
        OR (
          interval_goal_recurrence = 'weekly'
          AND interval_goal_start_minute IS NULL
          AND interval_goal_start_hour IS NULL
          AND interval_goal_start_weekday IS NOT NULL
          AND interval_goal_start_weekday BETWEEN 1 AND 7
          AND interval_goal_start_day IS NULL
          AND interval_goal_start_month IS NULL
        )
        OR (
          interval_goal_recurrence = 'monthly'
          AND interval_goal_start_minute IS NULL
          AND interval_goal_start_hour IS NULL
          AND interval_goal_start_weekday IS NULL
          AND interval_goal_start_day IS NOT NULL
          AND interval_goal_start_day BETWEEN 1 AND 31
          AND interval_goal_start_month IS NULL
        )
        OR (
          interval_goal_recurrence = 'yearly'
          AND interval_goal_start_minute IS NULL
          AND interval_goal_start_hour IS NULL
          AND interval_goal_start_weekday IS NULL
          AND interval_goal_start_day IS NOT NULL
          AND interval_goal_start_month IS NOT NULL
          AND interval_goal_start_month BETWEEN 1 AND 12
          AND interval_goal_start_day BETWEEN 1 AND CASE interval_goal_start_month
            WHEN 2 THEN 29
            WHEN 4 THEN 30
            WHEN 6 THEN 30
            WHEN 9 THEN 30
            WHEN 11 THEN 30
            ELSE 31
          END
        )
      )
    )
  ),
  ADD CONSTRAINT path_models_overall_target_check CHECK (
    overall_target_seconds IS NULL OR overall_target_seconds > 0
  );

COMMIT;
