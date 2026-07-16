import { test, expect, Page } from '@playwright/test';

const GRAFANA_CLIENT = 'grafana-client';
const EXPORT_DATA = 'Explore data';

async function login(page: Page) {
    await page.goto('http://localhost:3000/login');
    await page.getByTestId('data-testid Username input field').fill('admin');
    await page.getByTestId('data-testid Password input field').fill('admin');
    await page.getByTestId('data-testid Login button').click();
    await page.getByTestId('data-testid Skip change password button').click();
}

async function goToTrinoSettings(page: Page) {
    await page.goto('http://localhost:3000/connections/datasources/trino-datasource');
    await page.getByRole('button', {name: 'Add new data source'}).click();
}

async function setupDataSourceWithAccessToken(page: Page) {
    await page.getByTestId('data-testid Datasource HTTP settings url').fill('http://trino:8080');
    await page.locator('label[for="trino-settings-enable-impersonation"]').last().click();
    await page.locator('div').filter({hasText: /^Access token$/}).locator('input[type="password"]').fill('aaa');
    await page.getByTestId('data-testid Data source settings page Save and Test button').click();
}

async function setupDataSourceWithClientCredentials(page: Page, clientId: string) {
    await page.getByTestId('data-testid Datasource HTTP settings url').fill('http://trino:8080');
    await page.locator('div').filter({hasText: /^Token URL$/}).locator('input').fill('http://keycloak:8080/realms/trino-realm/protocol/openid-connect/token');
    await page.locator('div').filter({hasText: /^Client id$/}).locator('input').fill(clientId);
    await page.locator('div').filter({hasText: /^Client secret$/}).locator('input[type="password"]').fill('grafana-secret');
    await page.locator('div').filter({hasText: /^Impersonation user$/}).locator('input').fill('service-account-grafana-client');
    await page.getByTestId('data-testid Data source settings page Save and Test button').click();
}

async function setupDataSourceWithClientTags(page: Page, clientTags: string) {
    await page.getByTestId('data-testid Datasource HTTP settings url').fill('http://trino:8080');
    await page.locator('label[for="trino-settings-enable-impersonation"]').last().click();
    await page.locator('div').filter({hasText: /^Access token$/}).locator('input[type="password"]').fill('aaa');
    await page.locator('div').filter({hasText: /^Client Tags$/}).locator('input').fill(clientTags);
    await page.getByTestId('data-testid Data source settings page Save and Test button').click();
}

async function runQueryAndCheckResults(page: Page) {
    await page.getByLabel(EXPORT_DATA).click();
    await page.getByTestId('data-testid TimePicker Open Button').click();
    await page.getByTestId('data-testid Time Range from field').fill('1995-01-01');
    await page.getByTestId('data-testid Time Range to field').fill('1995-12-31');
    await page.getByTestId('data-testid TimePicker submit button').click();
    await page.getByRole('combobox', {name: 'Format as'}).click();
    await page.getByRole('option', {name: 'Table'}).click();
    await page.getByTestId('data-testid Code editor container').click();
    await page.getByTestId('data-testid RefreshPicker run button').click();
    await expect(page.getByRole('row', {name: /1995-01-\d\d .*:00:00 5703857 F/})).toBeVisible({timeout: 15000});
}

test('test with access token', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithAccessToken(page);
    await runQueryAndCheckResults(page);
});

test('test client credentials flow', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithClientCredentials(page, GRAFANA_CLIENT);
    await runQueryAndCheckResults(page);
});

test('test client credentials flow with wrong credentials', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithClientCredentials(page, "some-wrong-client");
    await expect(page.getByLabel(EXPORT_DATA)).toHaveCount(0);
});

test('test client credentials flow with configured access token', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await page.locator('div').filter({hasText: /^Access token$/}).locator('input[type="password"]').fill('aaa');
    await setupDataSourceWithClientCredentials(page, GRAFANA_CLIENT);
    await expect(page.getByLabel(EXPORT_DATA)).toHaveCount(0);
});

test('test with client tags', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithClientTags(page, 'tag1,tag2,tag3');
    await runQueryAndCheckResults(page);
});

test('test with roles', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithRoles(page, 'system:ALL;hive:admin');
    await runRoleQuery(page);
    await expect(page.getByRole('gridcell', {name: 'admin'})).toBeVisible();

});

test('test without role', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithRoles(page, '');
    await runRoleQuery(page);
    await expect(page.getByText(/Access Denied: Cannot show roles/)).toBeVisible();
});

