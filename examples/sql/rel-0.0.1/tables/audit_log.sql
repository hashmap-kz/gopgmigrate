create table audit.log (
    id         bigserial   primary key,
    table_name text        not null,
    operation  text        not null check (operation in ('INSERT', 'UPDATE', 'DELETE')),
    row_id     bigint,
    changed_by name        not null default session_user,
    changed_at timestamptz not null default now(),
    old_data   jsonb,
    new_data   jsonb
);

create index on audit.log (table_name, changed_at desc);
create index on audit.log (changed_at desc);
