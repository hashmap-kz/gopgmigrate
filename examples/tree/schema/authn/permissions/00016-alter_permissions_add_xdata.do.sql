alter table authn.permissions add column xdata text;
update authn.permissions set xdata = xname || '-data';
create index ix_permissions_xdata on authn.permissions (xdata);