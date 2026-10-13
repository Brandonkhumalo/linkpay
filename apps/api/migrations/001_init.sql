CREATE TABLE IF NOT EXISTS users (
 id uuid PRIMARY KEY, email text NOT NULL UNIQUE, password_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS sessions (
 token_hash text PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS password_reset_tokens (
 token_hash text PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 expires_at timestamptz NOT NULL, consumed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS password_reset_user_idx ON password_reset_tokens(user_id);
CREATE TABLE IF NOT EXISTS user_security_events (
 id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 action text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS businesses (
 id uuid PRIMARY KEY, name text NOT NULL, slug text NOT NULL UNIQUE,
 description text NOT NULL DEFAULT '', whatsapp text NOT NULL DEFAULT '',
 email text NOT NULL DEFAULT '', currency char(3) NOT NULL DEFAULT 'USD',
 timezone text NOT NULL DEFAULT 'UTC', theme_color text NOT NULL DEFAULT '#4f46e5',
 published boolean NOT NULL DEFAULT false, plan text NOT NULL DEFAULT 'free' CHECK (plan IN ('free','starter','business')),
 next_invoice_number bigint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS business_members (
 business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 role text NOT NULL CHECK (role IN ('owner','member')),
 PRIMARY KEY (business_id,user_id)
);
CREATE INDEX IF NOT EXISTS business_members_user_idx ON business_members(user_id);
CREATE TABLE IF NOT EXISTS products (
 id uuid PRIMARY KEY, business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
 name text NOT NULL, description text NOT NULL DEFAULT '', kind text NOT NULL CHECK (kind IN ('product','service')),
 price_minor bigint NOT NULL CHECK (price_minor >= 0), active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE (business_id,id)
);
CREATE INDEX IF NOT EXISTS products_business_idx ON products(business_id,created_at DESC);
CREATE TABLE IF NOT EXISTS customers (
 id uuid PRIMARY KEY, business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
 name text NOT NULL, email text NOT NULL DEFAULT '', phone text NOT NULL DEFAULT '',
 notes text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (business_id,id)
);
CREATE INDEX IF NOT EXISTS customers_business_idx ON customers(business_id,name);
CREATE TABLE IF NOT EXISTS enquiries (
 id uuid PRIMARY KEY, business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
 product_id uuid, name text NOT NULL, email text NOT NULL DEFAULT '', phone text NOT NULL DEFAULT '',
 message text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (business_id,product_id) REFERENCES products(business_id,id)
);
CREATE INDEX IF NOT EXISTS enquiries_business_idx ON enquiries(business_id,created_at DESC);
CREATE TABLE IF NOT EXISTS invoices (
 id uuid PRIMARY KEY, business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
 customer_id uuid NOT NULL, number bigint NOT NULL, public_token_hash text UNIQUE,
 status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','sent','void')),
 currency char(3) NOT NULL, issued_at timestamptz, due_at date NOT NULL,
 subtotal_minor bigint NOT NULL CHECK (subtotal_minor >= 0), discount_minor bigint NOT NULL DEFAULT 0 CHECK (discount_minor >= 0),
 tax_minor bigint NOT NULL DEFAULT 0 CHECK (tax_minor >= 0), total_minor bigint NOT NULL CHECK (total_minor >= 0),
 notes text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (business_id,id), UNIQUE (business_id,number),
 FOREIGN KEY (business_id,customer_id) REFERENCES customers(business_id,id)
);
CREATE INDEX IF NOT EXISTS invoices_business_due_idx ON invoices(business_id,status,due_at);
CREATE TABLE IF NOT EXISTS invoice_items (
 id uuid PRIMARY KEY, business_id uuid NOT NULL, invoice_id uuid NOT NULL,
 description text NOT NULL, quantity integer NOT NULL CHECK (quantity > 0), unit_minor bigint NOT NULL CHECK (unit_minor >= 0),
 total_minor bigint NOT NULL CHECK (total_minor >= 0),
 FOREIGN KEY (business_id,invoice_id) REFERENCES invoices(business_id,id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS invoice_items_invoice_idx ON invoice_items(business_id,invoice_id);
CREATE TABLE IF NOT EXISTS payments (
 id uuid PRIMARY KEY, business_id uuid NOT NULL, invoice_id uuid NOT NULL,
 amount_minor bigint NOT NULL CHECK (amount_minor > 0), method text NOT NULL,
 reference text NOT NULL DEFAULT '', idempotency_key text NOT NULL,
 recorded_by uuid NOT NULL REFERENCES users(id),
 recorded_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (business_id,invoice_id) REFERENCES invoices(business_id,id),
 UNIQUE (business_id,invoice_id,idempotency_key)
);
CREATE INDEX IF NOT EXISTS payments_invoice_idx ON payments(business_id,invoice_id);
CREATE TABLE IF NOT EXISTS reminder_rules (
 business_id uuid PRIMARY KEY REFERENCES businesses(id) ON DELETE CASCADE,
 enabled boolean NOT NULL DEFAULT false, offsets integer[] NOT NULL DEFAULT '{-3,0,3,7}',
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS reminder_deliveries (
 id uuid PRIMARY KEY, business_id uuid NOT NULL, invoice_id uuid NOT NULL,
 offset_days integer NOT NULL, status text NOT NULL CHECK (status IN ('pending','sending','sent','failed','skipped')),
 attempts integer NOT NULL DEFAULT 0, next_attempt_at timestamptz NOT NULL DEFAULT now(),
 last_error text NOT NULL DEFAULT '', sent_at timestamptz,
 UNIQUE (invoice_id,offset_days),
 FOREIGN KEY (business_id,invoice_id) REFERENCES invoices(business_id,id)
);
CREATE INDEX IF NOT EXISTS reminder_pending_idx ON reminder_deliveries(status,next_attempt_at);
CREATE TABLE IF NOT EXISTS audit_logs (
 id uuid PRIMARY KEY, business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
 actor_id uuid REFERENCES users(id), action text NOT NULL, subject_id uuid NOT NULL,
 details jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_business_idx ON audit_logs(business_id,created_at DESC);
CREATE TABLE IF NOT EXISTS admin_audit_logs (
 id uuid PRIMARY KEY, actor_id uuid NOT NULL REFERENCES users(id), action text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
