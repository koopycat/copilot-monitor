import { test, expect } from './fixtures.js';

const PERIODS = [
  { key: 'Today', label: 'today' },
  { key: 'Yesterday', label: 'yesterday' },
  { key: '7d', label: '7d' },
  { key: '30d', label: '30d' },
  { key: '90d', label: '90d' },
  { key: '365d', label: '365d' },
];

function dashboardPeriodBar(page) {
  return page.locator('.period-bar:not(.session-period-bar)');
}

test.describe('Initial Load', () => {
  test('header, period selector, and defaults', async ({ loadedPage: page }) => {
    const periodBar = dashboardPeriodBar(page);

    await expect(page.locator('h1')).toContainText('Copilot Monitor');
    await expect(periodBar).toBeVisible();
    await expect(periodBar.locator('.period-btn')).toHaveCount(6);
    await expect(periodBar.locator('.period-btn.active')).toHaveText('30d');
    await expect(page.locator('.metric-label').first()).toContainText('30d');
  });

  test('chart, model table, and live session', async ({ loadedPage: page }) => {
    await expect(page.locator('#chart')).toBeVisible();
    await expect(page.locator('table tbody tr').first()).toBeVisible();
    await expect(page.locator('.live-session')).toBeVisible();
  });
});

test.describe('Period Switching', () => {
  for (const period of PERIODS) {
    test(`switches to ${period.key}`, async ({ loadedPage: page }) => {
      const periodBar = dashboardPeriodBar(page);
      await periodBar.getByRole('button', { name: period.key, exact: true }).click();

      await expect(periodBar.locator('.period-btn.active')).toHaveText(period.key);
      await expect(page.locator('.metric-label').first()).toContainText(period.label);
      await expect(page.locator('#chart')).toBeVisible();
      await expect(page.locator('table tbody tr').first()).toBeVisible();
    });
  }
});

test.describe('Granularity Toggle', () => {
  test('30d defaults to day granularity', async ({ loadedPage: page }) => {
    await expect(page.locator('.toggle-btn:has-text("Day")')).toHaveClass(/active/);
  });

  test('hour granularity toggle works', async ({ loadedPage: page }) => {
    await page.locator('.toggle-btn:has-text("Hour")').click();
    await expect(page.locator('.toggle-btn:has-text("Hour")')).toHaveClass(/active/);
    await expect(page.locator('#chart')).toBeVisible();
  });
});

test.describe('Metric Toggle', () => {
  test('tokens is default active', async ({ loadedPage: page }) => {
    await expect(page.locator('.toggle-btn:has-text("Tokens")')).toHaveClass(/active/);
  });

  test('requests toggle works', async ({ loadedPage: page }) => {
    await page.locator('.toggle-btn:has-text("Requests")').click();
    await expect(page.locator('.toggle-btn:has-text("Requests")')).toHaveClass(/active/);
  });
});

test.describe('Refresh', () => {
  test('manual refresh completes', async ({ loadedPage: page }) => {
    const refreshButton = page.getByRole('button', { name: 'Refresh now' });
    const statsResponse = page.waitForResponse(
      (response) => response.url().includes('/api/stats?') && response.request().method() === 'GET',
    );

    await refreshButton.click();
    await statsResponse;
    await expect(refreshButton).toBeEnabled();
    await expect(page.locator('.subtitle')).toContainText('Updated');
  });
});

test.describe('Export Link', () => {
  test('href reflects active period', async ({ loadedPage: page }) => {
    const link = page.locator('a:has-text("Export CSV")');
    await expect(link).toHaveAttribute('href', /since=30d/);

    await dashboardPeriodBar(page).getByRole('button', { name: '7d', exact: true }).click();
    await expect(link).toHaveAttribute('href', /since=7d/);
  });
});

test.describe('Auto Granularity', () => {
  test('today defaults to hour granularity', async ({ loadedPage: page }) => {
    await dashboardPeriodBar(page).getByRole('button', { name: 'Today', exact: true }).click();
    await expect(page.locator('.toggle-btn:has-text("Hour")')).toHaveClass(/active/);
  });

  test('90d defaults to day granularity', async ({ loadedPage: page }) => {
    await dashboardPeriodBar(page).getByRole('button', { name: '90d', exact: true }).click();
    await expect(page.locator('.toggle-btn:has-text("Day")')).toHaveClass(/active/);
  });
});

