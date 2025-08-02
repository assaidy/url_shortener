-- name: InsertUser :execrows
insert into users (username, hashed_password)
values ($1, $2)
on conflict (username) do nothing;

-- name: GetUserByUsername :one
select * from users where username = $1;

-- name: DeleteUserByUsername :execrows
delete from users where username = $1;

-- name: CheckUsername :one
select exists (select 1 from users where username = $1 for update);
