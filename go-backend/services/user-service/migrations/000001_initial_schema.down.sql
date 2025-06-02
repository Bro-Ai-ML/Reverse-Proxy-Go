-- Drop functions
DROP FUNCTION IF EXISTS has_all_roles(UUID, TEXT[]);
DROP FUNCTION IF EXISTS has_any_role(UUID, TEXT[]);
DROP FUNCTION IF EXISTS has_role(UUID, TEXT);
DROP FUNCTION IF EXISTS get_user_roles(UUID);
DROP FUNCTION IF EXISTS search_users(TEXT);

-- Drop triggers
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables
DROP TABLE IF EXISTS password_reset_tokens CASCADE;
DROP TABLE IF EXISTS email_verification_tokens CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- Drop types
DROP TYPE IF EXISTS user_status;

-- Drop extensions
DROP EXTENSION IF EXISTS "uuid-ossp";
