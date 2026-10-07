# Trino Grafana Data Source Plugin

[![Build](https://github.com/trinodb/grafana-trino/workflows/CI/badge.svg)](https://github.com/grafana/grafana-datasource-backend/actions?query=workflow%3A%22CI%22)

The Trino datasource allows to query and visualize [Trino](https://trino.io/) data from within Grafana.

## Getting started

Drop this into Grafana's `plugins` directory. To run it locally without installing Grafana, run it in a Docker container using:

```bash
docker run -d -p 3000:3000 \
  -v "$(pwd):/var/lib/grafana/plugins/trino" \
  -e "GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=trino-datasource" \
  --name=grafana \
  grafana/grafana-oss
```


## Features

* Authentication:
  * HTTP Basic
  * TLS client authentication
  * Access token (JWT)
  * OAuth
* Raw SQL editor only, no query builder yet
* Macros
* Client tags support, used to identify resource groups. Tags can be set on the data source,
  and extended with additional tags in the query editor.
* Asynchronous queries, so long-running queries are not bound to a single HTTP request
* `ARRAY`, `MAP` and `ROW` columns rendered as JSON.
* Impersonation of the logged-in Grafana user, by login or email. This takes
  precedence over the OAuth "Impersonation user", which then only applies to
  anonymous users. Anonymous users are not impersonated and run as the data
  source's user, or the OAuth impersonation user if set.

## Asynchronous queries

By default a query is answered over one HTTP request, held open from the moment
the panel runs until the last row arrives. Any proxy or load balancer in front
of Grafana gets a say in how long that may take, and a long-running query is
often cut short by an idle timeout even though Trino was still working on it.

Enabling **Asynchronous queries** in the data source settings switches to a
polling flow: the plugin starts the query, immediately returns a handle, and the
browser polls for its status every few seconds until the results are ready. Each
request is short, so timeouts no longer apply, and a dropped connection no longer
throws away the work the cluster has already done. The data still arrives in one
piece at the end; the query does not get faster and results are not streamed in
progressively.

Alerting and expression queries always use the synchronous flow, because there is
no browser to poll on their behalf.

Two things are worth knowing before turning it on:

* Results are held in the plugin's memory between polls. Peak memory is the same
  as the synchronous flow, but it is held for longer, so consider setting
  Grafana's [`dataproxy.row_limit`](https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/#row_limit)
  if you have not already.
* If you run more than one Grafana instance behind a load balancer, enable
  session affinity (sticky sessions). A poll routed to a different instance
  cannot see the query, because the connection to Trino and the rows read so far
  live in the instance that started it. Polls that land on the wrong instance
  fail with an error saying so.

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
