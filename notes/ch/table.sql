create table if not exists migrate_history
(
    id               Int64,
    mh_version       Int64  not null,
    mh_name          String not null,
    mh_hash          String not null,
    mh_applied_by    String                   default currentUser(),
    mh_applied_at    DateTime64(3, 'UTC')     default now64(3),
    mh_txid          Nullable(String)         default '',
    mh_iter_id       UUID not null,
    mh_version_check UInt64 MATERIALIZED      toUInt64(left(mh_name, 5)),
    constraint       check_filename           check mh_name REGEXP '^(\d{5})-(.*)(?:\.ntx)?\.(do|r)\.sql$',
    constraint       check_version_unsigned   check mh_version >= 0,
    constraint       check_version_match_name check mh_version_check = mh_version
)
    ENGINE = MergeTree()
        ORDER BY (mh_version)
;

insert into migrate_history (mh_version, mh_name, mh_hash, mh_iter_id, mh_applied_by, mh_applied_at)
values (1, '00001-t.do.sql', '1', generateUUIDv4(), currentUser(), now64(3))
;

alter table migrate_history
update
    mh_hash         = '2',
    mh_applied_at   = now64(3),
    mh_iter_id      = generateUUIDv4()
where
    mh_version = 1
    settings mutations_sync = 2
;

select
    id,
    mh_version,
    mh_name,
    mh_hash,
    mh_applied_by,
    mh_applied_at,
    mh_txid,
    mh_iter_id
from migrate_history
order by mh_version
;

delete from migrate_history
where mh_version = 1
;

select exists (select 1 from migrate_history where mh_version = 1)
;

































