import { test, expect, Page } from '@playwright/test';
import { ensureAnonymous, loginFreshUser, createEventViaCookie, authedPost, randomEmail } from './helpers';

async function setPendingCookie(page: Page, value: string) {
  await page.context().addCookies([{
    name: 'pending_event',
    value,
    url: new URL(page.url()).origin,
  }]);
}

test.describe('Security and edge cases', () => {
  test('gallery titles are HTML escaped', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, '<script>alert(1)</script>', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
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
      expect(page.url()).not.toMatch(/\/overview\/[a-zA-Z0-9]{10,}/);
      await expect(page).toHaveURL(/\/create/);
    });
  }

  test('extra pending-cookie fields cannot choose the owner', async ({ page }) => {
    await loginFreshUser(page);
    const value = Buffer.from(JSON.stringify({
      title: 'Injection Test',
      prompts: 'Gallery uploads',
      design_id: 'classic',
      qr_mode: 'single',
      owner: 'DIFFERENT_USER_ID',
    })).toString('base64');
    await setPendingCookie(page, value);
    await page.goto('/create/finish');
    await expect(page).toHaveURL(/\/overview\/[a-zA-Z0-9]+/);
    await expect(page.locator('h1')).toContainText('Injection Test');
  });

  test('invalid and empty design changes are rejected', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Design Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    expect((await authedPost(page, `/design/${galleryId}`, { design_id: 'absolutely_invalid' })).status).toBe(400);
    expect((await authedPost(page, `/design/${galleryId}`)).status).toBe(400);
    await page.goto(`/overview/${galleryId}`);
    await expect(page.locator('input[name="design_id"][value="classic"]')).toBeChecked();
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
    await expect(nav.locator('a.btn-nav[href="/pricing"]')).toBeVisible();

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

  test('the simplified create form has gallery, date, design, and one hidden upload bucket', async ({ page }) => {
    await page.goto('/create');
    await expect(page.locator('#eventTitle')).toBeVisible();
    await expect(page.locator('#eventDate')).toBeVisible();
    await expect(page.locator('input[name="design_id"]')).toHaveCount(5);
    await expect(page.locator('input[name="prompts"]')).toHaveValue(/Photos and videos/i);
    await expect(page.locator('input[name="qr_mode"]')).toHaveValue('single');
  });

  test('free tier rejects more than the single hidden upload bucket', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Limit Test', ['One', 'Two'], 'classic', 'single');
    expect(galleryId).toBeNull();
    await expect(page.locator('.error-card')).toContainText(/limit|plan|upgrade/i);
  });

  test('free tier accepts exactly one upload bucket', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Max Free Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
  });
});
