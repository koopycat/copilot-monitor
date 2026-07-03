import { test as base } from '@playwright/test';

export const test = base.extend({
  loadedPage: async ({ page }, use) => {
    await page.goto('/', { waitUntil: 'domcontentloaded' });
    await page.locator('.period-btn').first().waitFor();
    await use(page);
  },
});

export { expect } from '@playwright/test';
