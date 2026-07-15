// force timezone to UTC to allow tests to work regardless of local timezone
// generally used by snapshots, but can affect specific tests
process.env.TZ = 'UTC';

module.exports = {
  // Jest configuration provided by Grafana scaffolding
  ...require('./.config/jest.config'),
  // e2e.test.ts is a Playwright suite, not a Jest suite - keep it out of `yarn test`
  testPathIgnorePatterns: ['/node_modules/', 'e2e.test.ts'],
};
