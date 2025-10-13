-- Add sweep tracking fields to payment_orders table

ALTER TABLE payment_orders 
ADD COLUMN IF NOT EXISTS swept_at TIMESTAMP WITH TIME ZONE,
ADD COLUMN IF NOT EXISTS sweep_tx_hash VARCHAR(66);

-- Add index for querying unswept orders
CREATE INDEX IF NOT EXISTS idx_payment_orders_swept_at 
ON payment_orders(swept_at) 
WHERE swept_at IS NULL AND status = 'validated';

-- Add comment
COMMENT ON COLUMN payment_orders.swept_at IS 'Timestamp when funds were swept from receive address to gateway';
COMMENT ON COLUMN payment_orders.sweep_tx_hash IS 'Transaction hash of the sweep transaction';
