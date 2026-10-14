ALTER TABLE invoices
  ADD COLUMN recipient_business_name text,
  ADD COLUMN recipient_email text,
  ADD COLUMN recipient_phone text,
  ADD COLUMN sender_business_name text,
  ADD COLUMN sender_phone text;

UPDATE invoices i SET
  recipient_business_name = c.name,
  recipient_email = c.email,
  recipient_phone = c.phone,
  sender_business_name = b.name,
  sender_phone = b.whatsapp
FROM customers c, businesses b
WHERE c.id = i.customer_id AND c.business_id = i.business_id AND b.id = i.business_id;
