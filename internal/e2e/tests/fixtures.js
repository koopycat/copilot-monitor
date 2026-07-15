import { expect, test as base } from '@playwright/test';

export const test = base.extend({
  loadedPage: async ({ page }, use) => {
    await page.goto('/', { waitUntil: 'domcontentloaded' });
    await page.locator('.period-btn').first().waitFor();
    await expect(page.getByRole('button', { name: 'Refresh now' })).toBeEnabled();
    await use(page);
  },
});

export { expect };
