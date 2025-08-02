-- +goose Up
-- +goose StatementBegin
create table short_url_length (
    length int,
    last_update timestamptz not null,

    primary key (length)
);
-- +goose StatementEnd

-- +goose StatementBegin
insert into short_url_length (length, last_update)
values (6, now());
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table short_url_length;
-- +goose StatementEnd
