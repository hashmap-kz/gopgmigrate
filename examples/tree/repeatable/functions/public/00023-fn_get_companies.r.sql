drop function if exists public.fn_get_companies;
create function public.fn_get_companies()
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
        from companies c;
end
$fn$ language plpgsql;