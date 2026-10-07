import { test, expect, Page } from '@playwright/test';
import { execSync } from 'child_process';

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

// Fills the query editor's Client tags field. Scoped by the label text rather
// than a role or testid - InlineField renders the label and the input as
// siblings inside one wrapper div on every supported Grafana version, while
// @grafana/ui's Input markup around them does not.
async function setQueryClientTags(page: Page, clientTags: string) {
    await page.locator('div').filter({hasText: /^Client tags$/}).locator('input').fill(clientTags);
    // typing only updates the query model - blurring the field commits it
    await page.keyboard.press('Tab');
    await page.waitForTimeout(500);
}

async function runQueryAndCheckResults(page: Page, queryClientTags?: string) {
    await page.getByLabel(EXPORT_DATA).click();
    await page.getByTestId('data-testid TimePicker Open Button').click();
    await page.getByTestId('data-testid Time Range from field').fill('1995-01-01');
    await page.getByTestId('data-testid Time Range to field').fill('1995-12-31');
    await page.getByTestId('data-testid TimePicker submit button').click();
    await commitQuery(page);
    if (queryClientTags !== undefined) {
        await setQueryClientTags(page, queryClientTags);
    }
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

test('test with client tags set in the query editor', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithClientTags(page, 'tag1,tag2,tag3');
    await runQueryAndCheckResults(page, 'panelTag');
});

async function setupDataSourceWithAsyncQueries(page: Page) {
    await page.getByTestId('data-testid Datasource HTTP settings url').fill('http://trino:8080');
    await page.locator('label[for="trino-settings-enable-impersonation"]').last().click();
    await page.locator('label[for="trino-settings-enable-async-query-data"]').last().click();
    await page.locator('div').filter({hasText: /^Access token$/}).locator('input[type="password"]').fill('aaa');
    await page.getByTestId('data-testid Data source settings page Save and Test button').click();
}

// Collects the body of every query request the page sends. The two flows are
// only distinguishable on the wire: they render identically, so asserting on
// the page alone cannot tell a polled query from a synchronous one. Matching
// requests rather than markup also keeps these tests clear of the
// @grafana/ui differences between the supported Grafana versions.
function recordQueryRequests(page: Page): string[] {
    const bodies: string[] = [];
    page.on('request', (request) => {
        if (request.method() === 'POST' && request.url().includes('/api/ds/query')) {
            const body = request.postData();
            if (body) {
                bodies.push(body);
            }
        }
    });
    return bodies;
}

test('test with asynchronous queries', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    const queryRequests = recordQueryRequests(page);
    await setupDataSourceWithAsyncQueries(page);
    // Same results as every other flow - polling must not change what the
    // panel ends up showing, only how it gets there.
    await runQueryAndCheckResults(page);

    // queryFlow marks a request as opting into the polling flow, and queryID
    // is the handle the backend hands back, which only a follow-up poll can
    // carry. Seeing both proves the query really went through the two-phase
    // flow instead of quietly falling back to the synchronous path, which
    // would produce the identical table above.
    expect(queryRequests.some((body) => body.includes('"queryFlow":"async"'))).toBe(true);
    expect(queryRequests.some((body) => body.includes('"queryID":"'))).toBe(true);
});

test('test without asynchronous queries', async ({ page }) => {
    // Negative control for the toggle: the setting is opt-in, so a data source
    // that never enables it must keep sending plain synchronous queries.
    await login(page);
    await goToTrinoSettings(page);
    const queryRequests = recordQueryRequests(page);
    await setupDataSourceWithAccessToken(page);
    await runQueryAndCheckResults(page);

    expect(queryRequests.length).toBeGreaterThan(0);
    expect(queryRequests.some((body) => body.includes('"queryFlow":"async"'))).toBe(false);
    expect(queryRequests.some((body) => body.includes('"queryID":"'))).toBe(false);
});

// The two tests above finish in well under a second, so they only ever prove
// the two-phase shape - the parts that matter under load never run. These
// drive a query that takes about a minute, which is what forces the poll
// backoff up to its ceiling and makes cancellation observable on the cluster.
// They cost a minute each and need the tpch.sf10 schema, so they are opt-in
// rather than part of every matrix run, the same way the PDC tests are.
// TRINO_CONTAINER names the Trino container to ask about query state; it
// differs between the `yarn server` stack and CI.
const SLOW_QUERY_TESTS = process.env.TRINO_SLOW_QUERY_TESTS;
const TRINO_CONTAINER = process.env.TRINO_CONTAINER ?? 'grafana-trino-trino-1';

