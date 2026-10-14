ALTER TABLE businesses
  ADD COLUMN logo_id uuid,
  ADD COLUMN logo_ext text CHECK (logo_ext IN ('jpg', 'png')),
  ADD COLUMN logo_size bigint NOT NULL DEFAULT 0 CHECK (logo_size >= 0),
  ADD CONSTRAINT businesses_logo_pair CHECK ((logo_id IS NULL) = (logo_ext IS NULL));
