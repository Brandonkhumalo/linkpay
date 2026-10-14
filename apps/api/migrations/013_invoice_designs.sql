ALTER TABLE invoices
  ADD COLUMN design_template text NOT NULL DEFAULT 'simple'
    CHECK (design_template IN ('simple','modern','corporate','executive','ledger')),
  ADD COLUMN design_color char(7) NOT NULL DEFAULT '#235743'
    CHECK (design_color ~ '^#[0-9A-Fa-f]{6}$'),
  ADD COLUMN show_logo boolean NOT NULL DEFAULT false,
  ADD COLUMN logo_id uuid,
  ADD COLUMN logo_ext text CHECK (logo_ext IN ('jpg','png')),
  ADD COLUMN logo_size bigint NOT NULL DEFAULT 0 CHECK (logo_size >= 0),
  ADD CONSTRAINT invoices_logo_pair CHECK ((logo_id IS NULL) = (logo_ext IS NULL)),
  ADD CONSTRAINT invoices_logo_enabled CHECK (NOT show_logo OR logo_id IS NOT NULL);

CREATE INDEX invoices_logo_reference_idx ON invoices(business_id,logo_id)
WHERE logo_id IS NOT NULL;
