drop function if exists fn_get_roles;
create function fn_get_roles()
    returns table
            (
                id        int,
                role_name varchar(250)
            )
as
$fn$
begin
    return query
        select c.record_id as id,
               c.role_name
        from mtest_roles c;
end
$fn$ language plpgsql;
