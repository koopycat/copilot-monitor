import { test, expect } from './fixtures.js';

const PERIODS = [
  { key: 'Today', label: 'today' },
  { key: 'Yesterday', label: 'yesterday' },
  { key: '7d', label: '7d' },
  { key: '30d', label: '30d' },
  { key: '90d', label: '90d' },
  { key: '365d', label: '365d' },
];

test.describe('Initial Load', () => {
  test('header, period selector, and defaults', async ({ loadedPage: page }) => {
    await expect(page.locator('h1')).toContainText('Copilot Monitor');
    await expect(page.locator('.period-bar')).toBeVisible();
    await expect(page.locator('.period-btn')).toHaveCount(6);
    await expect(page.locator('.period-btn.active')).toHaveText('30d');
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
      await page.locator(`.period-btn:has-text("${period.key}")`).click();

      await expect(page.locator('.period-btn.active')).toHaveText(period.key);
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
  test('refresh button updates timestamp', async ({ loadedPage: page }) => {
    // Both subtitles exist in DOM with x-show; data loads → "Loading…" hides
    const loading = page.locator('.subtitle').filter({ hasText: 'Loading…' });
    await expect(loading).toBeHidden({ timeout: 10_000 });
    await page.locator('.refresh-btn').click();
    await expect(loading).toBeHidden({ timeout: 10_000 });
  });
});

test.describe('Export Link', () => {
  test('href reflects active period', async ({ loadedPage: page }) => {
    const link = page.locator('a:has-text("Export CSV")');
    await expect(link).toHaveAttribute('href', /since=30d/);

    await page.locator('.period-btn:has-text("7d")').click();
    await expect(link).toHaveAttribute('href', /since=7d/);
  });
});

test.describe('Auto Granularity', () => {
  test('today defaults to hour granularity', async ({ loadedPage: page }) => {
    await page.locator('.period-btn:has-text("Today")').click();
    await expect(page.locator('.toggle-btn:has-text("Hour")')).toHaveClass(/active/);
  });

  test('90d defaults to day granularity', async ({ loadedPage: page }) => {
    await page.locator('.period-btn:has-text("90d")').click();
    await expect(page.locator('.toggle-btn:has-text("Day")')).toHaveClass(/active/);
  });
});

test.describe('Table Sections', () => {
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
    await page.locator('.refresh-btn').click();
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
