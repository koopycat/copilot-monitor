import { chromium } from 'playwright';
import { expect } from '@playwright/test';

const BASE = 'http://127.0.0.1:7740';
const LANDING = `${BASE}/site/`;
const issues = [];

async function check(name, fn) {
  try {
    await fn();
    console.log(`  PASS ${name}`);
  } catch (e) {
    issues.push(`${name}: ${e.message.split('\n')[0]}`);
    console.log(`  FAIL ${name}: ${e.message.split('\n')[0]}`);
  }
}

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({
  viewport: { width: 1280, height: 900 },
  colorScheme: 'dark', // landing page is dark by default
});
const page = await context.newPage();

const requests = [];
const failed = [];
page.on('request', r => requests.push({ url: r.url(), status: 'pending' }));
page.on('response', r => {
  const item = requests.find(x => x.url === r.url());
  if (item) item.status = String(r.status());
});
page.on('requestfailed', r => failed.push(r.url()));

console.log('\n=== Load ===');
const resp = await page.goto(LANDING, { waitUntil: 'networkidle' });
console.log(`  HTTP ${resp.status()}`);

await check('status is 200', async () => {
  expect(resp.status()).toBe(200);
});

await check('title is set', async () => {
  const title = await page.title();
  expect(title).toContain('Copilot Monitor');
});

await check('meta description present', async () => {
  const desc = await page.locator('meta[name="description"]').getAttribute('content');
  expect(desc.length).toBeGreaterThan(50);
});

console.log('\n=== Hero ===');
await check('hero h1 has tagline', async () => {
  const h1 = await page.locator('h1').first().textContent();
  expect(h1).toContain('GitHub Copilot');
  expect(h1).toContain('on your machine');
});

await check('install command visible', async () => {
  await expect(page.locator('pre.install').first()).toBeVisible();
  const txt = await page.locator('pre.install').first().textContent();
  expect(txt).toContain('copilot-monitor run');
});

await check('primary CTA points to GitHub', async () => {
  const href = await page.locator('a.btn.primary').first().getAttribute('href');
  expect(href).toMatch(/github\.com/);
});

console.log('\n=== Sections ===');
const sectionTitles = await page.locator('section h2').allTextContents();
console.log(`  sections: ${sectionTitles.join(' | ')}`);

await check('"Why" section', async () => {
  expect(sectionTitles.some(t => t === 'Why')).toBe(true);
});

await check('"What it looks like" section', async () => {
  expect(sectionTitles.some(t => t.includes('What it looks like'))).toBe(true);
});

await check('"What you get" section', async () => {
  expect(sectionTitles.some(t => t === 'What you get')).toBe(true);
});

await check('"What is and isn\'t stored" section', async () => {
  expect(sectionTitles.some(t => t.includes('stored'))).toBe(true);
});

await check('"Who it\'s for" section', async () => {
  expect(sectionTitles.some(t => t.includes("Who it's for"))).toBe(true);
});

await check('"Get started" section', async () => {
  expect(sectionTitles.some(t => t === 'Get started')).toBe(true);
});

console.log('\n=== Feature cards ===');
const featureCount = await page.locator('.feature').count();
console.log(`  feature cards: ${featureCount}`);
await check('at least 5 feature cards', async () => {
  expect(featureCount).toBeGreaterThanOrEqual(5);
});

console.log('\n=== Mock dashboard ===');
await check('mock dashboard renders', async () => {
  await expect(page.locator('.mock').first()).toBeVisible();
});

await check('mock has at least one bar', async () => {
  const bars = await page.locator('.mock .chart .bar').count();
  expect(bars).toBeGreaterThan(10);
});

await check('mock table has model rows', async () => {
  const rows = await page.locator('.mock tbody tr').count();
  expect(rows).toBeGreaterThanOrEqual(3);
});

await check('mock table mentions Sonnet or gpt-4.1', async () => {
  const txt = await page.locator('.mock').first().textContent();
  expect(txt).toMatch(/claude|gpt/i);
});

