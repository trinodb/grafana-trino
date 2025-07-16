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
    await page.getByTestId('data-testid Toggle menu').click();
    await page.getByRole('link', {name: 'Connections'}).click();
    await page.getByRole('link', {name: 'Trino'}).click();
    await page.locator('.css-1yhi3xa').click();
    await page.getByRole('button', {name: 'Add new data source'}).click();
}

async function setupDataSourceWithAccessToken(page: Page) {
    await page.getByTestId('data-testid Datasource HTTP settings url').fill('http://trino:8080');
    await page.locator('div').filter({hasText: /^Impersonate logged in user$/}).getByLabel('Toggle switch').click();
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
    await page.locator('div').filter({hasText: /^Impersonate logged in user$/}).getByLabel('Toggle switch').click();
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
    await page.locator('div').filter({hasText: /^Format asChoose$/}).locator('svg').click();
    await page.getByRole('option', {name: 'Table'}).click();
    await page.getByTestId('data-testid Code editor container').click();
    await page.getByTestId('data-testid RefreshPicker run button').click();
    await expect(page.getByTestId('data-testid table body')).toContainText(/.*1995-01-19 0.:00:005703857F.*/);
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
