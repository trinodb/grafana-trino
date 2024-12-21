import { test, expect } from '@playwright/test';

test('test', async ({ page }) => {
  await page.goto('http://localhost:3000/login');
  await page.getByTestId('data-testid Username input field').fill('admin');
  await page.getByTestId('data-testid Password input field').fill('admin');
  await page.getByTestId('data-testid Login button').click();
  await page.getByTestId('data-testid Skip change password button').click();
  await page.getByTestId('data-testid Toggle menu').click();
  await page.getByRole('link', { name: 'Connections' }).click();
  await page.getByRole('link', { name: 'Trino' }).click();
  await page.locator('.css-1yhi3xa').click();
  await page.getByRole('button', { name: 'Add new data source' }).click();
  await page.getByTestId('data-testid Datasource HTTP settings url').fill('http://trino:8080');
  await page.locator('div').filter({ hasText: /^Impersonate logged in userAccess token$/ }).getByLabel('Toggle switch').click();
  await page.locator('input[type="password"]').fill('aaa');
  await page.getByTestId('data-testid Data source settings page Save and Test button').click();
  await page.getByLabel('Explore data').click();
  await page.getByTestId('data-testid TimePicker Open Button').click();
  await page.getByTestId('data-testid Time Range from field').fill('1995-01-01');
  await page.getByTestId('data-testid Time Range to field').fill('1995-12-31');
  await page.getByTestId('data-testid TimePicker submit button').click();
  await page.locator('div').filter({ hasText: /^Format asChoose$/ }).locator('svg').click();
  await page.getByRole('option', { name: 'Table' }).click();
  await page.getByTestId('data-testid Code editor container').click();
  await page.getByTestId('data-testid RefreshPicker run button').click();
  await expect(page.getByTestId('data-testid table body')).toContainText(/.*1995-01-19 0.:00:005703857F.*/);
});
