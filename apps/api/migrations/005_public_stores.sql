CREATE INDEX IF NOT EXISTS businesses_published_name_idx ON businesses(name,id) WHERE published=true;
