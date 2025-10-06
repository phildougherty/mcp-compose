-- Registry Schema Verification Script
-- Run with: psql $POSTGRES_URL -f verify_registry_schema.sql

\echo '=== Checking for required tables ==='
SELECT
    tablename,
    CASE
        WHEN tablename IN ('marketplace_servers', 'marketplace_categories', 'user_installed_servers')
        THEN '✓ EXISTS'
        ELSE '✗ UNEXPECTED'
    END as status
FROM pg_tables
WHERE schemaname = 'public'
    AND tablename IN ('marketplace_servers', 'marketplace_categories', 'user_installed_servers', 'schema_migrations')
ORDER BY tablename;

\echo ''
\echo '=== Marketplace Categories Count ==='
SELECT COUNT(*) as category_count,
       'Expected: 8' as expected
FROM marketplace_categories;

\echo ''
\echo '=== Categories List ==='
SELECT name, display_name, sort_order
FROM marketplace_categories
ORDER BY sort_order;

\echo ''
\echo '=== Marketplace Servers Count ==='
SELECT COUNT(*) as server_count,
       'Expected: 10' as expected
FROM marketplace_servers;

\echo ''
\echo '=== Featured Servers ==='
SELECT name, display_name, category, downloads, rating
FROM marketplace_servers
WHERE featured = true
ORDER BY downloads DESC;

\echo ''
\echo '=== All Servers by Category ==='
SELECT category, COUNT(*) as count
FROM marketplace_servers
GROUP BY category
ORDER BY category;

\echo ''
\echo '=== Sample Server Details ==='
SELECT
    name,
    display_name,
    protocol,
    array_length(tags, 1) as tag_count,
    array_length(capabilities, 1) as capability_count,
    featured
FROM marketplace_servers
WHERE name = 'filesystem';

\echo ''
\echo '=== Installed Servers Count ==='
SELECT COUNT(*) as installed_count
FROM user_installed_servers;

\echo ''
\echo '=== Schema Migrations Applied ==='
SELECT filename, applied_at
FROM schema_migrations
ORDER BY applied_at;

\echo ''
\echo '=== Table Indexes ==='
SELECT
    schemaname,
    tablename,
    indexname
FROM pg_indexes
WHERE schemaname = 'public'
    AND tablename IN ('marketplace_servers', 'marketplace_categories', 'user_installed_servers')
ORDER BY tablename, indexname;

\echo ''
\echo '=== Verification Complete ==='
\echo 'Expected results:'
\echo '  - 3-4 tables (marketplace_servers, marketplace_categories, user_installed_servers, optionally schema_migrations)'
\echo '  - 8 categories'
\echo '  - 10 servers (6 featured)'
\echo '  - 6+ indexes'
\echo ''
