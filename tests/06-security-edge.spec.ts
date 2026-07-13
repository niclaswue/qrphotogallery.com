import { test, expect, Page } from '@playwright/test';
import { ensureAnonymous, loginFreshUser, createEventViaCookie, randomEmail } from './helpers';

async function setPendingCookie(page: Page, value: string) {
  await page.context().addCookies([{ name: 'pending_event', value, url: new URL(page.url()).origin }]);
}

test.describe('Security and edge cases', () => {
  test('gallery titles are HTML escaped', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, '<script>alert(1)</script>');
    await page.goto(`/overview/${galleryId}`);
    expect(await page.content()).not.toMatch(/<script>alert\(1\)<\/script>/);
    await expect(page.locator('h1')).toContainText('<script>alert(1)</script>');
  });

  test('login rejects protocol-relative redirects', async ({ page }) => {
    const email = randomEmail();
    await page.goto('/register');
    await page.fill('input[name="email"]', email);
    await page.fill('input[name="password"]', 'password123');
    await page.fill('input[name="password_confirm"]', 'password123');
    await page.locator('button[type="submit"]').click();
    await page.goto('/logout');
    await page.goto('/login?redirect=//evil.com');
    await page.fill('input[name="identity"]', email);
    await page.fill('input[name="password"]', 'password123');
    await page.locator('button[type="submit"]').click();
    expect(page.url()).not.toContain('evil.com');
  });

  for (const [name, value] of [
    ['invalid base64', 'not_valid_base64!!!'],
    ['invalid JSON', Buffer.from('this is not json').toString('base64')],
    ['missing fields', Buffer.from('{"foo":"bar"}').toString('base64')],
  ]) {
    test(`pending gallery cookie with ${name} creates nothing`, async ({ page }) => {
      await loginFreshUser(page);
      await setPendingCookie(page, value);
      await page.goto('/create/finish');
      await expect(page).toHaveURL(/\/create/);
    });
  }

  test('extra pending-cookie fields cannot choose owner or product shape', async ({ page }) => {
    await loginFreshUser(page);
    const value = Buffer.from(JSON.stringify({
      title: 'Injection Test',
      prompts: 'Injected task',
      design_id: 'modern',
      qr_mode: 'cards',
      owner: 'DIFFERENT_USER_ID',
    })).toString('base64');
    await setPendingCookie(page, value);
    await page.goto('/create/finish');
    await expect(page).toHaveURL(/\/overview\/[a-zA-Z0-9]+/);
    await expect(page.locator('h1')).toContainText('Injection Test');
    await expect(page.locator('body')).not.toContainText('Injected task');
  });

  test('PocketBase admin UI is reachable and missing static files stay 404', async ({ page }) => {
    expect((await page.goto('/_/'))!.status()).toBe(200);
    expect((await page.goto('/static/nonexistent.css'))!.status()).toBe(404);
  });

  test('navigation changes between anonymous and authenticated states', async ({ page }) => {
    await ensureAnonymous(page);
    await page.goto('/');
    let nav = page.locator('nav.site-nav').first();
    await expect(nav.locator('a[href="/login"]')).toBeVisible();
    await expect(nav.locator('a.btn-nav[href="/create"]')).toBeVisible();

    await loginFreshUser(page);
    await page.goto('/');
    nav = page.locator('nav.site-nav').first();
    await expect(nav.locator('a[href="/overview"]')).toBeVisible();
    await expect(nav.locator('a[href="/logout"]')).toBeVisible();
  });

  test('login/register cross-links preserve safe redirects', async ({ page }) => {
    await page.goto('/login?redirect=/create');
    await expect(page.locator('a[href*="/register"][href*="redirect="]')).toBeVisible();
    await page.goto('/register?redirect=/overview');
    await expect(page.locator('a[href*="/login"][href*="redirect="]')).toBeVisible();
  });

  test('registration fields carry browser validation attributes', async ({ page }) => {
    await page.goto('/register');
    await expect(page.locator('input[name="email"]')).toHaveAttribute('required', '');
    await expect(page.locator('input[name="password"]')).toHaveAttribute('required', '');
    await expect(page.locator('input[name="password"]')).toHaveAttribute('minlength', '8');
  });

  test('create form contains only gallery name and optional date', async ({ page }) => {
    await page.goto('/create');
    await expect(page.locator('#eventTitle')).toBeVisible();
    await expect(page.locator('#eventDate')).toBeVisible();
    await expect(page.locator('input[name="design_id"], input[name="prompts"], input[name="qr_mode"]')).toHaveCount(0);
    expect(await page.locator('main').innerText()).not.toMatch(/prompt|theme|look|card/i);
    expect(await page.locator('body').innerText()).not.toMatch(/\[[a-z_]+\.[^\]]+\]/i);
  });
});
