import { test, expect } from './fixtures.js';

const PERIODS = [
  { key: 'Today', label: 'today', showsCompare: false },
  { key: 'Yesterday', label: 'yesterday', showsCompare: false },
  { key: '7d', label: '7d', showsCompare: true },
  { key: '30d', label: '30d', showsCompare: true },
  { key: '90d', label: '90d', showsCompare: true },
  { key: '365d', label: '365d', showsCompare: true },
];

test.describe('Initial Load', () => {
  test('header, period selector, and defaults', async ({ loadedPage: page }) => {
    await expect(page.locator('h1')).toContainText('Copilot Monitor');
    await expect(page.locator('.period-bar')).toBeVisible();
    await expect(page.locator('.period-btn')).toHaveCount(6);
    await expect(page.locator('.period-btn.active')).toHaveText('30d');
    await expect(page.locator('.metric-label').first()).toContainText('30d');
  });

  test('chart, model table, live session, compare panel', async ({ loadedPage: page }) => {
    await expect(page.locator('#chart')).toBeVisible();
    await expect(page.locator('table tbody tr').first()).toBeVisible();
    await expect(page.locator('.live-session')).toBeVisible();
    await expect(page.locator('.compare-grid')).toBeVisible();
  });
});

test.describe('Period Switching', () => {
  for (const period of PERIODS) {
    test(`switches to ${period.key} and updates all sections`, async ({ loadedPage: page }) => {
      await page.locator(`.period-btn:has-text("${period.key}")`).click();

      await expect(page.locator('.period-btn.active')).toHaveText(period.key);
      await expect(page.locator('.metric-label').first()).toContainText(period.label);
      await expect(page.locator('#chart')).toBeVisible();
      await expect(page.locator('table tbody tr').first()).toBeVisible();

      const compare = page.locator('.compare-grid');
      if (period.showsCompare) {
        await expect(compare).toBeVisible();
      } else {
        await expect(compare).toBeHidden();
      }
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
    const subtitle = page.locator('.subtitle');
    await expect(subtitle).not.toHaveText('Loading…');
    await page.locator('.refresh-btn').click();
    await expect(subtitle).not.toHaveText('Loading…');
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

test.describe('Sessions Table', () => {
  test('renders sessions section', async ({ loadedPage: page }) => {
    await expect(page.locator('h2:has-text("Recent Sessions")')).toBeVisible();
  });
});