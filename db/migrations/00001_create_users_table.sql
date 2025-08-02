-- +goose Up
-- +goose StatementBegin
create table users (
    username varchar(20),
    hashed_password varchar(100) not null,
    created_at timestamptz not null default now(), 

    primary key (username)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table users;
-- +goose StatementEnd
