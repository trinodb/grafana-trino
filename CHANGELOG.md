# Changelog

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
