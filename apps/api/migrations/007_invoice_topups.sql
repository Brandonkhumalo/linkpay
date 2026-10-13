ALTER TABLE billing_payments
 ADD COLUMN kind text NOT NULL DEFAULT 'subscription' CHECK (kind IN ('subscription', 'invoice_topup')),
 ADD COLUMN invoice_quantity integer NOT NULL DEFAULT 0,
 ALTER COLUMN plan DROP NOT NULL,
 ALTER COLUMN months DROP NOT NULL,
 ADD CONSTRAINT billing_payments_purchase_check CHECK (
  (kind = 'subscription' AND plan IN ('starter', 'business') AND months IN (1, 12) AND invoice_quantity = 0)
  OR (kind = 'invoice_topup' AND plan IS NULL AND months IS NULL AND invoice_quantity IN (30, 100))
 );

ALTER TABLE invoices
 ADD COLUMN quota_source text NOT NULL DEFAULT 'subscription'
 CHECK (quota_source IN ('subscription', 'invoice_topup'));

CREATE INDEX invoices_monthly_subscription_idx ON invoices(business_id, created_at)
 WHERE quota_source = 'subscription';
CREATE INDEX invoices_topup_usage_idx ON invoices(business_id)
 WHERE quota_source = 'invoice_topup';
CREATE INDEX billing_paid_topups_idx ON billing_payments(business_id)
 WHERE kind = 'invoice_topup' AND status = 'paid';
