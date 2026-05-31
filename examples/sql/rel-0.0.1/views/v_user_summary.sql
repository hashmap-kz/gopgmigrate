create or replace view app.v_user_summary as
select
    u.id,
    u.email,
    u.username,
    u.display_name,
    u.avatar_url,
    u.bio,
    u.is_active,
    c.name          as country,
    l.name          as language,
    count(s.id)     as active_sessions,
    u.created_at
from app.users u
left join lookup.country  c on c.code = u.country_code
left join lookup.language l on l.code = u.language_code
left join app.sessions    s on s.user_id = u.id
                            and s.expires_at > now()
group by
    u.id, u.email, u.username, u.display_name,
    u.avatar_url, u.bio, u.is_active,
    c.name, l.name, u.created_at;
