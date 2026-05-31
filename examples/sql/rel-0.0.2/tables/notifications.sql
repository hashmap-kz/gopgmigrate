create table app.notifications (
    id          bigint generated always as identity primary key,
    user_id     bigint      not null references app.users (id) on delete cascade,
    preference_id bigint    not null references app.user_preferences (id),
    channel     text        not null check (channel in ('email', 'push', 'sms')),
    payload     jsonb       not null default '{}',
    sent_at     timestamptz,
    created_at  timestamptz not null default now()
);
