-- +goose Up
-- +goose StatementBegin
create table short_urls (
    username varchar(20) not null,
    long_url varchar not null,
    short_url varchar,
    created_at timestamptz not null default now(),

    primary key (short_url),
    foreign key (username) references users (username) on delete cascade
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table short_urls;
-- +goose StatementEnd
