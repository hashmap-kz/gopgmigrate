create index concurrently if not exists users_email_lower_idx
    on app.users (lower(email));

create index concurrently if not exists users_country_language_idx
    on app.users (country_code, language_code);

create index concurrently if not exists sessions_user_expires_idx
    on app.sessions (user_id, expires_at);
