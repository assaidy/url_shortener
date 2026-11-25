-- name: InsertRedirections :exec
insert into redirection.redirections (id, url_id, user_id, ip, platform, os, created_at)
select id, url_id, user_id, ip, platform, os, created_at
from jsonb_to_recordset(sqlc.arg(redirections)::jsonb) as x(
  id         uuid,
  url_id     uuid,
  user_id    uuid,
  ip         varchar(45),
  platform   varchar(32),
  os         varchar(32),
  created_at timestamptz
);

-- name: GetRedirectionById :one
select * from redirection.redirections where id = $1 and user_id = $2;

-- name: GetUserRedirections :many
select * from redirection.redirections
where user_id = $1 and (
  -- The cursor is only present from the second page on. Each sort key resumes
  -- after its own column in its own direction; id is the final tiebreaker and
  -- is always descending (see the order by clause).
  sqlc.narg(last_redirection_id)::uuid is null
  or case sqlc.arg(sort)::text
    when 'created_at.asc' then
      created_at > sqlc.narg(last_created_at)::timestamptz
      or (created_at = sqlc.narg(last_created_at)::timestamptz and id < sqlc.narg(last_redirection_id)::uuid)
    when 'created_at.desc' then
      created_at < sqlc.narg(last_created_at)::timestamptz
      or (created_at = sqlc.narg(last_created_at)::timestamptz and id < sqlc.narg(last_redirection_id)::uuid)
    when 'os.asc' then
      os > sqlc.narg(last_os)::varchar
      or (os = sqlc.narg(last_os)::varchar and id < sqlc.narg(last_redirection_id)::uuid)
    when 'os.desc' then
      os < sqlc.narg(last_os)::varchar
      or (os = sqlc.narg(last_os)::varchar and id < sqlc.narg(last_redirection_id)::uuid)
    when 'platform.asc' then
      platform > sqlc.narg(last_platform)::varchar
      or (platform = sqlc.narg(last_platform)::varchar and id < sqlc.narg(last_redirection_id)::uuid)
    when 'platform.desc' then
      platform < sqlc.narg(last_platform)::varchar
      or (platform = sqlc.narg(last_platform)::varchar and id < sqlc.narg(last_redirection_id)::uuid)
  end
)
order by
  case when sqlc.arg(sort)::text = 'created_at.asc' then created_at end asc,
  case when sqlc.arg(sort)::text = 'created_at.desc' then created_at end desc,
  case when sqlc.arg(sort)::text = 'os.asc' then os end asc,
  case when sqlc.arg(sort)::text = 'os.desc' then os end desc,
  case when sqlc.arg(sort)::text = 'platform.asc' then platform end asc,
  case when sqlc.arg(sort)::text = 'platform.desc' then platform end desc,
  id desc
limit $2;

-- name: GetUrlRedirections :many
select * from redirection.redirections
where url_id = $1 and (
  -- See GetUserRedirections for the keyset logic.
  sqlc.narg(last_redirection_id)::uuid is null
  or case sqlc.arg(sort)::text
    when 'created_at.asc' then
      created_at > sqlc.narg(last_created_at)::timestamptz
      or (created_at = sqlc.narg(last_created_at)::timestamptz and id < sqlc.narg(last_redirection_id)::uuid)
    when 'created_at.desc' then
      created_at < sqlc.narg(last_created_at)::timestamptz
      or (created_at = sqlc.narg(last_created_at)::timestamptz and id < sqlc.narg(last_redirection_id)::uuid)
    when 'os.asc' then
      os > sqlc.narg(last_os)::varchar
      or (os = sqlc.narg(last_os)::varchar and id < sqlc.narg(last_redirection_id)::uuid)
    when 'os.desc' then
      os < sqlc.narg(last_os)::varchar
      or (os = sqlc.narg(last_os)::varchar and id < sqlc.narg(last_redirection_id)::uuid)
    when 'platform.asc' then
      platform > sqlc.narg(last_platform)::varchar
      or (platform = sqlc.narg(last_platform)::varchar and id < sqlc.narg(last_redirection_id)::uuid)
    when 'platform.desc' then
      platform < sqlc.narg(last_platform)::varchar
      or (platform = sqlc.narg(last_platform)::varchar and id < sqlc.narg(last_redirection_id)::uuid)
  end
)
order by
  case when sqlc.arg(sort)::text = 'created_at.asc' then created_at end asc,
  case when sqlc.arg(sort)::text = 'created_at.desc' then created_at end desc,
  case when sqlc.arg(sort)::text = 'os.asc' then os end asc,
  case when sqlc.arg(sort)::text = 'os.desc' then os end desc,
  case when sqlc.arg(sort)::text = 'platform.asc' then platform end asc,
  case when sqlc.arg(sort)::text = 'platform.desc' then platform end desc,
  id desc
limit $2;
