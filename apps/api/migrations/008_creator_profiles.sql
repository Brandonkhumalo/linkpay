ALTER TABLE businesses
  ADD COLUMN profile_type text NOT NULL DEFAULT 'business' CHECK (profile_type IN ('business', 'creator')),
  ADD COLUMN creator_niche text NOT NULL DEFAULT '',
  ADD COLUMN creator_location text NOT NULL DEFAULT '',
  ADD COLUMN creator_portfolio_urls text[] NOT NULL DEFAULT '{}';

ALTER TABLE products
  ADD COLUMN price_on_request boolean NOT NULL DEFAULT false,
  ADD CONSTRAINT products_quote_service_only CHECK (NOT price_on_request OR (kind = 'service' AND price_minor = 0));

ALTER TABLE enquiries
  ADD COLUMN brand_name text NOT NULL DEFAULT '',
  ADD COLUMN budget_minor bigint CHECK (budget_minor IS NULL OR budget_minor >= 0),
  ADD COLUMN campaign_deadline date,
  ADD COLUMN deliverables text NOT NULL DEFAULT '',
  ADD COLUMN usage_rights text NOT NULL DEFAULT '';

CREATE INDEX businesses_public_profile_idx ON businesses(profile_type, name, id) WHERE published = true;
