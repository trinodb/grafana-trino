import { test, expect, Page } from '@playwright/test';

const GRAFANA_CLIENT = 'grafana-client';
const EXPLORE_DATA = 'Explore';

async function login(page: Page) {
    await page.goto('http://localhost:3000/login');
    await page.getByLabel('Username input field').fill('admin');
    await page.getByLabel('Password input field').fill('admin');
    await page.getByLabel('Login button').click();
    await page.getByLabel('Skip change password button').click();
}

async function goToTrinoSettings(page: Page) {
    await page.getByLabel('Toggle menu').click();
    await page.getByRole('link', {name: 'Connections'}).click();
    await page.getByRole('link', {name: 'Trino'}).click();
    await page.getByText('Create a Trino data source').click();
}

async function setupDataSourceWithAccessToken(page: Page) {
    await page.getByLabel('Datasource HTTP settings url').fill('http://trino:8080');
    await page.locator('div').filter({hasText: /^Impersonate logged in userAccess token$/}).getByLabel('Toggle switch').click();
    await page.locator('div').filter({hasText: /^Access token$/}).locator('input[type="password"]').fill('aaa');
    await page.getByLabel('Data source settings page Save and Test button').click();
}

async function setupDataSourceWithClientCredentials(page: Page, clientId: string) {
    await page.getByLabel('Datasource HTTP settings url').fill('http://trino:8080');
    await page.locator('div').filter({hasText: /^Token URL$/}).locator('input').fill('http://keycloak:8080/realms/trino-realm/protocol/openid-connect/token');
    await page.locator('div').filter({hasText: /^Client id$/}).locator('input').fill(clientId);
    await page.locator('div').filter({hasText: /^Client secret$/}).locator('input[type="password"]').fill('grafana-secret');
    await page.locator('div').filter({hasText: /^Impersonation user$/}).locator('input').fill('service-account-grafana-client');
    await page.getByLabel('Data source settings page Save and Test button').click();
}

async function runQueryAndCheckResults(page: Page) {
    await page.getByText(EXPLORE_DATA).click();
    await page.getByTestId('data-testid TimePicker Open Button').click();
    await page.getByLabel('Time Range from field').fill('1995-01-01');
    await page.getByLabel('Time Range to field').fill('1995-12-31');
    await page.getByTestId('data-testid TimePicker submit button').click();
    await page.getByLabel('Format as').click();
    await page.getByText('Table', { exact: true }).click();
    await page.getByTestId('data-testid RefreshPicker run button').click();
    await expect(page.getByTestId('data-testid table body')).toContainText(/.*1995-01-19 0.:00:00.*/);
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
    await expect(page.getByText(EXPLORE_DATA)).toHaveCount(0);
});

test('test client credentials flow with configured access token', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await page.locator('div').filter({hasText: /^Access token$/}).locator('input[type="password"]').fill('aaa');
    await setupDataSourceWithClientCredentials(page, GRAFANA_CLIENT);
    await expect(page.getByText(EXPLORE_DATA)).toHaveCount(0);
});
