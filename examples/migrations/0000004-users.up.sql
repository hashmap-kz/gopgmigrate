create table app.users (
    id            bigserial   primary key,
    email         text        not null unique,
    username      text        not null unique,
    display_name  text,
    avatar_url    text,
    bio           text,
    is_active     boolean     not null default true,
    country_code  char(2)     references lookup.country(code),
    language_code char(5)     references lookup.language(code),
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);
