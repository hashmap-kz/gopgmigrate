drop table if exists migrate_history;
create table if not exists migrate_history
(
    mh_version       Int64  not null,
    mh_name          String not null,
    mh_hash          String not null,
    mh_applied_by    String               default currentUser(),
    mh_applied_at    DateTime64(3, 'UTC') default now64(3),
    mh_version_check UInt64 MATERIALIZED toUInt64(left(mh_name, 5)),
    constraint       check_filename           check mh_name REGEXP '^(\d{5})-([a-zA-Z0-9_.-]+)\.(do|dontx|r|rntx)\.sql$',
    constraint       check_version_unsigned   check mh_version >= 0,
    constraint       check_version_match_name check mh_version_check = mh_version
)
ENGINE = MergeTree()
ORDER BY (mh_version);

INSERT INTO migrate_history (mh_version, mh_name, mh_hash, mh_applied_by, mh_applied_at)
VALUES (12345, '12345-migration.do.sql', '0-new_hash_value', currentUser(), now64(3));

ALTER TABLE migrate_history
UPDATE
    mh_hash = '5-new_hash_value',
    mh_applied_at = now64(3)
WHERE
    mh_version = 12345
    SETTINGS mutations_sync = 2
;

select *
from migrate_history mh;
