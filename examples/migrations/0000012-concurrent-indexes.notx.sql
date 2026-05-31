create index concurrently if not exists idx_user_preferences_user_id
    on app.user_preferences (user_id);

create index concurrently if not exists idx_notifications_user_id
    on app.notifications (user_id);

create index concurrently if not exists idx_notifications_sent_at
    on app.notifications (sent_at)
    where sent_at is null;