async function setupDataSourceWithRoles(page: Page, roles: string) {
    await page.getByTestId('data-testid Datasource HTTP settings url').fill('http://trino:8080');
    await page.locator('div').filter({hasText: /^Roles$/}).locator('input').fill(roles);
    await page.getByTestId('data-testid Data source settings page Save and Test button').click();
}

async function runRoleQuery(page: Page) {
    await page.getByLabel(EXPORT_DATA).click();
    await page.getByRole('combobox', {name: 'Format as'}).click();
    await page.getByRole('option', {name: 'Table'}).click();
    await setQuery(page, 'SHOW ROLES FROM hive')
    await page.getByTestId('data-testid Code editor container').click();
    await page.getByTestId('data-testid RefreshPicker run button').click();
}

async function setQuery(page: Page, query: string) {
    await page.getByTestId('data-testid Code editor container').click({ clickCount: 4 });
    await page.keyboard.type(query);
}

test('test check health failure surfaces an error', async ({ page }) => {
    // driver.Open() rejects this combination synchronously (access token set
    // within the OAuth section, which is reserved for the client secret) -
    // a deterministic failure to prove Save & Test surfaces backend errors,
    // since trino-go-client doesn't implement database/sql's Pinger
    // interface, so an unreachable host alone wouldn't actually fail here.
    await login(page);
    await goToTrinoSettings(page);
    await page.getByTestId('data-testid Datasource HTTP settings url').fill('http://trino:8080');
    await page.locator('div').filter({hasText: /^Access token$/}).locator('input[type="password"]').fill('aaa');
    await setupDataSourceWithClientCredentials(page, GRAFANA_CLIENT);
    await expect(page.getByText(/access token must not be set within 'OAuth Trino Authentication' settings/)).toBeVisible();
});

test('test with time series format', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithAccessToken(page);
    await page.getByLabel(EXPORT_DATA).click();
    await page.getByTestId('data-testid TimePicker Open Button').click();
    await page.getByTestId('data-testid Time Range from field').fill('1995-01-01');
    await page.getByTestId('data-testid Time Range to field').fill('1995-12-31');
    await page.getByTestId('data-testid TimePicker submit button').click();
    await page.getByTestId('data-testid Code editor container').click();
    await page.getByTestId('data-testid RefreshPicker run button').click();
    await expect(page.getByRole('heading', {name: 'Graph'})).toBeVisible();
    await expect(page.getByText(/error querying the database/i)).toHaveCount(0);
});

test('test with logs format', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithAccessToken(page);
    await page.getByLabel(EXPORT_DATA).click();
    await page.getByTestId('data-testid TimePicker Open Button').click();
    await page.getByTestId('data-testid Time Range from field').fill('1995-01-01');
    await page.getByTestId('data-testid Time Range to field').fill('1995-12-31');
    await page.getByTestId('data-testid TimePicker submit button').click();
    await page.getByRole('combobox', {name: 'Format as'}).click();
    await page.getByRole('option', {name: 'Logs'}).click();
    await setQuery(page, "SELECT orderdate as time, orderstatus as level, 'order ' || cast(orderkey as varchar) as message FROM tpch.tiny.orders WHERE $__timeFilter(orderdate)");
    await page.getByTestId('data-testid Code editor container').click();
    await page.getByTestId('data-testid RefreshPicker run button').click();
    await expect(page.getByText('Logs volume')).toBeVisible();
    await expect(page.getByText(/error querying the database/i)).toHaveCount(0);
});

test('test template variable backed by trino query', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithAccessToken(page);

    await page.goto('http://localhost:3000/dashboard/new?editview=templating&editIndex=0');
    await page.getByRole('tab', {name: 'Variables'}).click();
    await page.getByRole('button', {name: 'Add variable'}).click();
    await page.getByTestId('data-testid Variable editor Form Name field').fill('orderstatus');
    // The Trino datasource is already preselected as the only configured
    // datasource. Falls back to StandardVariableSupport's generic query
    // textbox, since this plugin doesn't implement a custom variable query
    // editor.
    await page.getByRole('textbox', {name: 'Metric name or tags query'}).fill('SELECT DISTINCT orderstatus FROM tpch.tiny.orders');
    await page.getByRole('button', {name: 'Run query'}).click();
    await expect(page.getByText('Preview of values (3)')).toBeVisible();
    await expect(page.getByRole('row', {name: 'F F F'})).toBeVisible();
    await expect(page.getByRole('row', {name: 'O O O'})).toBeVisible();
    await expect(page.getByRole('row', {name: 'P P P'})).toBeVisible();
});
