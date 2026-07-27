# Development

## Build and test

1. Install dependencies

   ```bash
   yarn install
   ```

2. Build plugin in development mode or run in watch mode

   ```bash
   yarn dev
   ```

   or

   ```bash
   yarn watch
   ```

3. Build plugin in production mode

   ```bash
   yarn build
   ```

4. Build backend plugin binaries for Linux, Windows and Darwin:

   ```bash
   mage -v
   ```

5. Sign the plugin for private deployments

   ```bash
   yarn sign --rootUrls http://localhost:3000
   ```

## Run the dev stack

`yarn server` starts Grafana with this plugin loaded via Docker Compose. It's
self-contained by default — it also starts a bundled Trino instance and
auto-provisions a "Trino" datasource pointed at it (see
`.config/provisioning/datasources/trino.yaml`), so you can open
http://localhost:3000 and run a query immediately, e.g.
`SELECT * FROM tpch.tiny.orders LIMIT 10`.

```bash
yarn build && mage -v
yarn server
```

To point at a different Trino instance instead (e.g. a real internal
cluster), copy `.env.example` to `.env` and set `TRINO_URL` plus whichever
auth fields you need — these flow through `docker-compose.yaml` into
`.config/provisioning/datasources/trino.yaml` via Grafana's `$__env{...}`
provisioning expansion:

| Variable | Description |
| --- | --- |
| `TRINO_URL` | Trino server URL, e.g. `http://trino:8080` |
| `TRINO_BASIC_AUTH_ENABLED` | `true`/`false` |
| `TRINO_BASIC_AUTH_USER` | Basic auth username |
| `TRINO_BASIC_AUTH_PASSWORD` | Basic auth password |
| `TRINO_ACCESS_TOKEN` | Bearer access token |
| `TRINO_ENABLE_IMPERSONATION` | `true`/`false` |
| `TRINO_IMPERSONATION_USER` | User to impersonate |
| `TRINO_ROLES` | `catalog:role;catalog:role` pairs |
| `TRINO_CLIENT_TAGS` | Comma-separated client tags |
| `TRINO_TOKEN_URL` | OAuth2 token URL |
| `TRINO_CLIENT_ID` | OAuth2 client ID |
| `TRINO_CLIENT_SECRET` | OAuth2 client secret |
| `TRINO_TLS_SKIP_VERIFY` | `true`/`false` |

Restart the stack to pick up changes; Grafana re-reads provisioning files on
boot.

## Verifier

```bash
name=$(jq -r '.id' src/plugin.json)
cp -a dist "$name"
zip -r "$name.zip" "$name"
docker run -it --rm -v $(pwd):/plugin grafana/plugin-validator-cli /app/bin/plugincheck2 -config config/default.yaml /plugin/$name.zip
```

## Releases

1. Update the [CHANGELOG.md](CHANGELOG.md)
1. Update the version in the [package.json](package.json)
1. Commit the changes and create a `vX.Y` tag matching contents of the `package.json` file.

The release workflow will be triggered when pushing the tag and will create the GitHub release with the artifacts and release notes.
