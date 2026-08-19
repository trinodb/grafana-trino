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

// Opens the QueryEditor's "Format as" dropdown by clicking the currently
// displayed value text rather than the Select's accessible role/name -
// older Grafana versions load an @grafana/ui build (from the host, not
// bundled with this plugin) whose Select doesn't expose an accessible name
// on the combobox trigger, even though the control itself works fine.
// A hidden overlay intercepts plain clicks on some versions, hence `force`.
async function openFormatDropdown(page: Page, currentValue: string) {
    await page.getByText(currentValue, { exact: true }).click({ force: true });
}

// Selects a "Format as" option and waits for the dropdown's closing overlay
// to clear - clicking straight into the next control (e.g. Run query)
// immediately after selecting an option can silently get swallowed by that
// overlay, with no error, just no request ever fired.
async function selectFormat(page: Page, currentValue: string, option: string) {
    await openFormatDropdown(page, currentValue);
    // On some older Grafana versions every option shares the same generic
    // accessible name ("Select option"); the real label is only in a child
    // element. Filter by visible text instead of accessible name.
    await page.getByRole('option').filter({hasText: option}).click();
    await page.waitForTimeout(500);
}

// Commits the code editor's current value (even if untouched/default) by
// focusing then blurring it. Must happen before any other query-affecting
// change (e.g. format selection) - on some older Grafana versions, changing
// the format re-runs the query immediately, and if the editor's default
// text was never committed to the bound query object yet, that run goes out
// with an empty rawSQL. This is a real, reproducible behavior difference in
// Explore across Grafana versions, not something this plugin controls.
async function commitQuery(page: Page) {
    await page.getByTestId('data-testid Code editor container').click();
    await page.keyboard.press('Escape');
    await page.waitForTimeout(500);
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
    await commitQuery(page);
    await selectFormat(page, 'Time Series', 'Table');
    await page.getByTestId('data-testid Code editor container').click();
    const runButton = page.getByTestId('data-testid RefreshPicker run button');
    // wait until any running queries have finished - the button is icon-only
    // on newer Grafana, so aria-label is the only text carrying its state
    await expect(runButton).toHaveAttribute('aria-label', /run/i, {timeout: 15000});
    await runButton.click();
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

// PDC_PRIVATE_TRINO_URL points at a Trino instance reachable only through
// the secure SOCKS proxy (no direct network route from Grafana). That proves
// the proxy toggle actually routes traffic rather than just accepting the
// setting, since a query can only succeed here if it truly went through the
// proxy. It needs a Grafana with the secure SOCKS proxy configured plus the
// isolated Trino and proxy containers, which the default `yarn server` stack
// doesn't start - so these two tests are skipped unless the variable is set.
// CI sets it, and DEVELOPMENT.md covers running them locally.
const PDC_PRIVATE_TRINO_URL = process.env.PDC_PRIVATE_TRINO_URL;

test.describe('secure socks proxy (PDC)', () => {
    test.skip(!PDC_PRIVATE_TRINO_URL, 'PDC_PRIVATE_TRINO_URL is not set, see DEVELOPMENT.md');

    test('test with secure socks proxy (PDC)', async ({ page }) => {
        await login(page);
        await goToTrinoSettings(page);
        await page.getByTestId('data-testid Datasource HTTP settings url').fill(PDC_PRIVATE_TRINO_URL!);
        await page.locator('label[for="trino-settings-enable-secure-socks-proxy"]').last().click();
        await page.getByTestId('data-testid Data source settings page Save and Test button').click();
        await expect(page.getByText('Data source is working')).toBeVisible({timeout: 10000});
        await runQueryAndCheckResults(page);
    });

    test('test without secure socks proxy cannot reach a PDC-only host', async ({ page }) => {
        // Negative control: the same otherwise-unreachable host, without
        // enabling the proxy toggle, must fail - proving the prior test's
        // success is actually caused by the proxy and not some other route.
        // Save & Test alone can't show this: trino-go-client doesn't implement
        // database/sql's Pinger interface, so CheckHealth is a no-op regardless
        // of reachability (see the "check health failure" test above). Only an
        // actual query attempt forces a real connection.
        await login(page);
        await goToTrinoSettings(page);
        await page.getByTestId('data-testid Datasource HTTP settings url').fill(PDC_PRIVATE_TRINO_URL!);
        await page.getByTestId('data-testid Data source settings page Save and Test button').click();
        await page.getByLabel(EXPORT_DATA).click();
        await commitQuery(page);
        // The Explore graph view renders a plain "No data" for a query error
        // instead of visible error text - Table format surfaces it properly
        // (same as the roles tests above).
        await selectFormat(page, 'Time Series', 'Table');
        await page.getByTestId('data-testid Code editor container').click();
        await page.getByTestId('data-testid RefreshPicker run button').click();
        await expect(page.getByText(/error querying the database/i)).toBeVisible({timeout: 15000});
    });
});

test('test with roles', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithRoles(page, 'system:ALL;hive:admin');
    await runRoleQuery(page);
    // Table panel cells render as role="gridcell" on newer @grafana/ui
    // (virtualized grid) and plain role="cell" on older versions (semantic
    // <table>) - match on visible text instead of a specific role.
    await expect(page.getByText('admin', { exact: true })).toBeVisible();

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
    await setQuery(page, 'SHOW ROLES FROM hive')
    await page.getByTestId('data-testid Code editor container').click();
    await selectFormat(page, 'Time Series', 'Table');
    await page.getByTestId('data-testid Code editor container').click();
    await page.getByTestId('data-testid RefreshPicker run button').click();
}

async function setQuery(page: Page, query: string) {
    const editor = page.getByTestId('data-testid Code editor container');
    // Give Monaco a moment to finish mounting before selecting-all - a
    // quad-click immediately after mount can land before the model is
    // ready, silently failing to select the existing (default) text, so
    // the typed text gets inserted alongside it instead of replacing it.
    await editor.waitFor();
    await page.waitForTimeout(500);
    await editor.click({ clickCount: 4 });
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
    await commitQuery(page);
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
    await setQuery(page, "SELECT orderdate as time, orderstatus as level, 'order ' || cast(orderkey as varchar) as message FROM tpch.tiny.orders WHERE $__timeFilter(orderdate)");
    await page.getByTestId('data-testid Code editor container').click();
    await selectFormat(page, 'Time Series', 'Logs');
    await page.getByTestId('data-testid Code editor container').click();
    await page.getByTestId('data-testid RefreshPicker run button').click();
    await expect(page.getByRole('button', {name: 'Logs volume'})).toBeVisible();
    await expect(page.getByText(/error querying the database/i)).toHaveCount(0);
});