const SLOW_QUERY =
    "SELECT count(*) AS n FROM tpch.sf10.lineitem l JOIN tpch.sf10.orders o ON l.orderkey = o.orderkey WHERE o.orderdate >= DATE '1995-01-01' AND o.orderdate < DATE '1995-02-01'";
const SLOW_QUERY_RESULT = '775514';

// Asks Trino directly, so cancellation is confirmed on the cluster rather
// than from the plugin's own account of itself.
function trinoQueryStates(): string {
    const sql =
        'SELECT query_id, state, error_code FROM system.runtime.queries ' +
        "WHERE query LIKE '%sf10.lineitem%' AND query NOT LIKE '%system.runtime%' " +
        'ORDER BY created DESC LIMIT 3';
    return execSync(`docker exec ${TRINO_CONTAINER} trino --user admin --execute ${JSON.stringify(sql)}`, {
        encoding: 'utf8',
    });
}

async function runSlowQuery(page: Page) {
    await page.getByLabel(EXPORT_DATA).click();
    await setQuery(page, SLOW_QUERY);
    await page.getByTestId('data-testid Code editor container').click();
    await selectFormat(page, 'Time Series', 'Table');
    await page.getByTestId('data-testid Code editor container').click();
    await page.getByTestId('data-testid RefreshPicker run button').click();
}

