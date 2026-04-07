-- Add Telegram user information fields
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_username VARCHAR(64);
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_first_name VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_last_name VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_photo_url TEXT;

-- Trigger to update updated_at on changes
CREATE OR REPLACE FUNCTION update_users_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_users_updated_at_trigger ON users;
CREATE TRIGGER update_users_updated_at_trigger
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_users_updated_at();
