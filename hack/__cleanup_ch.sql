-- ' ON CLUSTER default'
SELECT concat('DROP TABLE IF EXISTS default.', name, ';')
FROM system.tables
WHERE database = 'default';
