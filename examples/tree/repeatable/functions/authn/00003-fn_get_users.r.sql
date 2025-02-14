drop function if exists authn.fn_get_users;
create function authn.fn_get_users()
    returns table
            (
                id    int,
                xname text
            )
as
$fn$
begin
    return query
        select c.id   as id,
               c.xname as xname
        from users c;
end
$fn$ language plpgsql;