test.describe('long running asynchronous queries', () => {
    test.skip(!SLOW_QUERY_TESTS, 'TRINO_SLOW_QUERY_TESTS is not set, see DEVELOPMENT.md');
    test.describe.configure({ timeout: 10 * 60 * 1000 });

    test('test a long running query completes by polling', async ({ page }) => {
        const polls: number[] = [];
        const statuses: string[] = [];
        const start = Date.now();

        page.on('request', (request) => {
            if (request.method() === 'POST' && request.url().includes('/api/ds/query')) {
                if ((request.postData() ?? '').includes('"queryFlow":"async"')) {
                    polls.push(Date.now() - start);
                }
            }
        });
        page.on('response', async (response) => {
            if (!response.url().includes('/api/ds/query')) {
                return;
            }
            // The body is gone once a response is superseded, which is normal
            // here and not worth failing the test over.
            const body = await response.text().catch(() => '');
            const status = body.match(/"status":"(submitted|running|finished|failed|canceled)"/);
            if (status) {
                statuses.push(status[1]);
            }
        });

        await login(page);
        await goToTrinoSettings(page);
        await setupDataSourceWithAsyncQueries(page);
        await runSlowQuery(page);

        await expect(page.getByText(SLOW_QUERY_RESULT, { exact: true })).toBeVisible({ timeout: 5 * 60 * 1000 });

        // A query this long cannot have been answered in one request, and the
        // looper doubles its delay to a 10s ceiling rather than busy-polling,
        // so the longest gap has to land near that ceiling.
        const gaps = polls.slice(1).map((at, i) => at - polls[i]);
        expect(polls.length).toBeGreaterThan(5);
        expect(Math.max(...gaps)).toBeGreaterThan(5000);
        expect(statuses).toContain('submitted');
        expect(statuses).toContain('finished');
    });

    test('test cancelling a long running query stops it in Trino', async ({ page }) => {
        const cancelCalls: string[] = [];
        page.on('request', (request) => {
            if (request.url().includes('/resources/cancel')) {
                cancelCalls.push(request.postData() ?? '');
            }
        });

        await login(page);
        await goToTrinoSettings(page);
        await setupDataSourceWithAsyncQueries(page);
        await runSlowQuery(page);

        // Wait until Trino itself reports the query running, so the
        // cancellation below is aimed at something real.
        await expect(async () => {
            expect(trinoQueryStates()).toContain('RUNNING');
        }).toPass({ timeout: 60000 });

        // While a query is in flight the run button turns into Cancel, keeping
        // the same test id and carrying its state only in the aria-label.
        const runButton = page.getByTestId('data-testid RefreshPicker run button');
        await expect(runButton).toHaveAttribute('aria-label', 'Cancel');
        await runButton.click();

        // Trino has no CANCELED state: a client cancellation lands as FAILED
        // with error_code USER_CANCELED, which is what tells it apart from a
        // query that failed on its own. Getting there proves the chain all the
        // way through - the frontend's cancel resource, the registry
        // cancelling the run context, and the driver turning that into
        // DELETE /v1/query/{id} on its way out.
        await expect(async () => {
            const states = trinoQueryStates();
            expect(states).not.toContain('RUNNING');
            expect(states).toContain('USER_CANCELED');
        }).toPass({ timeout: 60000 });

        expect(cancelCalls.length).toBeGreaterThan(0);
        // The handle is the one this plugin process minted, in processID:uuid form.
        expect(cancelCalls[0]).toMatch(/"queryId":"[0-9a-f-]{36}:[0-9a-f-]{36}"/);
    });
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

test('test with complex types', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await setupDataSourceWithAccessToken(page);
    await page.getByLabel(EXPORT_DATA).click();
    await setQuery(page, "SELECT ARRAY[CAST(ROW(0.95, 'Bonsoir') AS ROW(score double, word varchar))] AS words, MAP(ARRAY['k'], ARRAY[ARRAY[1, 2]]) AS nested");
    await page.getByTestId('data-testid Code editor container').click();
    await selectFormat(page, 'Time Series', 'Table');
    await page.getByTestId('data-testid Code editor container').click();
    await page.getByTestId('data-testid RefreshPicker run button').click();
    // JSON cells are pretty-printed on some Grafana versions and compact on
    // others, so allow optional whitespace between tokens.
    await expect(page.getByText(/\[\s*\{\s*"score":\s*0\.95,\s*"word":\s*"Bonsoir"\s*\}\s*\]/).first()).toBeVisible({timeout: 15000});
    await expect(page.getByText(/\{\s*"k":\s*\[\s*1,\s*2\s*\]\s*\}/).first()).toBeVisible();
    await expect(page.getByText(/error querying the database/i)).toHaveCount(0);
    // Explore on Grafana 11.6 through 12.3 replaces each field's custom config
    // with its own column-limit settings, discarding the `inspect` flag this
    // plugin sets, so the inspect button never renders there. Fixed in 12.4.
    const [major, minor] = await grafanaVersion(page);
    if ((major === 11 && minor >= 6) || (major === 12 && minor < 4)) {
        return;
    }
    // The cell inspect button only renders while the cell is hovered.
    await page.getByText(/"score"/).first().hover();
    await page.getByRole('button', {name: 'Inspect value'}).first().click();
    await expect(page.getByText('Inspect value', {exact: true}).first()).toBeVisible();
});

async function grafanaVersion(page: Page): Promise<number[]> {
    const version: string = await page.evaluate(() => (window as any).grafanaBootData.settings.buildInfo.version);
    return version.split(/[.-]/).slice(0, 2).map(Number);
}

test('test impersonation with user email', async ({ page }) => {
    await login(page);
    await goToTrinoSettings(page);
    await page.getByTestId('data-testid Datasource HTTP settings url').fill('http://trino:8080');
    await page.locator('label[for="trino-settings-enable-impersonation"]').last().click();
    await page.getByRole('radio', {name: 'Email'}).check();
    await page.getByTestId('data-testid Data source settings page Save and Test button').click();
    await page.getByLabel(EXPORT_DATA).click();
    await setQuery(page, 'SELECT current_user AS trino_user');
    await page.getByTestId('data-testid Code editor container').click();
    await selectFormat(page, 'Time Series', 'Table');
    await page.getByTestId('data-testid Code editor container').click();
    await page.getByTestId('data-testid RefreshPicker run button').click();
    await expect(page.getByText('admin@localhost', {exact: true})).toBeVisible({timeout: 15000});
});

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
    // Wait for the save to land before navigating away - other tests get this
    // for free by staying on the page to click "Explore data", but this one
    // leaves immediately and would otherwise race the in-flight save, leaving
    // a datasource with no URL for the variable query to use.
    await expect(page.getByText('Data source is working')).toBeVisible({timeout: 15000});

    await page.goto('http://localhost:3000/dashboard/new?editview=templating&editIndex=0');
    await page.getByRole('tab', {name: 'Variables'}).click();

    // Newer Grafana versions moved variable management out of this
    // Settings tab into the dashboard's edit sidebar; the tab now just
    // shows a "Take me there" banner instead of an "Add variable" button.
    // Both flows end up at the same query editor, just reached differently.
    const takeMeThere = page.getByRole('button', {name: 'Take me there'});
    if (await takeMeThere.isVisible({timeout: 3000}).catch(() => false)) {
        await takeMeThere.click();
        // Grafana 13.2 renamed this control's test id from "edit pane" to
        // "sidebar"; accept either so both sides of that rename work.
        await page.getByTestId(/data-testid (edit pane|sidebar) add new variable button/).click();
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