test.describe('Model Row Disclosure', () => {
  test('shows five rows by default and toggles all rows from the keyboard', async ({
    loadedPage: page,
  }) => {
    const models = page.locator('.models-section');
    const rows = models.locator('tbody tr');
    const toggle = models.locator('.model-row-toggle');

    await expect(rows).toHaveCount(5);
    await expect(toggle).toHaveText(/^Show all \d+ models$/);
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
    await expect(toggle).toHaveAttribute('aria-controls', 'models-table-body');
    await expect(models.locator('#models-table-body')).toHaveCount(1);

    const match = (await toggle.textContent())?.match(/^Show all (\d+) models$/);
    const totalModels = Number(match?.[1] ?? 0);
    expect(totalModels).toBeGreaterThan(5);

    await toggle.focus();
    await page.keyboard.press('Enter');
    await expect(rows).toHaveCount(totalModels);
    await expect(toggle).toHaveText('Show top 5');
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');

    await toggle.focus();
    await page.keyboard.press('Space');
    await expect(rows).toHaveCount(5);
    await expect(toggle).toHaveText(`Show all ${totalModels} models`);
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  });

  test('sorts the complete row set before limiting without changing disclosure state', async ({
    loadedPage: page,
  }) => {
    const models = page.locator('.models-section');
    const rows = models.locator('tbody tr');
    const toggle = models.locator('.model-row-toggle');
    const requestSort = models.getByRole('button', { name: /^Requests/ });
    const requestValues = async () =>
      (await rows.locator('td:nth-child(3)').allTextContents()).map((value) =>
        Number(value.replaceAll(',', '').trim()),
      );

    await toggle.click();
    const totalModels = await rows.count();
    expect(totalModels).toBeGreaterThan(5);

    await requestSort.click();
    await expect(rows).toHaveCount(totalModels);
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');

    const allDescending = await requestValues();
    expect(allDescending).toEqual([...allDescending].sort((a, b) => b - a));

    await toggle.click();
    await expect(rows).toHaveCount(5);
    await requestSort.click();

    const visibleAscending = await requestValues();
    expect(visibleAscending).toEqual([...allDescending].sort((a, b) => a - b).slice(0, 5));
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  });

  test('preserves row disclosure and visual encoding through refresh and section toggles', async ({
    loadedPage: page,
  }) => {
    const models = page.locator('.models-section');
    const rows = models.locator('tbody tr');
    const toggle = models.locator('.model-row-toggle');
    const encoding = () =>
      rows
        .locator('.bar-inline')
        .evaluateAll((bars) => bars.map((bar) => bar.getAttribute('style')));

    const compactEncoding = await encoding();
    await toggle.click();
    const totalModels = await rows.count();
    expect(totalModels).toBeGreaterThan(5);
    expect((await encoding()).slice(0, compactEncoding.length)).toEqual(compactEncoding);

    const statsResponse = page.waitForResponse(
      (response) => response.url().includes('/api/stats?') && response.request().method() === 'GET',
    );
    const refreshButton = page.getByRole('button', { name: 'Refresh now' });
    await refreshButton.click();
    await statsResponse;
    await expect(refreshButton).toBeEnabled();
    await expect(rows).toHaveCount(totalModels);
    await expect(toggle).toHaveText('Show top 5');
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');

    await models.locator('summary').click();
    await expect(toggle).toBeHidden();
    await models.locator('summary').click();
    await expect(toggle).toBeVisible();
    await expect(rows).toHaveCount(totalModels);
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });

  test('keeps the disclosure control usable at a narrow viewport', async ({ loadedPage: page }) => {
    await page.setViewportSize({ width: 390, height: 844 });

    const models = page.locator('.models-section');
    const rows = models.locator('tbody tr');
    const toggle = models.locator('.model-row-toggle');
    const match = (await toggle.textContent())?.match(/^Show all (\d+) models$/);
    const totalModels = Number(match?.[1] ?? 0);

    await expect(toggle).toBeVisible();
    const box = await toggle.boundingBox();
    expect(box).not.toBeNull();
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(390);

    await toggle.click();
    await expect(rows).toHaveCount(totalModels);
    await expect(toggle).toHaveText('Show top 5');
  });

  test('shows every row without a disclosure control when five models exist', async ({ page }) => {
    await page.route(/\/api\/stats\?/, async (route) => {
      const response = await route.fetch();
      const stats = await response.json();
      await route.fulfill({ response, json: stats.slice(0, 5) });
    });

    await page.goto('/', { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('button', { name: 'Refresh now' })).toBeEnabled();

    const models = page.locator('.models-section');
    await expect(models.locator('tbody tr')).toHaveCount(5);
    await expect(models.locator('.model-row-toggle')).toHaveCount(0);
  });
});

test.describe('Table Sections', () => {
  test('all collapsible sections retain native disclosure markers', async ({
    loadedPage: page,
  }) => {
    const summaries = [
      '.models-section > summary',
      '.sessions-section > summary',
      '.anomaly-feed > summary',
    ];

    for (const selector of summaries) {
      await expect(page.locator(selector)).toHaveCSS('display', 'list-item');
    }
  });

  test('models and sessions sections are expanded initially', async ({ loadedPage: page }) => {
    await expect(page.locator('.models-section')).toHaveAttribute('open', '');
    await expect(page.locator('.sessions-section')).toHaveAttribute('open', '');
    await expect(page.locator('.models-section summary')).toContainText('Models');
    await expect(page.locator('.sessions-section summary')).toContainText('Recent Sessions');
    await expect(page.locator('.models-section table')).toBeVisible();
    await expect(page.locator('.sessions-section table')).toBeVisible();
  });

  test('collapsed state survives a data refresh', async ({ loadedPage: page }) => {
    const models = page.locator('.models-section');
    await models.locator('summary').click();
    await expect(models.locator('table')).toBeHidden();

    const statsResponse = page.waitForResponse(
      (response) => response.url().includes('/api/stats?') && response.request().method() === 'GET',
    );
    await page.getByRole('button', { name: 'Refresh now' }).click();
    await statsResponse;
    await expect(models.locator('table')).toBeHidden();
  });

  test('sections collapse independently with the keyboard', async ({ loadedPage: page }) => {
    const models = page.locator('.models-section');
    const sessions = page.locator('.sessions-section');

    await models.locator('summary').focus();
    await page.keyboard.press('Enter');
    await expect(models).not.toHaveAttribute('open', '');
    await expect(models.locator('summary')).toContainText('Models');
    await expect(models.locator('table')).toBeHidden();
    await expect(sessions.locator('table')).toBeVisible();

    await sessions.locator('summary').focus();
    await page.keyboard.press('Enter');
    await expect(sessions).not.toHaveAttribute('open', '');
    await expect(sessions.locator('summary')).toContainText('Recent Sessions');
    await expect(sessions.locator('table')).toBeHidden();
    await expect(models.locator('table')).toBeHidden();

    await models.locator('summary').focus();
    await page.keyboard.press('Enter');
    await expect(models.locator('table')).toBeVisible();
    await expect(sessions.locator('table')).toBeHidden();
  });
});
