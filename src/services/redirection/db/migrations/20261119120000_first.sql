-- +goose Up
create schema redirection;

create table redirection.redirections (
  id         uuid        primary key,
  url_id     uuid        not null,
  user_id    uuid        not null,
  ip         varchar(45) not null,
  platform   varchar(32) not null,
  os         varchar(32) not null,
  created_at timestamptz not null default now()
);

create index idx_redirections_url_id on redirection.redirections (url_id);
create index idx_redirections_user_id on redirection.redirections (user_id);

-- +goose Down
drop table redirection.redirections;
drop schema redirection;
