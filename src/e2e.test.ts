import { test, expect, Page } from '@playwright/test';

const GRAFANA_CLIENT = 'grafana-client';
const EXPLORE_DATA = 'Explore';

async function login(page: Page) {
    await page.goto('http://localhost:3000/login');
    await page.getByPlaceholder('email or username').fill('admin');
    await page.getByPlaceholder('password').fill('admin');
    await page.getByRole('button', { name: 'Login button' }).or(page.getByRole('button', { name: 'Log in' })).click();
    await page.getByRole('button', { name: /skip/i }).click();
}

async function goToTrinoSettings(page: Page) {
    await page.getByRole('button', { name: /toggle menu|open menu/i }).click();
    await page.getByText('Connections').click();
    await page.getByText(/add.*connection|connect data/i).first().click();
    await page.getByText('Trino').click();
    await page.getByRole('button', { name: /add new data source|create.*trino.*data source/i }).click();
}

async function setupDataSourceWithAccessToken(page: Page) {
    await page.getByPlaceholder('http://localhost:8080').fill('http://trino:8080');
    await page.locator('label[for="trino-settings-enable-impersonation"][aria-label="Toggle switch"], label[for="trino-settings-enable-impersonation"]:has(svg)').first().click();
    await page.locator('div').filter({hasText: /^Access token$/}).locator('input[type="password"]').fill('aaa');
    await page.getByRole('button', { name: /save.*test/i }).click();
}

async function setupDataSourceWithClientCredentials(page: Page, clientId: string) {
    await page.getByPlaceholder('http://localhost:8080').fill('http://trino:8080');
    await page.locator('div').filter({hasText: /^Token URL$/}).locator('input').fill('http://keycloak:8080/realms/trino-realm/protocol/openid-connect/token');
    await page.locator('div').filter({hasText: /^Client id$/}).locator('input').fill(clientId);
    await page.locator('div').filter({hasText: /^Client secret$/}).locator('input[type="password"]').fill('grafana-secret');
    await page.locator('div').filter({hasText: /^Impersonation user$/}).locator('input').fill('service-account-grafana-client');
    await page.getByRole('button', { name: /save.*test/i }).click();
}

async function runQueryAndCheckResults(page: Page) {
    await page.getByText(EXPLORE_DATA).click();
    await page.locator('textarea[aria-label*="Editor content"]').click();
    await page.getByTestId('data-testid TimePicker Open Button').click();
    const timeInputs = page.locator('input[value*="now"], input[value*="00:00:00"]');
    await timeInputs.first().fill('1995-01-01');
    await timeInputs.last().fill('1995-12-31');
    await page.getByTestId('data-testid TimePicker submit button').click();
    await page.locator('[role="combobox"][aria-label="Format as"]').click();
    await page.locator('[id^="react-select"][id$="option-1"]').waitFor();
    await page.locator('[id^="react-select"][id$="option-1"]').click();
    await expect(page.locator('[role="table"][aria-label="Explore Table"]').getByText(/1995-01-\d+/).first()).toBeVisible();
}

async function setupDataSourceWithClientTags(page: Page, clientTags: string) {
    await page.getByPlaceholder('http://localhost:8080').fill('http://trino:8080');
    await page.locator('label[for="trino-settings-enable-impersonation"][aria-label="Toggle switch"], label[for="trino-settings-enable-impersonation"]:has(svg)').first().click();
    await page.locator('div').filter({hasText: /^Access token$/}).locator('input[type="password"]').fill('aaa');
    await page.locator('div').filter({hasText: /^Client Tags$/}).locator('input').fill(clientTags);
    await page.getByRole('button', { name: /save.*test/i }).click();
}

async function setupDataSourceWithRoles(page: Page, roles: string) {
    await page.getByPlaceholder('http://localhost:8080').fill('http://trino:8080');
    await page.locator('div').filter({hasText: /^Roles$/}).locator('input').fill(roles);
    await page.getByRole('button', { name: /save.*test/i }).click();
}

async function runRoleQuery(page: Page) {
    await page.getByText(EXPLORE_DATA).click();
    await setQuery(page, 'SHOW ROLES FROM hive')
    await page.locator('[role="combobox"][aria-label="Format as"]').click();
    await page.locator('[id^="react-select"][id$="option-1"]').waitFor();
    await page.locator('[id^="react-select"][id$="option-1"]').click();

}

async function setQuery(page: Page, query: string) {
    await page.locator('textarea[aria-label*="Editor content"]').click();
    await page.keyboard.press('Control+a');
    await page.keyboard.type(query);
}

test('test client credentials flow with wrong credentials', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithClientCredentials(page, "some-wrong-client");
    await expect(page.getByTestId('data-testid Alert error')).toBeVisible();
    await expect(page.getByTestId('data-testid Alert error')).toContainText('Bad Request');
});

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

test('test client credentials flow with configured access token', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await page.locator('div').filter({hasText: /^Access token$/}).locator('input[type="password"]').fill('aaa');
    await setupDataSourceWithClientCredentials(page, GRAFANA_CLIENT);
    await expect(page.getByTestId('data-testid Alert error')).toBeVisible();
    await expect(page.getByTestId('data-testid Alert error')).toContainText('Internal Server Error');
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
    await expect(page.locator('[role="table"][aria-label="Explore Table"]').getByText(/.*admin.*/).first()).toBeVisible();

});

test('test without role', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithRoles(page, '');
    await runRoleQuery(page);
    await expect(page.getByText(/Access Denied: Cannot show roles/)).toBeVisible();
});
