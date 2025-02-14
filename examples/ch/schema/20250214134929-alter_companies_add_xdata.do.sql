alter table default.companies add column xdata text
;

-- alter table default.companies
-- update xdata = 'data-' || xname
-- where true
-- settings mutations_sync = 2
-- ;