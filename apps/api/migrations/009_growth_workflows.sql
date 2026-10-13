ALTER TABLE enquiries ADD COLUMN stage text NOT NULL DEFAULT 'new'
  CHECK (stage IN ('new','contacted','quoted','won','lost'));
ALTER TABLE enquiries ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE enquiries ADD CONSTRAINT enquiries_business_id_unique UNIQUE (business_id,id);
CREATE INDEX enquiries_business_stage_idx ON enquiries(business_id,stage,created_at DESC);

CREATE TABLE quotes (
 id uuid PRIMARY KEY,
 business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
 enquiry_id uuid NOT NULL,
 public_token_hash text NOT NULL UNIQUE,
 description text NOT NULL CHECK (length(description) BETWEEN 2 AND 2000),
 amount_minor bigint NOT NULL CHECK (amount_minor > 0 AND amount_minor <= 1000000000),
 currency char(3) NOT NULL,
 expires_at date NOT NULL,
 status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','accepted','declined','converted')),
 responded_at timestamptz,
 invoice_id uuid,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (business_id,id),
 FOREIGN KEY (business_id,enquiry_id) REFERENCES enquiries(business_id,id),
 FOREIGN KEY (business_id,invoice_id) REFERENCES invoices(business_id,id)
);
CREATE INDEX quotes_business_idx ON quotes(business_id,created_at DESC);
CREATE INDEX quotes_enquiry_idx ON quotes(business_id,enquiry_id);

ALTER TABLE businesses ADD COLUMN payment_instructions text NOT NULL DEFAULT '';
ALTER TABLE invoices ADD COLUMN source_quote_id uuid;
ALTER TABLE invoices ADD CONSTRAINT invoices_source_quote_fk
 FOREIGN KEY (business_id,source_quote_id) REFERENCES quotes(business_id,id);

CREATE TABLE invoice_schedules (
 id uuid PRIMARY KEY,
 business_id uuid NOT NULL,
 source_invoice_id uuid NOT NULL,
 interval_months integer NOT NULL CHECK (interval_months BETWEEN 1 AND 12),
 next_run_on date NOT NULL,
 active boolean NOT NULL DEFAULT true,
 last_error text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (business_id,source_invoice_id) REFERENCES invoices(business_id,id),
 UNIQUE (business_id,source_invoice_id)
);
CREATE INDEX invoice_schedules_due_idx ON invoice_schedules(next_run_on) WHERE active;

CREATE TABLE customer_followups (
 id uuid PRIMARY KEY,
 business_id uuid NOT NULL,
 customer_id uuid NOT NULL,
 due_on date NOT NULL,
 note text NOT NULL CHECK (length(note) BETWEEN 2 AND 500),
 completed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (business_id,customer_id) REFERENCES customers(business_id,id)
);
CREATE INDEX customer_followups_due_idx ON customer_followups(business_id,due_on) WHERE completed_at IS NULL;

CREATE TABLE merchant_notifications (
 id uuid PRIMARY KEY,
 business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK (kind IN ('enquiry','quote_accepted','quote_declined','payment_claim','followup')),
 subject_id uuid NOT NULL,
 title text NOT NULL,
 email_status text NOT NULL DEFAULT 'pending' CHECK (email_status IN ('pending','sending','sent','failed','disabled')),
 attempts integer NOT NULL DEFAULT 0,
 next_attempt_at timestamptz NOT NULL DEFAULT now(),
 read_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (business_id,kind,subject_id)
);
CREATE INDEX merchant_notifications_pending_idx ON merchant_notifications(email_status,next_attempt_at);

CREATE TABLE payment_claims (
 id uuid PRIMARY KEY,
 business_id uuid NOT NULL,
 invoice_id uuid NOT NULL,
 amount_minor bigint NOT NULL CHECK (amount_minor > 0),
 reference text NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','dismissed','verified')),
 created_at timestamptz NOT NULL DEFAULT now(),
 resolved_at timestamptz,
 FOREIGN KEY (business_id,invoice_id) REFERENCES invoices(business_id,id)
);
CREATE INDEX payment_claims_invoice_idx ON payment_claims(business_id,invoice_id,created_at DESC);
CREATE UNIQUE INDEX payment_claims_one_pending_idx ON payment_claims(invoice_id) WHERE status='pending';
