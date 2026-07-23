CREATE SCHEMA IF NOT EXISTS bbs_auth;
CREATE SCHEMA IF NOT EXISTS bbs_user;
CREATE SCHEMA IF NOT EXISTS bbs_content;
CREATE SCHEMA IF NOT EXISTS bbs_reaction;
CREATE SCHEMA IF NOT EXISTS bbs_credit;
CREATE SCHEMA IF NOT EXISTS bbs_notification;
CREATE SCHEMA IF NOT EXISTS bbs_admin;
CREATE SCHEMA IF NOT EXISTS bbs_config;
CREATE SCHEMA IF NOT EXISTS bbs_file;
CREATE SCHEMA IF NOT EXISTS bbs_audit;
CREATE SCHEMA IF NOT EXISTS bbs_mall;
CREATE SCHEMA IF NOT EXISTS bbs_chat;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bbs_auth_app') THEN
    CREATE ROLE bbs_auth_app LOGIN PASSWORD 'local_auth_pass';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bbs_user_app') THEN
    CREATE ROLE bbs_user_app LOGIN PASSWORD 'local_user_pass';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bbs_content_app') THEN
    CREATE ROLE bbs_content_app LOGIN PASSWORD 'local_content_pass';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bbs_reaction_app') THEN
    CREATE ROLE bbs_reaction_app LOGIN PASSWORD 'local_reaction_pass';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bbs_credit_app') THEN
    CREATE ROLE bbs_credit_app LOGIN PASSWORD 'local_credit_pass';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bbs_notification_app') THEN
    CREATE ROLE bbs_notification_app LOGIN PASSWORD 'local_notification_pass';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bbs_admin_app') THEN
    CREATE ROLE bbs_admin_app LOGIN PASSWORD 'local_admin_pass';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bbs_config_app') THEN
    CREATE ROLE bbs_config_app LOGIN PASSWORD 'local_config_pass';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bbs_file_app') THEN
    CREATE ROLE bbs_file_app LOGIN PASSWORD 'local_file_pass';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bbs_audit_app') THEN
    CREATE ROLE bbs_audit_app LOGIN PASSWORD 'local_audit_pass';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bbs_mall_app') THEN
    CREATE ROLE bbs_mall_app LOGIN PASSWORD 'local_mall_pass';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bbs_chat_app') THEN
    CREATE ROLE bbs_chat_app LOGIN PASSWORD 'local_chat_pass';
  END IF;
END $$;

GRANT USAGE, CREATE ON SCHEMA bbs_auth TO bbs_auth_app;
GRANT USAGE, CREATE ON SCHEMA bbs_user TO bbs_user_app;
GRANT USAGE, CREATE ON SCHEMA bbs_content TO bbs_content_app;
GRANT USAGE, CREATE ON SCHEMA bbs_reaction TO bbs_reaction_app;
GRANT USAGE, CREATE ON SCHEMA bbs_credit TO bbs_credit_app;
GRANT USAGE, CREATE ON SCHEMA bbs_notification TO bbs_notification_app;
GRANT USAGE, CREATE ON SCHEMA bbs_admin TO bbs_admin_app;
GRANT USAGE, CREATE ON SCHEMA bbs_config TO bbs_config_app;
GRANT USAGE, CREATE ON SCHEMA bbs_file TO bbs_file_app;
GRANT USAGE, CREATE ON SCHEMA bbs_audit TO bbs_audit_app;
GRANT USAGE, CREATE ON SCHEMA bbs_mall TO bbs_mall_app;
GRANT USAGE, CREATE ON SCHEMA bbs_chat TO bbs_chat_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_auth GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_auth_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_user GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_user_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_content GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_content_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_reaction GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_reaction_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_credit GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_credit_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_notification GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_notification_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_admin GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_admin_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_config GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_config_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_file GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_file_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_audit GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_audit_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_mall GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_mall_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_chat GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_chat_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_auth GRANT USAGE, SELECT ON SEQUENCES TO bbs_auth_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_user GRANT USAGE, SELECT ON SEQUENCES TO bbs_user_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_content GRANT USAGE, SELECT ON SEQUENCES TO bbs_content_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_reaction GRANT USAGE, SELECT ON SEQUENCES TO bbs_reaction_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_credit GRANT USAGE, SELECT ON SEQUENCES TO bbs_credit_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_notification GRANT USAGE, SELECT ON SEQUENCES TO bbs_notification_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_admin GRANT USAGE, SELECT ON SEQUENCES TO bbs_admin_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_config GRANT USAGE, SELECT ON SEQUENCES TO bbs_config_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_file GRANT USAGE, SELECT ON SEQUENCES TO bbs_file_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_audit GRANT USAGE, SELECT ON SEQUENCES TO bbs_audit_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_mall GRANT USAGE, SELECT ON SEQUENCES TO bbs_mall_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_chat GRANT USAGE, SELECT ON SEQUENCES TO bbs_chat_app;

ALTER ROLE bbs_auth_app SET search_path TO bbs_auth, public;
ALTER ROLE bbs_user_app SET search_path TO bbs_user, public;
ALTER ROLE bbs_content_app SET search_path TO bbs_content, public;
ALTER ROLE bbs_reaction_app SET search_path TO bbs_reaction, public;
ALTER ROLE bbs_credit_app SET search_path TO bbs_credit, public;
ALTER ROLE bbs_notification_app SET search_path TO bbs_notification, public;
ALTER ROLE bbs_admin_app SET search_path TO bbs_admin, public;
ALTER ROLE bbs_config_app SET search_path TO bbs_config, public;
ALTER ROLE bbs_file_app SET search_path TO bbs_file, public;
ALTER ROLE bbs_audit_app SET search_path TO bbs_audit, public;
ALTER ROLE bbs_mall_app SET search_path TO bbs_mall, public;
ALTER ROLE bbs_chat_app SET search_path TO bbs_chat, public;
