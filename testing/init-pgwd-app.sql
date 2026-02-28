-- Create a non-superuser role for client containers.
-- Clients use this role so they only consume "normal" connection slots;
-- superuser_reserved_connections (default 3) stay free for admin (psql -U pgwd).
CREATE USER pgwd_app WITH PASSWORD 'pgwd' NOSUPERUSER;
GRANT CONNECT ON DATABASE pgwd TO pgwd_app;
