drop schema if exists audit_logs cascade;
drop schema if exists authn cascade;

do
$$
    declare
        v_rec record;
        v_sql text;
    begin
        for v_rec in (select *
                      from information_schema.tables it
                      where it.table_schema = 'public'
                        and it.table_type = 'BASE TABLE')
        loop
            v_sql = format('drop table if exists public.%I cascade;', v_rec.table_name);
            raise notice '%', v_sql;
            execute v_sql;
        end loop;
    end
$$;
