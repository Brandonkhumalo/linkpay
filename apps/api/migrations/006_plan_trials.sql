ALTER TABLE businesses
 ADD COLUMN trial_plan text,
 ADD COLUMN trial_started_at timestamptz,
 ADD COLUMN trial_expires_at timestamptz,
 ADD CONSTRAINT businesses_trial_window_check CHECK (
  (trial_plan IS NULL AND trial_started_at IS NULL AND trial_expires_at IS NULL)
  OR (trial_plan IN ('starter', 'business') AND trial_started_at IS NOT NULL AND trial_expires_at IS NOT NULL
      AND trial_expires_at = trial_started_at + interval '168 hours')
 );
