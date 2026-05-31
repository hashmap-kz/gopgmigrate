-- CREATE OR REPLACE VIEW cannot remove or reorder columns.
-- DROP + CREATE inside the repeatable transaction is the safe alternative.
drop view if exists app.v_user_summary;

create view app.v_user_summary as
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
    count(distinct s.id)  as active_sessions,
    count(distinct p.key) as preference_count,
    u.created_at
from app.users u
left join lookup.country       c on c.code = u.country_code
left join lookup.language      l on l.code = u.language_code
left join app.sessions         s on s.user_id = u.id
                                and s.expires_at > now()
left join app.user_preferences p on p.user_id = u.id
group by
    u.id, u.email, u.username, u.display_name,
    u.avatar_url, u.bio, u.is_active,
    c.name, l.name, u.created_at;
