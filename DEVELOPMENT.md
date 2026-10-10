# Development

## Prerequisites

Node.js 24 (see `.nvmrc`) and Yarn 4. Yarn is pinned by the `packageManager`
field in `package.json` and provided by corepack, which ships with Node.js 24,
so you do not install Yarn yourself — just enable corepack once:

```bash
corepack enable
```

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
| `TRINO_IMPERSONATION_IDENTITY` | `login`/`email`: which Grafana user attribute to impersonate as |
| `TRINO_IMPERSONATION_USER` | User to impersonate |
| `TRINO_ROLES` | `catalog:role;catalog:role` pairs |
| `TRINO_CLIENT_TAGS` | Comma-separated client tags |
| `TRINO_TOKEN_URL` | OAuth2 token URL |
| `TRINO_CLIENT_ID` | OAuth2 client ID |
| `TRINO_CLIENT_SECRET` | OAuth2 client secret |
| `TRINO_TLS_SKIP_VERIFY` | `true`/`false` |
| `TRINO_KERBEROS_ENABLED` | `true`/`false`, requires an `https://` `TRINO_URL` |
| `TRINO_KERBEROS_PRINCIPAL` | Kerberos principal, without the realm |
| `TRINO_KERBEROS_REALM` | Realm of the principal |
| `TRINO_KERBEROS_CONFIG_PATH` | krb5 config, defaults to `/etc/krb5.conf` |
| `TRINO_KERBEROS_KEYTAB_PATH` | Keytab to log in with |
| `TRINO_KERBEROS_CREDENTIAL_CACHE_PATH` | Credential cache to use instead of a keytab |
| `TRINO_KERBEROS_REMOTE_SERVICE_NAME` | Service name of the coordinator, defaults to `trino` |
| `TRINO_KERBEROS_SERVICE_PRINCIPAL_PATTERN` | Defaults to `${SERVICE}@${HOST}` |
| `TRINO_KERBEROS_DISABLE_CANONICAL_HOSTNAME` | `true` to use the URL host in the service principal as is |

The Kerberos paths are read inside the Grafana container, which mounts this
repository at `/root/trino-datasource`. See [Kerberos](#kerberos) for the
bundled Kerberized Trino.

Restart the stack to pick up changes; Grafana re-reads provisioning files on
boot.

### Run a local build in a plain Grafana container

To load a local build into a standalone Grafana container, without the rest of
the dev stack, mount the `dist` directory and allow the unsigned plugin:

```bash
yarn build && mage -v
docker run -d -p 3000:3000 \
  -v "$(pwd)/dist:/var/lib/grafana/plugins/trino-datasource" \
  -e "GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=trino-datasource" \
  --name=grafana \
  grafana/grafana-oss
```

## Secure SOCKS proxy (PDC)

The dev stack always enables Grafana's secure SOCKS datasource proxy, so the
"Secure Socks Proxy" toggle shows up in the datasource settings. To actually
exercise it — the setup the `secure socks proxy (PDC)` e2e tests need — bring
the stack up with the `pdc` profile:

```bash
yarn build && mage -v
docker compose --profile pdc up --build
```

That adds two containers: `trino-private`, a second Trino instance on an
isolated `pdc-private` network with no route to/from the network Grafana runs
on, and `socks-proxy`, dual-homed onto both. `trino-private` is therefore
reachable *only* through the proxy, so a query against it succeeding proves the
toggle really routes traffic rather than just being accepted and ignored. The
paired negative-control test checks the same host fails with the toggle off.

Run the e2e suite against it with:

```bash
PDC_PRIVATE_TRINO_URL=http://trino-private:8080 yarn e2e
```

The two PDC tests are skipped when `PDC_PRIVATE_TRINO_URL` is unset, so a plain
`yarn e2e` against a default `yarn server` stack is unaffected. CI sets it in
the `End to end test` step.

## Kerberos

To exercise Kerberos authentication, bring the stack up with the `kerberos`
profile:

```bash
yarn build && mage -v
docker compose --profile kerberos up --build
```

That adds two containers: `kdc`, a Kerberos KDC for the `TRINO.TEST` realm
(the image Trino's own product tests use), and `trino-kerberos`, a Trino
instance that only accepts Kerberos over HTTPS on port 8443, with a self-signed
certificate. At startup, `test-data/kerberos/init.sh` creates the
coordinator's `trino/trino-kerberos` service principal, a `grafana` client
principal and the certificate in a volume that Grafana mounts at
`/etc/trino-kerberos`, next to `test-data/kerberos/krb5.conf` at
`/etc/krb5.conf`. Trino maps the `grafana@TRINO.TEST` principal to the
`grafana` user.

To try it in the UI, create a data source with the URL
`https://trino-kerberos:8443`, "Skip TLS Verify" on, and Kerberos enabled with
principal `grafana`, realm `TRINO.TEST`, config path `/etc/krb5.conf` and
keytab path `/etc/trino-kerberos/grafana.keytab`. Turn off "Use canonical
hostname": Docker's DNS resolves the coordinator's address back to
`trino-kerberos.<network>`, which has no service principal, so the service
principal has to be built from the URL host. To provision it instead, set
these in `.env`:

```bash
TRINO_URL=https://trino-kerberos:8443
TRINO_TLS_SKIP_VERIFY=true
TRINO_KERBEROS_ENABLED=true
TRINO_KERBEROS_PRINCIPAL=grafana
TRINO_KERBEROS_REALM=TRINO.TEST
TRINO_KERBEROS_CONFIG_PATH=/etc/krb5.conf
TRINO_KERBEROS_KEYTAB_PATH=/etc/trino-kerberos/grafana.keytab
TRINO_KERBEROS_DISABLE_CANONICAL_HOSTNAME=true
```

Run the e2e suite against it with:

```bash
KERBEROS_TRINO_URL=https://trino-kerberos:8443 yarn e2e
```

The two Kerberos tests are skipped when `KERBEROS_TRINO_URL` is unset. CI sets
it in the `End to end test` step.

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
