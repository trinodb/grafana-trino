# Trino Grafana Data Source Plugin

[![Build](https://github.com/starburstdata/grafana-trino/workflows/CI/badge.svg)](https://github.com/grafana/grafana-datasource-backend/actions?query=workflow%3A%22CI%22)

The Trino datasource allows to query and visualize Trino data from within Grafana.

## Getting started

Drop this into Grafana's `plugins` directory. To run it locally without installing Grafana, run it in a Docker container using:

```bash
docker run -d -p 3000:3000 \
  -v "$(pwd):/var/lib/grafana/plugins/trino" \
  -e "GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=starburst-data-trino" \
  --name=grafana \
  grafana/grafana-oss
```

### Build

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
