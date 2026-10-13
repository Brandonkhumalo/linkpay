ALTER TABLE businesses ADD COLUMN plan_expires_at timestamptz;

CREATE TABLE billing_payments (
 id uuid PRIMARY KEY,
 business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
 created_by uuid NOT NULL REFERENCES users(id),
 reference text NOT NULL UNIQUE,
 paynow_reference text NOT NULL DEFAULT '',
 plan text NOT NULL CHECK (plan IN ('starter','business')),
 months integer NOT NULL CHECK (months IN (1,12)),
 amount_minor bigint NOT NULL CHECK (amount_minor > 0),
 currency char(3) NOT NULL DEFAULT 'USD',
 status text NOT NULL DEFAULT 'created' CHECK (status IN ('created','pending','paid','cancelled','disputed','refunded','failed')),
 browser_url text NOT NULL DEFAULT '',
 poll_url text NOT NULL DEFAULT '',
 granted_until timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX billing_payments_business_idx ON billing_payments(business_id,created_at DESC);
CREATE INDEX billing_payments_pending_idx ON billing_payments(created_at) WHERE status IN ('created','pending');
