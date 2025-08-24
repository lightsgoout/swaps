# How to launch

```
docker-compose build
docker-compose up
```

This page will be updated with websockets `http://127.0.0.1:8085/`

API is available at: http://127.0.0.1:8085/stats

Swaps will start generating after 10 seconds - this pause is required to wait for Postgres to come online.

Most knobs are configurable - see cmd flags.

# To whom it may concern

Three main components:

* cmd/producer - continuously generates random swaps and persists them in PG. By default, generates 1000 swaps per
  second.
* cmd/swapper-api - serves index.html (/), websocket API (/ws) and JSON api (/stats).
* cmd/swapper-worker - processes new swaps and pushes them into VM (VictoriaMetrics).

* All stats are calculated with VM.
* Swaps are durably stored in PG.
* Producer assigns each swap an ULID, which is used as primary key in Postgres, and it's timestamp part is used for VM.
* Swaps are transported to VM with the outbox pattern.
* Due to VM apparently not supporting idempotency, swaps could be inserted into VM multiple times, in case of network issues.
