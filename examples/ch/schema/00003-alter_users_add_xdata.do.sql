alter table default.users add column xdata text
;

alter table default.users
update xdata = 'data-' || xname
where true
settings mutations_sync = 2
;