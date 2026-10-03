-- Restoring foreign keys cannot recreate deleted accounts or discard deletion
-- records needed for backup recovery. Roll forward after account deletion.
DO $$ BEGIN
  RAISE EXCEPTION 'account deletion migration requires a forward migration';
END $$;
