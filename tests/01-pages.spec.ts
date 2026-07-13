import { test, expect } from '@playwright/test';
import { ensureAnonymous } from './helpers';

test.describe('Public product pages', () => {
  test.beforeEach(async ({ page }) => {
    await ensureAnonymous(page);
  });

  test('home page presents the one-QR gallery MVP', async ({ page }) => {
    const response = await page.goto('/');
    expect(response!.status()).toBe(200);
    await expect(page).toHaveTitle(/QR Photo Gallery/i);
    await expect(page.locator('h1')).toContainText(/One QR code/i);
    await expect(page.locator('.hero-subtitle')).toContainText(/Guests/i);
    await expect(page.locator('.hero-actions a.btn-primary')).toHaveAttribute('href', '/create');
    await expect(page.locator('.how-section .steps article')).toHaveCount(3);
    await expect(page.locator('.feature-list article')).toHaveCount(4);
    expect(await page.locator('body').innerText()).not.toMatch(/\[[a-z_]+\.[^\]]+\]/i);
  });

  test('anonymous navigation exposes pricing, login, and create CTA', async ({ page }) => {
    await page.goto('/');
    const nav = page.locator('nav.site-nav').first();
    await expect(nav.locator('a[href="/login"]')).toBeVisible();
    await expect(nav.locator('a[href="/pricing"]')).toHaveCount(1);
    await expect(nav.locator('a.btn-nav[href="/create"]')).toContainText(/Create/i);
    await expect(page.locator('footer a[href="/legal"]')).toBeVisible();
  });

  test('pricing shows the configured personal and commercial offers', async ({ page }) => {
    await page.goto('/pricing');
    const cards = page.locator('.price-card');
    await expect(cards).toHaveCount(2);
    await expect(cards.nth(0)).toContainText('€19');
    await expect(cards.nth(0)).toContainText(/100 GB/i);
    await expect(cards.nth(0).locator('a[href*="/payment?plan=standard"]')).toBeVisible();
    await expect(cards.nth(0)).toHaveClass(/price-card-featured/);
    await expect(cards.nth(1)).toContainText('€29');
    await expect(cards.nth(1).locator('a[href*="/payment?plan=premium"]')).toBeVisible();
    await expect(page.locator('main')).not.toContainText(/prompt|challenge|theme/i);
  });

  test('English legal page renders all policy sections', async ({ page }) => {
    await page.goto('/legal');
    await expect(page.locator('html')).toHaveAttribute('lang', 'en');
    for (const id of ['#imprint', '#privacy', '#refund']) {
      await expect(page.locator(id)).toBeVisible();
      await expect(page.locator(`${id} h2`).first()).toBeVisible();
      await expect(page.locator(`${id} p, ${id} li`).first()).toBeVisible();
    }
    await expect(page.locator('#privacy')).toContainText('Privacy Policy');
  });

  test('German pages and legal content do not fall back to English', async ({ page }) => {
    await page.goto('/de/');
    await expect(page.locator('html')).toHaveAttribute('lang', 'de');
    await expect(page.locator('h1')).toContainText('Ein QR-Code');
    await expect(page.locator('.hero-subtitle')).toContainText('Gäste');
    expect(await page.content()).toContain('Analyse-Cookies');

    await page.goto('/de/pricing');
    await expect(page.locator('.price-card').nth(0)).toContainText('19 €');
    await expect(page.locator('.price-card').nth(1)).toContainText('29 €');

    await page.goto('/de/legal');
    await expect(page.locator('h1')).toContainText('Rechtliches');
    await expect(page.locator('#privacy')).toContainText('Datenschutzerklärung');
    await expect(page.locator('#privacy')).not.toContainText('Privacy Policy');
  });
});
