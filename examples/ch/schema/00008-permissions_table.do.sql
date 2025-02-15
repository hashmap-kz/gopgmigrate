CREATE TABLE default.permissions
(
    id         UUID     DEFAULT generateUUIDv4(),
    xname      String
) ENGINE = MergeTree()
ORDER BY id;