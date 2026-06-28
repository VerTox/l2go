-- L2Go Database Initialization Script
-- This script is executed when the PostgreSQL container starts for the first time

-- Create additional databases if needed
-- CREATE DATABASE l2go_game;

-- Create user with appropriate permissions (optional, since we're using postgres user)
-- CREATE USER l2go_user WITH PASSWORD 'l2go_password';
-- GRANT ALL PRIVILEGES ON DATABASE l2go_login TO l2go_user;

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- The main tables will be created by the application migrations
-- This script is mainly for setting up the environment

-- You can add any additional setup here, such as:
-- - Creating indexes
-- - Setting up roles
-- - Configuring database settings

-- Log that initialization is complete
DO $$
BEGIN
    RAISE NOTICE 'L2Go database initialization completed successfully';
END $$;