CREATE INDEX merchant_notifications_unread_business_idx
ON merchant_notifications(business_id)
WHERE read_at IS NULL;
