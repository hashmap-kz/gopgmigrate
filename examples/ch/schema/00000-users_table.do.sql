CREATE TABLE default.users
(
    id         UUID     DEFAULT generateUUIDv4(),
    xname      String
) ENGINE = MergeTree()
ORDER BY id;