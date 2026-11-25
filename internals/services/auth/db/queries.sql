-- name: InsertUser :execrows
insert into auth.users (id, name, username, password_hash)
values ($1, $2, $3, $4)
on conflict (username) do nothing;

-- name: CheckUserId :one
select exists (select 1 from auth.users where id = $1 and deleted_at is null for update);

-- name: GetUserByUsername :one
select * from auth.users where username = $1 and deleted_at is null for update;

-- name: GetUserById :one
select * from auth.users where id = $1 and deleted_at is null for update;

-- name: UpdateUsername :one
with updated_rows as (
  update auth.users
  set username = $1
  where id = $2
    and deleted_at is null
    and not exists (select 1 from auth.users u where u.username = $1 and u.id != $2)
  returning 1
)
select
  (select count(*) from updated_rows) != 0 as updated,
  exists (select 1 from auth.users u where u.id = $2 and u.deleted_at is null) as user_exists;

-- name: UpdatePasswordHash :exec
update auth.users set password_hash = $1 where id = $2;

-- name: SoftDeleteUser :execrows
update auth.users set deleted_at = now() where id = $1 and deleted_at is null;

-- name: InsertSession :exec
insert into auth.sessions (id, user_id, token, csrf, expires_at, platform, os)
values ($1, $2, $3, $4, $5, $6, $7);

-- name: GetSessionById :one
select * from auth.sessions where id = $1 and deleted_at is null;

-- name: DeleteUserSession :execrows
update auth.sessions set deleted_at = now()
where id = $1 and user_id = $2 and deleted_at is null;

-- name: DeleteAllUserSessions :exec
update auth.sessions set deleted_at = now()
where user_id = $1 and deleted_at is null;

-- name: DeleteAllUserSessionsExceptCurrent :exec
update auth.sessions set deleted_at = now()
where user_id = $1 and id != sqlc.arg(current) and deleted_at is null;

-- name: BatchDeleteExpiredOrSoftDeletedSessions :exec
do $$
declare
  rows_deleted int;
begin
  loop
    delete from auth.sessions
    where ctid in (
      select ctid
      from auth.sessions
      where deleted_at is not null or expires_at <= now()
      limit 1000
    );
    get diagnostics rows_deleted = row_count;
    exit when rows_deleted = 0;
  end loop;
end
$$;
