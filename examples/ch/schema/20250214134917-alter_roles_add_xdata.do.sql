alter table default.roles add column xdata text
;

-- alter table default.roles
-- update xdata = 'data-' || xname
-- where true
-- settings mutations_sync = 2
-- ;