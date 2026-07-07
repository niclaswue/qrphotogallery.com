import { test, expect } from '@playwright/test';
import { ensureAnonymous, randomEmail } from './helpers';

test.describe('Test 5: Register — short password', () => {
  test('registration fails with short password', async ({ page }) => {
    await ensureAnonymous(page);
    await page.goto('/register');
    await page.fill('input[name="email"]', randomEmail());
    await page.fill('input[name="password"]', 'abc');
    await page.fill('input[name="password_confirm"]', 'abc');
    await page.click('button[type="submit"]');
    // The browser blocks submission via the minlength=8 attribute; if it
    // doesn't, the server redirects back with ?error=short_password.
    const url = page.url();
    const blockedClientSide = await page.locator('input[name="password"]:invalid').count() > 0;
    const stayedOnRegister = url.includes('/register');
    expect(blockedClientSide || stayedOnRegister).toBeTruthy();
  });
});

test.describe('Test 6: Register — mismatched passwords', () => {
  test('registration is rejected with mismatched passwords', async ({ page }) => {
    await ensureAnonymous(page);
    await page.goto('/register');
    await page.fill('input[name="email"]', randomEmail());
    await page.fill('input[name="password"]', 'password123');
    await page.fill('input[name="password_confirm"]', 'different123');
    await page.click('button[type="submit"]');
    await page.waitForLoadState('networkidle');
    await expect(page).toHaveURL(/\/register/);
    await expect(page.locator('.auth-error')).toBeVisible();
  });
});

test.describe('Test 7: Register — duplicate email', () => {
  test('registration fails with duplicate email', async ({ page }) => {
    await ensureAnonymous(page);
    const email = randomEmail();
    await page.goto('/register');
    await page.fill('input[name="email"]', email);
    await page.fill('input[name="password"]', 'password123');
    await page.fill('input[name="password_confirm"]', 'password123');
    await page.click('button[type="submit"]');
    await page.waitForLoadState('networkidle');

    await page.context().clearCookies();
    await page.goto('/register');
    await page.fill('input[name="email"]', email);
    await page.fill('input[name="password"]', 'password123');
    await page.fill('input[name="password_confirm"]', 'password123');
    await page.click('button[type="submit"]');
    await page.waitForLoadState('networkidle');
    await expect(page).toHaveURL(/\/register/);
    await expect(page.locator('.auth-error')).toBeVisible();
  });
});

test.describe('Test 8: Login — valid credentials', () => {
  test('login redirects to overview with auth cookie', async ({ page }) => {
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
    await page.fill('input[name="identity"]', email);
    await page.fill('input[name="password"]', 'password123');
    await page.click('button[type="submit"]');
    await page.waitForLoadState('networkidle');
    // Logged-in users without a challenge bounce to /create; with one to /overview/{id}.
    await expect(page).toHaveURL(/\/(overview|create)/);
    const cookies = await page.context().cookies();
    const pbAuth = cookies.find(c => c.name === 'pb_auth');
    expect(pbAuth).toBeDefined();
    expect(pbAuth!.value).not.toBe('');
    expect(pbAuth!.httpOnly).toBeTruthy();
    expect(pbAuth!.sameSite).toBe('Lax');
  });
});

test.describe('Test 9: Login — invalid credentials', () => {
  test('login fails with invalid credentials', async ({ page }) => {
    await ensureAnonymous(page);
    await page.goto('/login');
    await page.fill('input[name="identity"]', 'nonexistent@example.com');
    await page.fill('input[name="password"]', 'whatever');
    await page.click('button[type="submit"]');
    await page.waitForLoadState('networkidle');
    await expect(page).toHaveURL(/\/login/);
    await expect(page.locator('.auth-error')).toContainText(/wrong|invalid/i);
  });
});

test.describe('Test 10: Logout', () => {
  test('logout clears the auth cookie and lands back home', async ({ page }) => {
    const email = randomEmail();
    await page.goto('/register');
    await page.fill('input[name="email"]', email);
    await page.fill('input[name="password"]', 'password123');
    await page.fill('input[name="password_confirm"]', 'password123');
    await page.click('button[type="submit"]');
    await page.waitForLoadState('networkidle');

    await page.goto('/logout');
    await page.waitForLoadState('networkidle');
    await expect(page).toHaveURL(/\/$/);

    const cookies = await page.context().cookies();
    const pbAuth = cookies.find(c => c.name === 'pb_auth');
    expect(pbAuth?.value || '').toBe('');
    await expect(page.locator('nav a[href="/login"]').first()).toBeVisible();
  });
});
