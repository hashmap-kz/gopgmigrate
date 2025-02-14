alter table authn.roles add column xdata text;
update authn.roles set xdata = xname || '-data';
create index ix_roles_xdata on authn.roles (xdata);