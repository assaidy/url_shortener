-- +goose Up
create schema shortening;

-- Shortened URLs are mutable only in a restricted sense: after creation, only
-- title, deleted_at, and the click counter may change. Every other column is
-- immutable, and soft-deleted rows keep their alias reserved (aliases are never
-- reused).
create table shortening.urls (
  id         uuid         primary key,
  user_id    uuid         not null,
  title      varchar(256) not null default '',
  url        varchar      not null,
  alias      varchar      not null unique,
  created_at timestamptz  not null default now(),
  expires_at timestamptz,
  is_active  bool         not null default true,
  clicks     bigint       not null default 0,
  deleted_at timestamptz
);

create index shortening_urls_user_id_idx on shortening.urls (user_id);
create index shortening_urls_deleted_at_idx on shortening.urls (deleted_at) where deleted_at is not null;

create extension if not exists pg_trgm;

-- Searchable text of a url. The fields are joined with a newline, which a
-- search term never contains, so a match can't span two fields.
create function shortening.urls_search_text(title varchar, url varchar)
returns text
language sql
immutable
as $$
  select title || E'\n' || url
$$;

create index shortening_urls_search_idx on shortening.urls
  using gin (shortening.urls_search_text(title, url) gin_trgm_ops)
  where deleted_at is null;

create table shortening.alias_length (
  length int not null
);

insert into shortening.alias_length (length) values (5);

-- +goose Down
drop table if exists shortening.alias_length;
drop index if exists shortening.shortening_urls_search_idx;
drop index if exists shortening.shortening_urls_deleted_at_idx;
drop index if exists shortening.shortening_urls_user_id_idx;
drop function if exists shortening.urls_search_text(varchar, varchar, varchar);
drop extension if exists pg_trgm;
drop table if exists shortening.urls;
drop schema if exists shortening;
