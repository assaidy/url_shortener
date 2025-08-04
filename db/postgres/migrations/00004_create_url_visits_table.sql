-- +goose Up
-- +goose StatementBegin
create table url_visits (
    short_url varchar not null,
    visitor_ip varchar(50) not null, -- IPv4/IPv6
    visited_at timestamp not null,

    foreign key (short_url) references short_urls (short_url) on delete cascade
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table url_visits;
-- +goose StatementEnd
