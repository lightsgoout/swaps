create table if not exists swap (
    ulid text primary key,

    who text not null,
    token text not null,
    amount numeric not null,
    usd numeric not null,
    side boolean not null,

    constraint who_not_empty check(who <> ''),
    constraint token_not_empty check(token <> '')
);

create or replace function t_swap_to_outbox()
    returns trigger as
$$
begin
    insert into swap_outbox (ulid)
    values (NEW.ulid);
    return NEW;
end;
$$ language plpgsql volatile;

create or replace trigger t_swap_to_outbox before insert on swap for each row execute function t_swap_to_outbox();

create table if not exists swap_outbox(
    id bigint generated always as identity,
    ulid text
);
