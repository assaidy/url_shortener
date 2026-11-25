-- name: GetAliasLength :one
select length from shortening.alias_length;

-- name: IncrementAliasLength :one
-- Increment the length only if it hasn't changed since we last fetched it.
update shortening.alias_length
set length = case when length = $1 then length + 1 else length end
returning length;

-- name: InsertUrl :execrows
insert into shortening.urls (id, user_id, title, url, alias, expires_at, created_at)
values ($1, $2, $3, $4, $5, $6, $7)
on conflict (alias) do nothing;

-- name: GetUserUrls :many
select * from shortening.urls
where user_id = $1 and deleted_at is null and title ilike '%'|| sqlc.arg(search)::varchar || '%' and (
  -- The cursor is only present from the second page on. Each sort key resumes
  -- after its own column in its own direction; id is the final tiebreaker and
  -- is always descending (see the order by clause).
  sqlc.narg(last_id)::uuid is null
  or case sqlc.arg(sort)::text
    when 'title.asc' then
      title > sqlc.narg(last_title)::varchar
      or (title = sqlc.narg(last_title)::varchar and id < sqlc.narg(last_id)::uuid)
    when 'title.desc' then
      title < sqlc.narg(last_title)::varchar
      or (title = sqlc.narg(last_title)::varchar and id < sqlc.narg(last_id)::uuid)
    when 'url.asc' then
      url > sqlc.narg(last_url)::varchar
      or (url = sqlc.narg(last_url)::varchar and id < sqlc.narg(last_id)::uuid)
    when 'url.desc' then
      url < sqlc.narg(last_url)::varchar
      or (url = sqlc.narg(last_url)::varchar and id < sqlc.narg(last_id)::uuid)
    when 'alias.asc' then
      alias > sqlc.narg(last_alias)::varchar
      or (alias = sqlc.narg(last_alias)::varchar and id < sqlc.narg(last_id)::uuid)
    when 'alias.desc' then
      alias < sqlc.narg(last_alias)::varchar
      or (alias = sqlc.narg(last_alias)::varchar and id < sqlc.narg(last_id)::uuid)
    when 'created_at.asc' then
      created_at > sqlc.narg(last_created_at)::timestamptz
      or (created_at = sqlc.narg(last_created_at)::timestamptz and id < sqlc.narg(last_id)::uuid)
    when 'created_at.desc' then
      created_at < sqlc.narg(last_created_at)::timestamptz
      or (created_at = sqlc.narg(last_created_at)::timestamptz and id < sqlc.narg(last_id)::uuid)
  end
)
order by
    case when sqlc.arg(sort)::text = 'title.asc' then title end asc,
    case when sqlc.arg(sort)::text = 'title.desc' then title end desc,
    case when sqlc.arg(sort)::text = 'url.asc' then url end asc,
    case when sqlc.arg(sort)::text = 'url.desc' then url end desc,
    case when sqlc.arg(sort)::text = 'alias.asc' then alias end asc,
    case when sqlc.arg(sort)::text = 'alias.desc' then alias end desc,
    case when sqlc.arg(sort)::text = 'created_at.asc' then created_at end asc,
    case when sqlc.arg(sort)::text = 'created_at.desc' then created_at end desc,
    id desc
limit $2;

-- name: GetUserUrlById :one
select * from shortening.urls
where id = $1 and user_id = $2 and deleted_at is null;

-- name: UpdateUserUrl :one
update shortening.urls
set title = $3, expires_at = $4, is_active = $5
where id = $1 and user_id = $2 and deleted_at is null
returning *;

-- name: SoftDeleteUserUrl :execrows
update shortening.urls
set deleted_at = now()
where id = $1 and user_id = $2 and deleted_at is null;

-- name: SoftDeleteUserUrls :execrows
update shortening.urls
set deleted_at = now()
where user_id = $1 and deleted_at is null;

-- name: IncrementUrlClicks :exec
update shortening.urls
set clicks = clicks + 1
where id = $1;

-- name: GetUrlByAlias :one
select * from shortening.urls where alias = $1 and deleted_at is null;
