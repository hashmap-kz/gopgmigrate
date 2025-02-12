VACUUM FULL public.mtest_roles;

-- this fail, because another statement is executed inside transaction
-- to handle this, we should split statements, that makes implementation A LOT complicated
--
-- ALTER SYSTEM SET work_mem = '64MB';
