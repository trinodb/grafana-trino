# Trino Grafana Data Source Plugin

[![CI](https://github.com/trinodb/grafana-trino/actions/workflows/ci.yml/badge.svg)](https://github.com/trinodb/grafana-trino/actions/workflows/ci.yml)

The Trino datasource allows to query and visualize [Trino](https://trino.io/) data from within Grafana.

## Getting started

### Install the plugin

The plugin is published in the [Grafana plugin catalog](https://grafana.com/grafana/plugins/trino-datasource/).
Install it in one of these ways:

* In the Grafana UI, go to **Administration > Plugins and data > Plugins**, search for
  "Trino" and click **Install**.
* With the Grafana CLI, then restart Grafana:

  ```bash
  grafana cli plugins install trino-datasource
  ```

  Older Grafana installations provide the same command as `grafana-cli`.
* In Docker, let Grafana install it on startup:

  ```bash
  docker run -d -p 3000:3000 \
    -e "GF_PLUGINS_PREINSTALL=trino-datasource" \
    --name=grafana \
    grafana/grafana-oss
  ```

  Grafana versions that don't support `GF_PLUGINS_PREINSTALL` use
  `GF_INSTALL_PLUGINS=trino-datasource` instead; newer versions still accept it,
  but log a deprecation warning.

To run a locally built, unsigned copy of the plugin, see
[DEVELOPMENT.md](DEVELOPMENT.md).

### Add the data source

1. In Grafana, go to **Connections > Data sources** and click **Add new data source**.
2. Select **Trino**.
3. Set **URL** to the Trino coordinator, for example `http://trino.example.com:8080`.
4. Configure [authentication](#authentication), if your Trino cluster requires it.
5. Click **Save & test**.

Open **Explore**, or add a panel to a dashboard, select the Trino data source
and run a query, for example:

```sql
SELECT * FROM tpch.tiny.orders LIMIT 10
```

The `tpch` catalog is only available if it's configured in Trino.

## Authentication

Authentication is configured in the data source settings. The options below can
be combined, except where noted.

### Basic auth

Under **Auth**, enable **Basic auth** and set **User** and **Password**. Trino
only accepts passwords over HTTPS, unless it's configured to allow them over
HTTP.

The basic auth user is also the Trino session user, even without a password.
Without basic auth, the session user is `grafana`. When Trino authenticates a
request with a token or certificate and its principal differs from the session
user, Trino's access control must allow impersonating that user. The
impersonation options below change the session user.

### TLS

Under **Auth**:

* **TLS Client Auth** sends a client certificate and key, for Trino's
  certificate authentication.
* **With CA Cert** verifies the Trino server certificate against a custom CA.
* **Skip TLS Verify** disables server certificate verification.

### Access token

Set **Access token** in the **Trino** section to send a static bearer token, such
as a JWT, with every request. It can't be combined with the OAuth client
credentials flow.

### OAuth client credentials

In the **OAuth Trino Authentication** section, set **Token URL**, **Client id** and
**Client secret**. All three are required. The plugin gets an access token from
the token URL using the OAuth 2.0 client credentials flow, and sends it to Trino.

**Impersonation user**, if set, is sent as the Trino session user. Trino must
allow the token's principal to impersonate that user. When
[impersonation of the signed-in user](#impersonate-the-signed-in-user) is
enabled, the impersonation user only applies to anonymous users.

### Forward OAuth Identity

If Grafana users sign in with OAuth, enable **Forward OAuth Identity** under
**Auth** to send the signed-in user's OAuth access token to Trino as a bearer
token, instead of the configured access token. Trino must be configured to
accept tokens from the same identity provider. The forwarded token is not used
when the OAuth client credentials flow is configured, because the token from
that flow replaces it.

### Impersonate the signed-in user

Enable **Impersonate logged in user** in the **Trino** section to run queries as
the Grafana user, instead of the data source's user. **Impersonate as** selects
whether the user's **Login** or **Email** becomes the Trino session user; with
**Email**, queries from users without an email fail. Trino must allow the
authenticated user to impersonate other users.

Anonymous users are not impersonated, and run as the data source's user, or the
OAuth impersonation user if set.

## Other settings

### Roles

**Roles** in the **Trino** section sets authorization roles per catalog, as
`catalog:role` pairs separated by semicolons. Use `system` for system roles, for
example `system:admin;hive:analyst`.

### Client tags

**Client Tags** in the **Trino** section is a comma-separated list of tags sent
with every query, used for example to select a Trino
[resource group](https://trino.io/docs/current/admin/resource-groups.html).
Queries can add more tags in the query editor.

### Private Data Source Connect

When Grafana has the secure SOCKS proxy enabled, as in Grafana Cloud with
[Private Data Source Connect](https://grafana.com/docs/grafana-cloud/connect-externally-hosted/private-data-source-connect/),
enable **Secure Socks Proxy** to reach a Trino cluster in a private network.
The toggle is only shown when the proxy is enabled.

Custom HTTP headers are not supported, and the data source fails to load if any
are set.

## Features

* [Authentication](#authentication) with basic auth, TLS client certificates,
  access tokens, OAuth client credentials or the signed-in user's forwarded OAuth
  identity
* Raw SQL editor only, no query builder yet
* Macros
* Client tags support, used to identify resource groups. Tags can be set on the data source,
  and extended with additional tags in the query editor.
* `ARRAY`, `MAP` and `ROW` columns rendered as JSON.
* [Impersonation](#impersonate-the-signed-in-user) of the signed-in Grafana user, by login or email.

## Complex types

`ARRAY`, `MAP` and `ROW` values are returned as JSON fields, with `ROW` values
converted to objects keyed by field name. Unnamed fields, and fields whose
names collide after lower-casing, are keyed by position as `_col<N>`; rows with
no named fields at all stay positional arrays. In the table panel, hover a cell and click the eye icon to
open the value in a formatted, collapsible JSON viewer. Explore on Grafana 11.6
through 12.3 does not show the eye icon; dashboard table panels do.

Row field names are returned in lower case, so quoted mixed-case field names
such as `"Word Start"` appear as `word start`.

## Macros support

Plugin supports the following marcos:

* `$timeFrom($column)` - replaced with the lower boundary of the currently selected "Time Range" as a timestamp.
* `$timeTo($column)` - replaced with the upper boundary of the currently selected "Time Range" as a timestamp.
* `$timeGroup($column, $interval)` - replaced with an expression that rounds values of a column
  to the selected "Group by a time interval" value.
* `$dateFilter($column)` - replaced with a range condition for the currently selected "Time Range" as dates,
  on a column passed as the $column argument. Use it in queries or query variables
  as `...WHERE $dateFilter($column)...` or `...WHERE $dateFilter(created_at)....`.
* `$timeFilter($column)` - replaced with a range condition for the currently selected "Time Range" as timestamps,
  on a column passed as the $column argument.
* `$unixEpochFilter($column)` - replaced with a range condition for the currently selected "Time Range",
  on a column passed as the $column argument.
* `$parseTime` - parse a timestamp string using the default or specified format.

A description of macros is available by typing their names in Raw Editor

## Templating

### Using Variables in Queries

Template variable values are only quoted when the template variable is a `multi-value`.

If the variable is a multi-value variable then use the `IN` comparison operator
rather than `=` to match against multiple values.

Example with a template variable named hostname:

```sql
SELECT
  atimestamp as time,
  aint as value
FROM table
WHERE $__timeFilter(atimestamp) and hostname in($hostname)
ORDER BY atimestamp ASC
```

### Disabling quoting for multi-value variables

Grafana automatically creates a quoted, comma-separated string for multi-value variables.
For example: if `server01` and `server02` are selected then it will be formatted as:
`'server01', 'server02'`. To disable quoting, use the `csv` formatting option for variables:

```
${servers:csv}
```

Read more about variable formatting options in the [Variables](https://grafana.com/docs/grafana/latest/variables/#advanced-formatting-options) documentation.

# Contributing

If you have any idea for an improvement or found a bug do not hesitate to open an issue or submit a pull request.
We will appreciate any help from the community.

# Development

See [DEVELOPMENT.md](https://github.com/trinodb/grafana-trino/blob/main/DEVELOPMENT.md) for development instructions.

# License

Apache 2.0 License, please see [LICENSE](https://github.com/trinodb/grafana-trino/blob/main/LICENSE) for details.
