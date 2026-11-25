-- +goose Up
create schema auth;

create table auth.users (
  id            uuid         primary key,
  name          varchar(256) not null,
  username      varchar(32)  not null unique,
  password_hash varchar(256) not null,
  created_at    timestamptz  not null default now(),
  deleted_at    timestamptz
);

create index auth_users_deleted_at_idx on auth.users (deleted_at) where deleted_at is not null;

create table auth.sessions (
  id         uuid         primary key,
  user_id    uuid         not null references auth.users (id) on delete cascade,
  token      varchar(256) not null unique,
  csrf       varchar(256) not null unique,
  platform   varchar      not null,
  os         varchar      not null,
  expires_at timestamptz  not null,
  created_at timestamptz  not null default now(),
  deleted_at timestamptz
);

create index auth_sessions_user_id_idx on auth.sessions (user_id);
create index auth_sessions_expires_at_idx on auth.sessions (expires_at);
create index auth_sessions_deleted_at_idx on auth.sessions (deleted_at) where deleted_at is not null;


-- +goose Down
drop table auth.sessions;
drop table auth.users;
drop schema auth;
