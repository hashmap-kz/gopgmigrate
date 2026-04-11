drop function if exists authn.fn_get_permissions;
create function authn.fn_get_permissions()
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
        from permissions c;
end
$fn$ language plpgsql;