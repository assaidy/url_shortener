-- +goose Up
-- +goose StatementBegin
create table url_visits (
    short_url varchar not null,
    visitor_ip varchar(50) not null, -- IPv4/IPv6
    visited_at timestamptz not null,

    foreign key (short_url) references short_urls (short_url)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table url_visits;
-- +goose StatementEnd
