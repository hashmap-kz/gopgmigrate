create table app.sessions (
    id         uuid        primary key default gen_random_uuid(),
    user_id    bigint      not null references app.users(id) on delete cascade,
    ip_address inet,
    user_agent text,
    created_at timestamptz not null default now(),
    expires_at timestamptz not null
);
