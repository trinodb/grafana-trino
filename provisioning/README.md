# Provisioning

Grafana provisioning config for the `yarn server` dev stack (see root
`docker-compose.yaml`).

`datasources/trino.yaml` auto-configures a "Trino" datasource on startup, by
default pointed at the bundled `trino` service so the dev stack is
self-contained and ready to query out of the box (try
`SELECT * FROM tpch.tiny.orders LIMIT 10`).

To point at a different Trino instance instead (e.g. a real internal
cluster), copy `.env.example` (repo root) to `.env` and set `TRINO_URL` plus
whichever auth fields you need — these flow through `docker-compose.yaml`
into `datasources/trino.yaml` via Grafana's `$__env{...}` provisioning
expansion. Restart the stack to pick up changes; Grafana re-reads
provisioning files on boot.
