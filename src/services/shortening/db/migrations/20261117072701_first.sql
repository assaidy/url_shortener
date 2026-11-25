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

create table shortening.alias_length (
  length int not null
);

insert into shortening.alias_length (length) values (5);

-- +goose Down
drop table shortening.alias_length;
drop table shortening.urls;
drop schema shortening;
