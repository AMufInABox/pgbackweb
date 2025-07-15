-- +goose Up
-- +goose StatementBegin
ALTER TABLE destinations 
ADD COLUMN encryption_type TEXT NOT NULL DEFAULT 'none',
ADD COLUMN encryption_key_id TEXT,
ADD COLUMN encryption_key_region TEXT;

-- Add constraint to ensure encryption_key_id is provided when using customer-managed keys
ALTER TABLE destinations ADD CONSTRAINT destinations_encryption_check 
CHECK (
    (encryption_type = 'none') OR
    (encryption_type = 'aes256') OR
    (encryption_type = 'aws:kms' AND encryption_key_id IS NOT NULL) OR
    (encryption_type = 'aws:kms' AND encryption_key_id IS NOT NULL AND encryption_key_region IS NOT NULL) OR
    (encryption_type = 'sse-c' AND encryption_key_id IS NOT NULL)
);

-- Create index for encryption lookups
CREATE INDEX idx_destinations_encryption_type ON destinations(encryption_type);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_destinations_encryption_type;
DROP CONSTRAINT IF EXISTS destinations_encryption_check;
ALTER TABLE destinations 
DROP COLUMN IF EXISTS encryption_key_region,
DROP COLUMN IF EXISTS encryption_key_id,
DROP COLUMN IF EXISTS encryption_type;
-- +goose StatementEnd
