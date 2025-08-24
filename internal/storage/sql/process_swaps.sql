select
    o.id,
    o.ulid,
    sw.who,
    sw.token,
    sw.amount,
    sw.usd,
    sw.side
from swap_outbox o
join swap sw on sw.ulid = o.ulid
for update of o
skip locked