alter table public.companies add column xdata text;
update public.companies set xdata = xname || '-data';
create index ix_companies_xdata on public.companies (xdata);