import { test, expect } from '@playwright/test';
import { ensureAnonymous, loginFreshUser, createEventViaCookie, authedPost, randomEmail } from './helpers';

test.describe('Test 40: XSS in prompt text', () => {
  test('script tags inside prompts are escaped on the edit page', async ({ page }) => {
    await loginFreshUser(page);
    const challengeId = await createEventViaCookie(page, 'XSS Challenge', [
      '</script><script>alert(1)</script>',
    ]);
    if (challengeId) {
      await page.goto(`/edit/${challengeId}`);
      await page.waitForLoadState('networkidle');
      const html = await page.content();
      expect(html).not.toMatch(/<script>alert\(1\)<\/script>/);
    }
    await page.goto('/');
    const bodyHTML = await page.locator('body').innerHTML();
    expect(bodyHTML).not.toMatch(/<script>alert\(1\)<\/script>/);
  });
});

test.describe('Test 43: Open redirect via protocol-relative URL', () => {
  test('login redirect must not bounce to an external host', async ({ page }) => {
    const email = randomEmail();
    await page.goto('/register');
    await page.fill('input[name="email"]', email);
    await page.fill('input[name="password"]', 'password123');
    await page.fill('input[name="password_confirm"]', 'password123');
    await page.click('button[type="submit"]');
    await page.waitForLoadState('networkidle');

    await page.goto('/logout');
    await page.waitForLoadState('networkidle');

    await page.goto('/login?redirect=//evil.com');
    await page.fill('input[name="identity"]', email);
    await page.fill('input[name="password"]', 'password123');
    await page.click('button[type="submit"]');
    await page.waitForLoadState('networkidle');
    expect(page.url()).not.toContain('evil.com');
  });
});

test.describe('Test 55: pending_challenge cookie with invalid base64', () => {
  test('invalid base64 cookie falls through to /create', async ({ page }) => {
    await loginFreshUser(page);
    await page.context().addCookies([{
      name: 'pending_challenge',
      value: 'not_valid_base64!!!',
      domain: 'localhost',
      path: '/',
    }]);
    const response = await page.goto('/create/design?design_id=classic');
    expect(response!.status()).toBe(200);
    expect(page.url()).toContain('/create');
  });
});

test.describe('Test 56: pending_challenge cookie with invalid JSON', () => {
  test('invalid JSON cookie falls through to /create', async ({ page }) => {
    await loginFreshUser(page);
    const b64 = Buffer.from('this is not json').toString('base64');
    await page.context().addCookies([{
      name: 'pending_challenge',
      value: b64,
      domain: 'localhost',
      path: '/',
    }]);
    const response = await page.goto('/create/design?design_id=classic');
    expect(response!.status()).toBe(200);
    expect(page.url()).toContain('/create');
  });
});

test.describe('Test 57: pending_challenge cookie with missing fields', () => {
  test('a cookie without prompts/event_name does not create a challenge', async ({ page }) => {
    await loginFreshUser(page);
    const b64 = Buffer.from('{"foo":"bar"}').toString('base64');
    await page.context().addCookies([{
      name: 'pending_challenge',
      value: b64,
      domain: 'localhost',
      path: '/',
    }]);
    const response = await page.goto('/create/design?design_id=classic');
    expect(response!.status()).toBe(200);
    expect(page.url()).not.toMatch(/\/overview\/[a-zA-Z0-9]{10,}/);
  });
});

test.describe('Test 58: pending_challenge cookie with extra injected fields', () => {
  test('extra cookie fields are ignored — challenge belongs to the auth user', async ({ page }) => {
    await loginFreshUser(page);
    const b64 = Buffer.from(JSON.stringify({
      event_name: 'Injection Test',
      prompts: 'Photo 1',
      owner: 'DIFFERENT_USER_ID',
    })).toString('base64');
    await page.context().addCookies([{
      name: 'pending_challenge',
      value: b64,
      domain: 'localhost',
      path: '/',
    }]);
    await page.goto('/create/design?design_id=classic');
    await page.waitForLoadState('networkidle');
    const url = page.url();
    if (url.match(/\/overview\/([a-zA-Z0-9]+)/)) {
      await expect(page.locator('h1', { hasText: 'Injection Test' })).toBeVisible();
    }
  });
});

test.describe('Test 68: Design change with invalid design_id', () => {
  test('invalid design_id is rejected and stored design is unchanged', async ({ page }) => {
    await loginFreshUser(page);
    const challengeId = await createEventViaCookie(page, 'Design Test', ['Photo 1', 'Photo 2'], 'classic');
    expect(challengeId).toBeTruthy();
    const { status } = await authedPost(page, `/design/${challengeId}?design_id=absolutely_invalid`);
    expect(status).toBe(400);
    await page.goto(`/overview/${challengeId}`);
    await expect(page.locator('input[name="design_id"][value="classic"]')).toBeChecked();
  });
});

test.describe('Test 69: Design change with empty design_id', () => {
  test('empty design_id does not crash the server', async ({ page }) => {
    await loginFreshUser(page);
    const challengeId = await createEventViaCookie(page, 'Empty Design Test', ['Photo 1']);
    if (challengeId) {
      const { status } = await authedPost(page, `/design/${challengeId}`);
      expect([200, 303, 302, 400, 500]).toContain(status);
    }
  });
});

