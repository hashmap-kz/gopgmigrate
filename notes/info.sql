with max_lengths as (select max(length(mh_mode))                                         as max_len_mode,
                            max(length(mh_name))                                         as max_len_name,
                            max(length(to_char(mh_applied_at, 'YYYY-MM-DD HH24:MI:SS'))) as max_len_ts
                     from public.migrate_history),
     formatted_data as (select mh_mode,
                               mh_name,
                               to_char(mh_applied_at, 'YYYY-MM-DD HH24:MI:SS') as ts
                        from public.migrate_history
                        order by mh_mode, mh_name),
     formatted_rows as (select format(
                                       e'%s | %s | %s',
                                       rpad(mh_mode, max_len_mode, ' '),
                                       rpad(mh_name, max_len_name, ' '),
                                       rpad(ts, max_len_ts, ' ')
                               ) as line
                        from formatted_data,
                             max_lengths),
     header as (select format(
                               e'%s | %s | %s',
                               rpad('mode', max_len_mode, ' '),
                               rpad('name', max_len_name, ' '),
                               rpad('applied-at', max_len_ts, ' ')
                       ) as line
                from max_lengths),
     separator as (select repeat('-', (max_len_mode + max_len_name + max_len_ts + 6)) as line
                   from max_lengths)
select string_agg(line, e'\n') as pretty_table
from (select *
      from header
      union all
      select *
      from separator
      union all
      select *
      from formatted_rows) formatted_output;
