alter table default.groups add column xdata text
;

-- alter table default.groups
-- update xdata = 'data-' || xname
-- where true
-- settings mutations_sync = 2
-- ;