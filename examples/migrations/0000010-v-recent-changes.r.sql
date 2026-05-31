create or replace view audit.v_recent_changes as
select
    l.id,
    l.table_name,
    l.operation,
    l.row_id,
    l.changed_by,
    l.changed_at,
    l.old_data,
    l.new_data
from audit.log l
where l.changed_at >= now() - interval '7 days'
order by l.changed_at desc;
