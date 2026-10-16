# Agent instructions

This repository contains a Grafana data source plugin for Trino. See
[DEVELOPMENT.md](DEVELOPMENT.md) for the full build/test/release workflow;
this file covers the parts specific to working with an AI coding agent.

## Critical rules

- Do not change the plugin ID or plugin type in `src/plugin.json`.
- Any change to `src/plugin.json` requires restarting the Grafana server
  (`yarn server`) to take effect.
- Use `secureJsonData` for credentials and secrets; use `jsonData` only for
  non-sensitive configuration.
- Most of `.config/` (webpack, tsconfig, eslint, jest, docker base image) is
  generated and maintained by `@grafana/create-plugin` tooling — see
  `.config/README.md` before editing it directly. `.config/provisioning/`
  is this repo's own Grafana provisioning config for the `yarn server` dev
  stack, not tool-generated, and is fine to edit.
- Frontend builds go through webpack, backend builds through `mage` — both
  configured under `.config/`. See `.config/README.md` for how to extend
  either without editing the generated files in place.

## Grafana's plugin policy

Releases go to the Grafana plugin catalog, whose review enforces Grafana's
plugin best practices and rejects a release that breaks them, whatever CI
said. The rules below are the ones that bind this plugin, restated here so
they apply without fetching anything; the source is
https://grafana.com/developers/plugin-tools/key-concepts/best-practices, and
the catalog review follows that page, so check it when a review cites a rule
missing here and add the rule. Apply them to every change, not only before a
release.

Security, which the review checks first:

- Do not use the local filesystem. Plugins share one host, and a data source
  editor who can name a path makes the plugin use any file Grafana can read.
  No path settings, no reading or writing files, not even through a library:
  take file contents as settings instead and build in memory. The in-memory
  Kerberos client in `pkg/trino/driver/driver.go` is the pattern; 1.4.0 was
  rejected for taking the keytab, krb5.conf and credential cache as paths.
- Do not use environment variables, not even as a fallback a library
  provides, for the same reason.
- Store credentials and secrets (passwords, tokens, keys, keytabs) in
  `secureJsonData`, which is encrypted at rest; `jsonData` is not.
- Do not expose sensitive information: never log credentials, and keep
  logging sparse. Log errors at `error`, everything else at `debug`, never
  at `info`. No `console.log` in the frontend.
- Do not let user input become code the plugin runs, and do not execute
  commands in the backend.
- A credential must only go where it was configured for. The SDK's basic
  auth middleware adds the data source password to any request without an
  Authorization header, so a request to another host (an OAuth token URL)
  through the same client would carry it; reject such combinations in
  `validateKerberos`/`validateOAuth` and in the `ConfigEditor` alerts.
- Restrict the URLs the plugin connects to, to HTTP and HTTPS.

Behavior the review also expects of a data source backend:

- `grafanaDependency` in `src/plugin.json` names the earliest Grafana the
  plugin fully supports, and the e2e matrix in CI covers that range.
- The data source stays provisionable: every setting, including every
  secret, has a key in `.config/provisioning/datasources/trino.yaml`.
- Keep the health check meaningful: Save & test reaches Trino with the
  configured credentials and surfaces its error.
- Keep cached connections (sqlds keeps one `*sql.DB` per data source
  instance; do not open one per query) and keep queries concurrent.
- When a release changes what the plugin sends, such as a password that
  earlier versions dropped, say so plainly in the changelog and README and
  make the error users will hit point at the setting to change.

## Changelog

- Every user-visible change gets a line in `CHANGELOG.md` in the same pull
  request, under the heading of the next release, not only at release time.
  Keep the heading in step with `version` in `package.json`.
- Entries describe what the user sees, accurately: what the plugin now does
  and what the user has to do about it, such as reconfigure a data source or
  remove a setting. Verify a claim about earlier versions against their code
  (`git show v1.2.0:pkg/...`) before writing it; the catalog review caught a
  1.3.1 entry saying the password was sent over HTTP "again", which no
  release had ever done.
- Before a release, reconcile the section with `git log <last tag>..main` and
  the release checklist in [DEVELOPMENT.md](DEVELOPMENT.md#releases).

## Build, test, validate

Build and test commands are in [DEVELOPMENT.md](DEVELOPMENT.md#build-and-test).
In short: `yarn build` (frontend), `mage -v` (backend), `yarn test:ci` / `go
test ./...`, `yarn lint`, `yarn typecheck`.

Run the plugin validator before declaring a change done that touches the
backend, dependencies, `src/plugin.json`, `README.md` or packaging, the same
way CI does (`.github/workflows/ci.yml`, job `package`):

- `grafana/plugin-actions/package-plugin` builds and validates the archive
  with the validator version the release action pins.
- `npx -y @grafana/plugin-validator@latest -sourceCodeUri file://./ <archive>`
  runs the latest validator, which the catalog review uses and which can be
  ahead of the pinned one. Locally, build the archive as shown in
  [DEVELOPMENT.md](DEVELOPMENT.md#verifier) and run the validator on it. A
  finding from either validator blocks the catalog, so fix it rather than
  suppress it.

## Vulnerability scanners

CI runs three scanners, each pinned to a version in
`.github/workflows/ci.yml`. Run them at those exact versions, since a newer
or older scanner reports a different set of advisories than the one CI and
the catalog enforce:

- `google/osv-scanner-action` over `go.mod` and `yarn.lock`. Its
  `osv-scanner.toml` suppressions only apply to CI: the catalog scans with
  the latest validator, which ignores that file, so a suppressed advisory can
  still reject a release. Prefer updating the dependency; suppress only
  with a reason that holds for the catalog too, and expect to argue it there.
- `govulncheck` (`go run golang.org/x/vuln/cmd/govulncheck@<version> ./...`)
  for reachable Go vulnerabilities.
- `gosec` (`go run github.com/securego/gosec/v2/cmd/gosec@<version> -quiet
  ./...`) for the Go sources. A `#nosec` needs the rule ID and a reason.

Bump a scanner's pinned version deliberately, in its own commit, after
running it and dealing with what the new version reports.

## Writing e2e tests

E2E tests live in `src/e2e.test.ts` (matched by `playwright.config.ts`'s
`testMatch: 'e2e*.test.ts'`), not under a `tests/` directory. This repo does
**not** use `@grafana/plugin-e2e` — tests import `test`/`expect` directly
from `@playwright/test` and drive the UI with plain Playwright locators.

The plugin declares `grafanaDependency` (in `src/plugin.json`) covering
several Grafana major/minor versions, and CI runs the e2e suite against
every version in that range plus `nightly`. `@grafana/ui` is loaded from the
host Grafana at runtime, not bundled with this plugin, so its rendered
markup — accessible names, ARIA roles, testids — genuinely differs across
versions. When a selector needs a version-specific workaround, prefer
matching on visible text or scoping to the narrowest container over
assuming a specific role or a specific `@grafana/ui` internal, and leave a
comment explaining which version(s) need it and why. See the existing
helpers in `src/e2e.test.ts` (e.g. `selectFormat`, `commitQuery`) for the
established patterns.

Credentials the tests need, such as the Kerberos keytab the KDC container
creates, reach the test through environment variables and are entered in
the UI like a user would; nothing is mounted into the Grafana container.
