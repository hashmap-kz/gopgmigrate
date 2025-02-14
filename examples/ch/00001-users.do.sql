CREATE TABLE default.users
(
    id         UUID     DEFAULT generateUUIDv4(),
    name       String,
    age        UInt8,
    created_at DateTime DEFAULT now()
) ENGINE = MergeTree()
ORDER BY id;
