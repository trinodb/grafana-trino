# Changelog

## Unreleased

* Add optional asynchronous query support, so long-running queries are polled over
  short requests instead of one long-lived one and are no longer cut short by proxy
  or load balancer timeouts

## 1.3.0

* Allow adding Trino client tags in the query editor, appended to the ones configured on the data source
* Render ARRAY, MAP and ROW columns as JSON
* Allow impersonating the Grafana user by email instead of login
* Upgrade the frontend toolchain to Node 24 and Yarn 4
* Update dependencies and CI tooling to address known vulnerabilities

## 1.2.0

* Add support for Grafana Private Data Source Connect (PDC)
* Keep HTTP clients and their authentication, TLS, and proxy configuration isolated between data sources
* Prevent data source configuration changes from closing connections in replacement instances
* Update dependencies and the Go toolchain to address known vulnerabilities

## 1.1.1

* Update dependencies to address Grafana plugin validation findings
* Restrict Trino and OAuth token endpoints to HTTP and HTTPS
* Publish releases with build provenance

## 1.1.0

* Add support for Trino roles
* Add support for client tags
* Keep OAuth2 tokens isolated between data source instances
* Respect configured CA certificates for Trino TLS connections
* Modernize the Grafana plugin SDK and tooling and require Grafana 10.4 or later

## 1.0.12

* Add support for OAuth2 client-credentials flow

## 1.0.11

* Don't overwrite Grafana variables when editing existing dashboard queries
* Don't cancel queries running for longer than 60 seconds

## 1.0.10

* Store access token securely
* Update dependencies

## 1.0.9

* Add support for access token (JWT) authentication

## 1.0.8

* Add support for OAuth
* Add support for annotations
* Use UTC timestamps in macroTimeFilter

## 1.0.7

* Add support for user impersonation

## 1.0.6

* Revert focus change actions from last version to fix running queries

## 1.0.5

* Don't execute query with every focus change
* Fix connection error handling

## 1.0.4

* Add query variable support
* Enable alerting

## 1.0.3

* Use the custom CA in the custom http client
* Only check for client certs when TLS options are present

## 1.0.2

### What's Changed

Add support for TLS client auth

## 1.0.1

Updated dependencies.

## 1.0.0

Initial release.