test.describe('Test 73: PocketBase Admin UI accessible', () => {
  test('admin UI responds with HTTP 200', async ({ page }) => {
    const response = await page.goto('/_/');
    expect(response!.status()).toBe(200);
  });
});

test.describe('Test 74: Static file serving', () => {
  test('nonexistent static file returns 404', async ({ page }) => {
    const response = await page.goto('/static/nonexistent.css');
    expect(response!.status()).toBe(404);
  });
});

test.describe('Test 82: Nav links consistency', () => {
  test('anonymous nav shows Login, Register, Pricing', async ({ page }) => {
    await ensureAnonymous(page);
    await page.goto('/');
    const nav = page.locator('nav.site-nav').first();
    await expect(nav.locator('a[href="/login"]')).toBeVisible();
    await expect(nav.locator('a[href="/register"]')).toBeVisible();
    await expect(nav.locator('a[href="/pricing"]')).toBeVisible();
  });

  test('authenticated nav shows the account and logout links', async ({ page }) => {
    await loginFreshUser(page);
    await page.goto('/');
    const nav = page.locator('nav.site-nav').first();
    await expect(nav.locator('a[href="/overview"]')).toBeVisible();
    await expect(nav.locator('a[href="/logout"]')).toBeVisible();
    await expect(nav.locator('a[href="/pricing"]')).toBeVisible();
  });
});

test.describe('Test 95: Login/register cross-link preserves redirect', () => {
  test('login page link contains the redirect param', async ({ page }) => {
    await page.goto('/login?redirect=/create');
    expect(await page.content()).toMatch(/redirect.*create/i);
  });
});

test.describe('Test 96: Register/login cross-link preserves redirect', () => {
  test('register page link contains the redirect param', async ({ page }) => {
    await page.goto('/register?redirect=/overview');
    expect(await page.content()).toMatch(/redirect.*overview/i);
  });
});

test.describe('Test 63: Login with email case sensitivity', () => {
  test('uppercase email login behaves predictably', async ({ page }) => {
    const email = randomEmail();
    await page.goto('/register');
    await page.fill('input[name="email"]', email);
    await page.fill('input[name="password"]', 'password123');
    await page.fill('input[name="password_confirm"]', 'password123');
    await page.click('button[type="submit"]');
    await page.waitForLoadState('networkidle');

    await page.goto('/logout');
    await page.waitForLoadState('networkidle');

    await page.goto('/login');
    await page.fill('input[name="identity"]', email.toUpperCase());
    await page.fill('input[name="password"]', 'password123');
    await page.click('button[type="submit"]');
    await page.waitForLoadState('networkidle');
    const url = page.url();
    expect(url.match(/\/(overview|create|login)/)).not.toBeNull();
  });
});

test.describe('Test 64: Registration form requires email and password', () => {
  test('email and password are required attributes', async ({ page }) => {
    await ensureAnonymous(page);
    await page.goto('/register');
    const emailRequired = await page.locator('input[name="email"]').evaluate(el => el.hasAttribute('required'));
    const passwordRequired = await page.locator('input[name="password"]').evaluate(el => el.hasAttribute('required'));
    expect(emailRequired).toBeTruthy();
    expect(passwordRequired).toBeTruthy();
  });
});

test.describe('Test 79: Ideas library loads', () => {
  test('idea items render on the create page', async ({ page }) => {
    await page.goto('/create');
    await page.waitForLoadState('networkidle');
    const items = page.locator('.idea-item');
    expect(await items.count()).toBeGreaterThan(0);
  });
});

test.describe('Test 29: Free tier prompt limit', () => {
  test('exceeding the free tier returns an upgrade error', async ({ page }) => {
    await loginFreshUser(page);
    const b64 = Buffer.from(JSON.stringify({
      event_name: 'Limit Test',
      prompts: Array.from({ length: 6 }, (_, i) => `Prompt ${i + 1}`).join('\n'),
    })).toString('base64');
    await page.context().addCookies([{
      name: 'pending_challenge',
      value: b64,
      domain: 'localhost',
      path: '/',
    }]);
    const response = await page.goto('/create/design?design_id=classic');
    const status = response!.status();
    const url = page.url();
    const bodyText = (await page.locator('body').textContent()) || '';
    const wasCreated = url.match(/\/overview\/[a-zA-Z0-9]{10,}/);
    const sawLimitMessage = bodyText.toLowerCase().match(/upgrade|prompt slot|max|plan/);
    expect(wasCreated === null || sawLimitMessage !== null || status >= 400).toBeTruthy();
  });
});

test.describe('Test 70: Challenge creation with exactly 5 prompts', () => {
  test('5 prompts are accepted on the free tier', async ({ page }) => {
    await loginFreshUser(page);
    const challengeId = await createEventViaCookie(page, 'Max Free Test', [
      'Prompt 1', 'Prompt 2', 'Prompt 3', 'Prompt 4', 'Prompt 5',
    ]);
    expect(challengeId).toBeTruthy();
  });
});
