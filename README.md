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
