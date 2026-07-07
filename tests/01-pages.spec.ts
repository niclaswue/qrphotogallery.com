import { test, expect } from '@playwright/test';
import { ensureAnonymous } from './helpers';

test.describe('Test 1: Home page — anonymous visitor', () => {
  test.beforeEach(async ({ page }) => {
    await ensureAnonymous(page);
  });

  test('page loads with HTTP 200', async ({ page }) => {
    const response = await page.goto('/');
    expect(response!.status()).toBe(200);
  });

  test('title contains the brand suffix', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/photo/i);
  });

  test('hero h1 mentions guests', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('h1').first()).toContainText(/guest|photographer/i);
  });

  test('hero subtitle mentions guests', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('.hero-subtitle')).toContainText(/guests/i);
  });

  test('primary hero CTA links to /create', async ({ page }) => {
    await page.goto('/');
    const cta = page.locator('.hero-actions a.btn-primary[href="/create"]');
    await expect(cta).toBeVisible();
  });

  test('how-it-works has three steps', async ({ page }) => {
    await page.goto('/');
    const steps = page.locator('.how-it-works .step');
    await expect(steps).toHaveCount(3);
  });

  test('feature grid shows the four benefits', async ({ page }) => {
    await page.goto('/');
    const features = page.locator('.feature-grid .feature');
    await expect(features).toHaveCount(4);
  });

  test('nav shows Login, Register, Pricing for anonymous', async ({ page }) => {
    await page.goto('/');
    const nav = page.locator('nav.site-nav').first();
    await expect(nav.locator('a[href="/login"]')).toBeVisible();
    await expect(nav.locator('a[href="/register"]')).toBeVisible();
    await expect(nav.locator('a[href="/pricing"]')).toBeVisible();
  });

  test('footer has Legal & Privacy link', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('footer a[href="/legal"]')).toBeVisible();
  });
});

test.describe('Test 2: Pricing page — anonymous visitor', () => {
  test.beforeEach(async ({ page }) => {
    await ensureAnonymous(page);
    await page.goto('/pricing');
  });

  test('three pricing cards visible', async ({ page }) => {
    const cards = page.locator('.price-card');
    await expect(cards).toHaveCount(3);
  });

  test('Free tier shows €0 price and the 5-prompt limit', async ({ page }) => {
    const freeCard = page.locator('.price-card').nth(0);
    await expect(freeCard.locator('.price')).toContainText('€0');
    await expect(freeCard).toContainText('5');
  });

  test('Free tier CTA links to /create', async ({ page }) => {
    const freeCard = page.locator('.price-card').nth(0);
    await expect(freeCard.locator('a[href="/create"]')).toBeVisible();
  });

  test('Standard tier shows €29 and a 100-prompt limit', async ({ page }) => {
    const standardCard = page.locator('.price-card').nth(1);
    await expect(standardCard.locator('.price')).toContainText('€29');
    await expect(standardCard).toContainText('100');
  });

  test('Standard tier CTA links to /payment?plan=standard', async ({ page }) => {
    const standardCard = page.locator('.price-card').nth(1);
    await expect(standardCard.locator('a[href*="/payment?plan=standard"]')).toBeVisible();
  });

  test('Premium tier shows €39 and the 500-prompt limit', async ({ page }) => {
    const premiumCard = page.locator('.price-card').nth(2);
    await expect(premiumCard.locator('.price')).toContainText('€39');
    await expect(premiumCard).toContainText('500');
  });

  test('Premium tier CTA links to /payment?plan=premium', async ({ page }) => {
    const premiumCard = page.locator('.price-card').nth(2);
    await expect(premiumCard.locator('a[href*="/payment?plan=premium"]')).toBeVisible();
  });

  test('Standard card has the popular styling', async ({ page }) => {
    const standardCard = page.locator('.price-card').nth(1);
    await expect(standardCard).toHaveClass(/price-card-featured/);
  });
});

test.describe('Test 3: Legal page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/legal');
  });

  test('three legal sections exist', async ({ page }) => {
    await expect(page.locator('#imprint')).toBeVisible();
    await expect(page.locator('#privacy')).toBeVisible();
    await expect(page.locator('#refund')).toBeVisible();
  });

  test('Imprint section is rendered', async ({ page }) => {
    await expect(page.locator('#imprint h1, #imprint h2').first()).toBeVisible();
  });

  test('Privacy section is rendered', async ({ page }) => {
    await expect(page.locator('#privacy h1, #privacy h2').first()).toBeVisible();
  });

  test('Refund section is rendered', async ({ page }) => {
    await expect(page.locator('#refund h1, #refund h2').first()).toBeVisible();
  });

  test('sections have readable content', async ({ page }) => {
    for (const id of ['#imprint', '#privacy', '#refund']) {
      const content = page.locator(`${id} p, ${id} li`);
      await expect(content.first()).toBeVisible();
    }
  });
});

