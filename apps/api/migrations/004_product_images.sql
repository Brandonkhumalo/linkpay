ALTER TABLE products ADD COLUMN image_id uuid;
ALTER TABLE products ADD COLUMN image_ext text CHECK (image_ext IN ('jpg','png'));
ALTER TABLE products ADD COLUMN image_size bigint NOT NULL DEFAULT 0 CHECK (image_size >= 0);
ALTER TABLE products ADD CONSTRAINT products_image_pair CHECK ((image_id IS NULL) = (image_ext IS NULL));
