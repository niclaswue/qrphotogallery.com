import { test, expect } from '@playwright/test';
import { ensureAnonymous, loginFreshUser, createEventViaCookie, authedRequest } from './helpers';

test.describe('Guest access and route protection', () => {
  test('shared QR route renders the upload page without authentication', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Upload Test Wedding', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await ensureAnonymous(page);
    const response = await page.goto(`/e/${galleryId}`);
    expect(response!.status()).toBe(200);
    await expect(page.locator('.guest-title h1')).toContainText('Upload Test Wedding');
    await expect(page.locator('input#photo')).toBeAttached();
    await expect(page.locator('input#photo')).toHaveAttribute('multiple', '');
  });

  test('missing galleries return localized HTML 404 pages', async ({ page }) => {
    const response = await page.goto('/de/e/completely_fake_id');
    expect(response!.status()).toBe(404);
    await expect(page.locator('html')).toHaveAttribute('lang', 'de');
    await expect(page.locator('h1')).toContainText('Nicht gefunden');
    const bodyText = (await page.locator('body').textContent())?.trim() || '';
    expect(bodyText.startsWith('{')).toBeFalsy();
  });

  test('done page renders success copy and gallery link', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Done Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await page.goto(`/e/${galleryId}/done?mode=qr`);
    await expect(page.locator('.done-card h1')).toBeVisible();
    await expect(page.locator(`a[href="/e/${galleryId}/library"]`)).toBeVisible();
  });

  for (const route of ['/overview/some_id', '/gallery/some_id', '/edit/some_id', '/poster/some_id', '/download/some_id']) {
    test(`${route} is protected from anonymous access`, async ({ page }) => {
      await ensureAnonymous(page);
      const response = await page.goto(route);
      const bouncedToAuth = /\/(login|register)/.test(page.url());
      expect(bouncedToAuth || response!.status() >= 400).toBeTruthy();
    });
  }

  test("a second user cannot access another user's gallery", async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Private Gallery', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await page.goto('/logout');
    await ensureAnonymous(page);
    await loginFreshUser(page);
    const { status } = await authedRequest(page, `/overview/${galleryId}`);
    expect([401, 403, 404]).toContain(status);
  });

  test('payment success never upgrades a free account by itself', async ({ page }) => {
    await loginFreshUser(page);
    const cookie = (await page.context().cookies()).find(c => c.name === 'pb_auth')!;
    await page.goto('/payment/success?plan=standard');
    await expect(page.locator('body')).toContainText(/Payment|upgrade/i);
    const tier = await (await page.request.get('/api/user/tier', {
      headers: { Cookie: `pb_auth=${cookie.value}` },
    })).json();
    expect(tier.tier).toBe('free');
    expect(tier.max_prompts).toBe(1);
  });

  test('pricing links to both hosted checkout routes', async ({ page }) => {
    await page.goto('/pricing');
    await expect(page.locator('a[href*="/payment?plan=standard"]').first()).toBeVisible();
    await expect(page.locator('a[href*="/payment?plan=premium"]').first()).toBeVisible();
  });

  test('an invalid hidden upload destination returns 404', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Invalid Destination Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    const response = await page.goto(`/e/${galleryId}/not_a_real_prompt`);
    expect(response!.status()).toBe(404);
  });
});
