alter table authn.users add column xdata text;
update authn.users set xdata = xname || '-data';
create index ix_users_xdata on authn.users (xdata);