console.log('\n=== Privacy callout ===');
await check('stored list present', async () => {
  await expect(page.locator('.privacy-card.stored')).toBeVisible();
});

await check('not-stored list present', async () => {
  await expect(page.locator('.privacy-card.not-stored')).toBeVisible();
});

await check('mentions prompts are not stored', async () => {
  const txt = await page.locator('.privacy-card.not-stored').textContent();
  expect(txt).toContain('Prompt');
});

await check('mentions auth headers are not stored', async () => {
  const txt = await page.locator('.privacy-card.not-stored').textContent();
  expect(txt).toMatch(/auth|cookie/i);
});

console.log('\n=== Personas ===');
const personas = await page.locator('.persona').count();
console.log(`  personas: ${personas}`);
await check('at least 3 personas', async () => {
  expect(personas).toBeGreaterThanOrEqual(3);
});

console.log('\n=== Footer ===');
await check('footer present', async () => {
  await expect(page.locator('footer')).toBeVisible();
});

await check('footer links to api/architecture', async () => {
  const links = await page.locator('footer a').allTextContents();
  expect(links.some(l => l === 'API')).toBe(true);
  expect(links.some(l => l === 'Architecture')).toBe(true);
});

console.log('\n=== Internal links ===');
const links = await page.locator('a').evaluateAll(els =>
  els.map(a => ({ text: a.textContent.trim(), href: a.getAttribute('href') }))
    .filter(l => l.href && !l.href.startsWith('http') && !l.href.startsWith('#') && !l.href.startsWith('mailto:'))
);
console.log(`  internal links: ${JSON.stringify(links, null, 2)}`);

await check('"Get started" section links to GitHub repo', async () => {
  const href = await page.locator('a:has-text("Quickstart")').getAttribute('href');
  expect(href).toMatch(/github\.com.*#quickstart/);
});

console.log('\n=== Resources ===');
console.log(`  total requests: ${requests.length}`);
console.log(`  failed: ${failed.length}`);
const nonOk = requests.filter(r => r.status !== '200' && r.status !== 'pending' && !r.status.startsWith('3'));
if (nonOk.length) {
  console.log(`  non-OK responses:`);
  nonOk.forEach(r => console.log(`    ${r.status} ${r.url}`));
}

await check('no failed requests', async () => {
  expect(failed).toHaveLength(0);
});

await check('all resources returned 200', async () => {
  expect(nonOk).toHaveLength(0);
});

console.log('\n=== Layout ===');
await check('no horizontal scroll at 1280px', async () => {
  const overflow = await page.evaluate(() => {
    return document.documentElement.scrollWidth - document.documentElement.clientWidth;
  });
  expect(overflow).toBeLessThanOrEqual(1);
});

console.log('\n=== Dark theme ===');
const bg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
const accent = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--accent').trim());
console.log(`  body bg: ${bg}`);
console.log(`  accent var: ${accent}`);
await check('body background is dark', async () => {
  expect(bg).toBe('rgb(13, 17, 23)');
});

console.log('\n=== Mobile (375px) ===');
await page.setViewportSize({ width: 375, height: 800 });
await page.waitForTimeout(300);
const mobileOverflow = await page.evaluate(() =>
  document.documentElement.scrollWidth - document.documentElement.clientWidth
);
console.log(`  mobile overflow: ${mobileOverflow}px`);
await check('no horizontal scroll at 375px', async () => {
  expect(mobileOverflow).toBeLessThanOrEqual(1);
});

await page.screenshot({ path: '/tmp/landing-mobile.png', fullPage: true });
await page.setViewportSize({ width: 1280, height: 900 });
await page.screenshot({ path: '/tmp/landing-desktop.png', fullPage: true });

await browser.close();

console.log(`\n${'='.repeat(50)}`);
if (issues.length === 0) {
  console.log('ALL CHECKS PASSED');
} else {
  console.log(`${issues.length} CHECK(S) FAILED:`);
  issues.forEach(i => console.log(`  - ${i}`));
}
process.exit(issues.length > 0 ? 1 : 0);
