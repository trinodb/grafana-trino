// force timezone to UTC to allow tests to work regardless of local timezone
// generally used by snapshots, but can affect specific tests
process.env.TZ = 'UTC';

const grafanaConfig = require('./.config/jest.config');
const swcTransform = grafanaConfig.transform['^.+\\.(t|j)sx?$'];

module.exports = {
  // Jest configuration provided by Grafana scaffolding
  ...grafanaConfig,
  // Since 3.0.2, the entry point of d3-interpolate, which @grafana/data
  // requires, is an .mjs file, which the scaffolded transform does not match.
  transform: { ...grafanaConfig.transform, '^.+\\.mjs$': swcTransform },
  // e2e.test.ts is a Playwright suite, not a Jest suite - keep it out of `yarn test`
  testPathIgnorePatterns: ['/node_modules/', 'e2e.test.ts'],
};
