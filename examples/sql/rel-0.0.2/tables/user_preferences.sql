create table app.user_preferences (
    id         bigint generated always as identity primary key,
    user_id    bigint      not null references app.users (id) on delete cascade,
    key        text        not null,
    value      text        not null,
    created_at timestamptz not null default now(),
    unique (user_id, key)
);
