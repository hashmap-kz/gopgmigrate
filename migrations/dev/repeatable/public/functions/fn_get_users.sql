drop function if exists public.fn_get_users;
create function public.fn_get_users()
    returns table
            (
                id    int,
                email varchar(250)
            )
as
$fn$
begin
    return query
        select c.record_id as id,
               c.email
        from mtest_users c;

end
$fn$ language plpgsql;
