-- name: CheckShortUrl :one
select exists (select 1 from short_urls where short_url = $1 for update);

-- name: InsertShortUrl :exec
insert into short_urls (username, long_url, short_url)
values ($1, $2, $3);

-- name: GetShortUrlLength :one
select length from short_url_length for update;

-- name: IncrementShortUrlLength :one
update short_url_length 
set 
    length = length + 1,
    last_update = now()
returning length;
