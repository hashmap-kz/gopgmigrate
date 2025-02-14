alter table default.permissions add column xdata text
;

-- alter table default.permissions
-- update xdata = 'data-' || xname
-- where true
-- settings mutations_sync = 2
-- ;