CREATE TABLE default.roles
(
    id         UUID     DEFAULT generateUUIDv4(),
    xname      String
) ENGINE = MergeTree()
ORDER BY id;