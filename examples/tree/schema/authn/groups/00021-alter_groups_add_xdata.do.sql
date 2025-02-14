alter table authn.groups add column xdata text;
update authn.groups set xdata = xname || '-data';
create index ix_groups_xdata on authn.groups (xdata);