test('test template variable backed by trino query', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithAccessToken(page);

    await page.goto('http://localhost:3000/dashboard/new?editview=templating&editIndex=0');
    await page.getByRole('tab', {name: 'Variables'}).click();

    // Newer Grafana versions moved variable management out of this
    // Settings tab into the dashboard's edit sidebar; the tab now just
    // shows a "Take me there" banner instead of an "Add variable" button.
    // Both flows end up at the same query editor, just reached differently.
    const takeMeThere = page.getByRole('button', {name: 'Take me there'});
    if (await takeMeThere.isVisible({timeout: 3000}).catch(() => false)) {
        await takeMeThere.click();
        await page.getByTestId('data-testid edit pane add new variable button').click();
        await page.getByTestId('data-testid variable type query').click();
        await page.getByTestId('data-testid variable name input').fill('orderstatus');
        await page.getByText('Open variable editor').click();
        // The Trino datasource is already preselected as the only configured
        // datasource. Falls back to StandardVariableSupport's generic query
        // textarea, since this plugin doesn't implement a custom variable
        // query editor.
        await page.getByTestId('data-testid Variable editor Form Default Variable Query Editor textarea').fill('SELECT DISTINCT orderstatus FROM tpch.tiny.orders');
        await page.getByRole('button', {name: 'Run query'}).click();
        await expect(page.getByText(/Preview of values/)).toBeVisible({timeout: 10000});
        // The query editor opens in a modal dialog - scope to it since the
        // rest of the dashboard-builder page behind it also renders text.
        const dialog = page.getByRole('dialog');
        await expect(dialog.getByText('F', {exact: true}).first()).toBeVisible();
        await expect(dialog.getByText('O', {exact: true}).first()).toBeVisible();
        await expect(dialog.getByText('P', {exact: true}).first()).toBeVisible();
        return;
    }

    await page.getByRole('button', {name: 'Add variable'}).click();
    await page.getByTestId('data-testid Variable editor Form Name field').fill('orderstatus');
    await page.getByRole('textbox', {name: 'Metric name or tags query'}).fill('SELECT DISTINCT orderstatus FROM tpch.tiny.orders');
    await page.getByRole('button', {name: 'Run query'}).click();
    // Older Grafana versions don't show the "(N)" count suffix.
    await expect(page.getByText(/Preview of values/)).toBeVisible({timeout: 10000});
    // Older Grafana renders the preview as plain inline text tags; newer
    // versions render an actual sortable table. Scope to the variable
    // editor form and match loosely rather than assume either structure.
    const variableForm = page.getByRole('form', {name: 'Variable editor form'});
    await expect(variableForm.getByText('F', {exact: true}).first()).toBeVisible();
    await expect(variableForm.getByText('O', {exact: true}).first()).toBeVisible();
    await expect(variableForm.getByText('P', {exact: true}).first()).toBeVisible();
});
