create table lookup.country (
    code char(2)  primary key,
    name text     not null
);

create table lookup.currency (
    code   char(3) primary key,
    name   text    not null,
    symbol text    not null
);

create table lookup.language (
    code char(5) primary key,
    name text    not null
);
