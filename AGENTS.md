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

## Build, test, validate

Build and test commands are in [DEVELOPMENT.md](DEVELOPMENT.md#build-and-test).
In short: `yarn build` (frontend), `mage -v` (backend), `yarn test:ci` / `go
test ./...`, `yarn lint`, `yarn typecheck`.

To validate a packaged build against the Grafana plugin validator:

```bash
name=$(jq -r '.id' src/plugin.json)
cp -a dist "$name"
zip -r "$name.zip" "$name"
docker run -it --rm -v $(pwd):/plugin grafana/plugin-validator-cli /app/bin/plugincheck2 -config config/default.yaml /plugin/$name.zip
```

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
