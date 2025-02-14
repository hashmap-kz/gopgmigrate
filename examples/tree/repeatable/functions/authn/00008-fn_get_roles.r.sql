drop function if exists authn.fn_get_roles;
create function authn.fn_get_roles()
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
        from roles c;
end
$fn$ language plpgsql;