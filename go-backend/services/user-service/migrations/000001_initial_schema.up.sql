-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create enum types
CREATE TYPE user_status AS ENUM ('pending', 'active', 'suspended');

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    avatar_url TEXT,
    bio TEXT,
    is_active BOOLEAN DEFAULT FALSE,
    is_verified BOOLEAN DEFAULT FALSE,
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    roles TEXT[] DEFAULT '{}'::TEXT[],
    
    -- RGPD Fields
    data_portability BOOLEAN DEFAULT FALSE,
    marketing_consent BOOLEAN DEFAULT FALSE,
    terms_accepted BOOLEAN DEFAULT FALSE,
    terms_version VARCHAR(50),
    last_ip VARCHAR(45),
    user_agent TEXT
);

-- Indexes for users table
CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_created_at ON users(created_at);
CREATE INDEX idx_users_deleted_at ON users(deleted_at);
CREATE INDEX idx_users_marketing_consent ON users(marketing_consent) WHERE marketing_consent = true;
CREATE INDEX idx_users_terms_accepted ON users(terms_accepted, terms_version) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_roles ON users USING GIN (roles);

-- Email verification tokens table
CREATE TABLE email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes for email verification tokens
CREATE INDEX idx_email_verification_tokens_token ON email_verification_tokens (token);
CREATE INDEX idx_email_verification_tokens_user_id ON email_verification_tokens (user_id);

-- Password reset tokens table
CREATE TABLE password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    used_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for password reset tokens
CREATE INDEX idx_password_reset_tokens_token ON password_reset_tokens (token);
CREATE INDEX idx_password_reset_tokens_user_id ON password_reset_tokens (user_id);

-- Function to update the updated_at column
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update updated_at on users table
CREATE TRIGGER update_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- Create a function to search users
CREATE OR REPLACE FUNCTION search_users(search_term TEXT)
RETURNS TABLE (
    id UUID,
    email VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    avatar_url TEXT,
    bio TEXT,
    is_active BOOLEAN,
    is_verified BOOLEAN,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    roles TEXT[],
    rank FLOAT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        u.*,
        ts_rank(
            to_tsvector('english', 
                COALESCE(u.email, '') || ' ' || 
                COALESCE(u.first_name, '') || ' ' || 
                COALESCE(u.last_name, '') || ' ' || 
                COALESCE(u.bio, '')
            ),
            plainto_tsquery('english', search_term)
        ) as rank
    FROM users u
    WHERE 
        to_tsvector('english', 
            COALESCE(u.email, '') || ' ' || 
            COALESCE(u.first_name, '') || ' ' || 
            COALESCE(u.last_name, '') || ' ' || 
            COALESCE(u.bio, '')
        ) @@ plainto_tsquery('english', search_term)
        AND u.deleted_at IS NULL
    ORDER BY rank DESC;
END;
$$ LANGUAGE plpgsql;

-- Create a function to get user roles
CREATE OR REPLACE FUNCTION get_user_roles(p_user_id UUID)
RETURNS TEXT[] AS $$
DECLARE
    user_roles TEXT[];
BEGIN
    SELECT roles INTO user_roles
    FROM users
    WHERE id = p_user_id AND deleted_at IS NULL;
    
    RETURN COALESCE(user_roles, '{}'::TEXT[]);
END;
$$ LANGUAGE plpgsql;

-- Create a function to check if a user has a role
CREATE OR REPLACE FUNCTION has_role(p_user_id UUID, p_role TEXT)
RETURNS BOOLEAN AS $$
DECLARE
    has_role BOOLEAN;
BEGIN
    SELECT p_role = ANY(roles) INTO has_role
    FROM users
    WHERE id = p_user_id AND deleted_at IS NULL;
    
    RETURN COALESCE(has_role, FALSE);
END;
$$ LANGUAGE plpgsql;

-- Create a function to check if a user has any of the specified roles
CREATE OR REPLACE FUNCTION has_any_role(p_user_id UUID, p_roles TEXT[])
RETURNS BOOLEAN AS $$
DECLARE
    has_any BOOLEAN;
BEGIN
    SELECT roles && p_roles INTO has_any
    FROM users
    WHERE id = p_user_id AND deleted_at IS NULL;
    
    RETURN COALESCE(has_any, FALSE);
END;
$$ LANGUAGE plpgsql;

-- Create a function to check if a user has all of the specified roles
CREATE OR REPLACE FUNCTION has_all_roles(p_user_id UUID, p_roles TEXT[])
RETURNS BOOLEAN AS $$
DECLARE
    has_all BOOLEAN;
BEGIN
    SELECT roles @> p_roles INTO has_all
    FROM users
    WHERE id = p_user_id AND deleted_at IS NULL;
    
    RETURN COALESCE(has_all, FALSE);
END;
$$ LANGUAGE plpgsql;
