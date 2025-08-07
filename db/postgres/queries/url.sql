-- name: CheckShortUrl :one
select exists (select 1 from short_urls where short_url = $1 for update);

-- name: InsertShortUrl :exec
insert into short_urls (username, long_url, short_url)
values ($1, $2, $3);

-- name: GetLongUrl :one
select long_url from short_urls where short_url = $1;

-- name: GetShortUrlLength :one
select length from short_url_length for update;

-- name: IncrementShortUrlLength :one
update short_url_length 
set 
    length = length + 1,
    last_update = now()
returning length;

-- name: InsertUrlVisits :exec
with visits_data as (
    select jsonb_array_elements(@json_visits::jsonb) as v
)
insert into url_visits (short_url, visitor_ip, visited_at) 
select 
    v ->> 'shortUrl',
    v ->> 'visitorIp',
    (v ->> 'visitedAt')::timestamp
from visits_data;
