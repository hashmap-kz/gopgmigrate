create index concurrently if not exists users_email_lower_idx
    on app.users (lower(email));

create index concurrently if not exists users_country_language_idx
    on app.users (country_code, language_code);

create index concurrently if not exists sessions_user_expires_idx
    on app.sessions (user_id, expires_at);

create index concurrently if not exists idx_user_preferences_user_id
    on app.user_preferences (user_id);

create index concurrently if not exists idx_notifications_user_id
    on app.notifications (user_id);

create index concurrently if not exists idx_notifications_sent_at
    on app.notifications (sent_at)
    where sent_at is null;
