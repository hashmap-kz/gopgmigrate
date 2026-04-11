drop function if exists authn.fn_get_groups;
create function authn.fn_get_groups()
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
        from groups c;
end
$fn$ language plpgsql;