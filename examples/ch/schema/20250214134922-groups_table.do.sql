CREATE TABLE default.groups
(
    id         UUID     DEFAULT generateUUIDv4(),
    xname      String
) ENGINE = MergeTree()
ORDER BY id;