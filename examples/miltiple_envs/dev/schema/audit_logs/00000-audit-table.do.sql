create schema audit_logs;

set search_path to 'audit_logs';

create table audit_logs.mtest_audit
(
    id    serial primary key,
    email varchar(255) not null unique
);


