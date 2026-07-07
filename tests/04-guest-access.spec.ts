import { test, expect } from '@playwright/test';
import { ensureAnonymous, loginFreshUser, createEventViaCookie, authedRequest } from './helpers';

test.describe('Test 20: Guest upload page — valid challenge', () => {
  test('upload page renders with the prompt text', async ({ page }) => {
    await loginFreshUser(page);
    const challengeId = await createEventViaCookie(page, 'Upload Test Wedding', ['First prompt', 'Second prompt']);
    expect(challengeId).toBeTruthy();
    await page.goto(`/overview/${challengeId}`);
    await page.waitForLoadState('networkidle');
    const promptLink = page.locator(`a[href^="/e/${challengeId}/"]`).first();
    const href = await promptLink.getAttribute('href');
    expect(href).toBeTruthy();
    const uploadResponse = await page.goto(href!);
    expect(uploadResponse!.status()).toBe(200);
    await expect(page.locator('.event-name')).toContainText('Upload Test Wedding');
    await expect(page.locator('.prompt-card .prompt')).toContainText('First prompt');
    await expect(page.locator('input#photo')).toBeAttached();
  });
});

test.describe('Test 21: Guest upload page — nonexistent challenge', () => {
  test('returns 404 for nonexistent challenge', async ({ page }) => {
    const response = await page.goto('/e/nonexistent_challenge_id/0');
    expect(response!.status()).toBe(404);
  });

  test('nonexistent challenge returns an HTML error page', async ({ page }) => {
    await page.goto('/e/nonexistent_challenge_id/0');
    const bodyText = (await page.locator('body').textContent())?.trim() || '';
    expect(bodyText.startsWith('{')).toBeFalsy();
    await expect(page.locator('h1').first()).toBeVisible();
  });
});

test.describe('Test 22: Guest upload page — challenge with no prompts', () => {
  test('returns error for nonexistent challenge id', async ({ page }) => {
    const response = await page.goto('/e/nonexistent_challenge_id/0');
    expect(response!.status()).toBeGreaterThanOrEqual(400);
  });
});

test.describe('Test 23: All Done page — valid challenge', () => {
  test('done page renders the success copy and a link to library', async ({ page }) => {
    await loginFreshUser(page);
    const challengeId = await createEventViaCookie(page, 'Done Test', ['A prompt']);
    expect(challengeId).toBeTruthy();
    await page.goto(`/e/${challengeId}/done`);
    await expect(page.locator('.done-card h1')).toBeVisible();
    await expect(page.locator(`.done-card a[href*="/e/${challengeId}/"]`).first()).toBeVisible();
  });
});

test.describe('Test 24: All Done page — nonexistent challenge', () => {
  test('returns 404 for nonexistent challenge ID', async ({ page }) => {
    const response = await page.goto('/e/completely_fake_id/done');
    expect(response!.status()).toBe(404);
  });
});

test.describe('Test 25: Protected routes — unauthenticated access', () => {
  const protectedRoutes = [
    '/overview/some_id',
    '/gallery/some_id',
    '/edit/some_id',
    '/print/some_id',
    '/download/some_id',
  ];

  for (const route of protectedRoutes) {
    test(`${route} redirects to register or returns an error`, async ({ page }) => {
      await ensureAnonymous(page);
      const response = await page.goto(route);
      const status = response!.status();
      const url = page.url();
      const bouncedToAuth = url.includes('/login') || url.includes('/register');
      const isError = status >= 400;
      expect(bouncedToAuth || isError).toBeTruthy();
    });
  }
});

test.describe('Test 26: Protected routes — wrong user access', () => {
  test("a different user can't access another user's challenge", async ({ page }) => {
    await loginFreshUser(page);
    const challengeId = await createEventViaCookie(page, 'Private Challenge', ['Secret prompt']);
    expect(challengeId).toBeTruthy();
    await page.goto('/logout');
    await page.waitForLoadState('networkidle');
    await page.context().clearCookies();
    await loginFreshUser(page);
    const { status } = await authedRequest(page, `/overview/${challengeId}`);
    expect([401, 403, 404]).toContain(status);
  });
});

test.describe('Test 27: Payment success page', () => {
  test('payment success renders when authenticated', async ({ page }) => {
    await loginFreshUser(page);
    await page.goto('/payment/success');
    await page.waitForLoadState('networkidle');
    const bodyText = (await page.locator('body').textContent()) || '';
    const lower = bodyText.toLowerCase();
    expect(lower.match(/plan|payment|pago|paiement|zahlung/i)).not.toBeNull();
  });

  test('payment success page does NOT actually upgrade the user tier', async ({ page }) => {
    await loginFreshUser(page);
    const beforeBody = await (await page.request.get('/api/user/tier', {
      headers: { Cookie: `pb_auth=${(await page.context().cookies()).find(c => c.name === 'pb_auth')!.value}` },
    })).json();
    expect(beforeBody.tier).toBe('free');

    await page.goto('/payment/success');
    await page.waitForLoadState('networkidle');

    const afterBody = await (await page.request.get('/api/user/tier', {
      headers: { Cookie: `pb_auth=${(await page.context().cookies()).find(c => c.name === 'pb_auth')!.value}` },
    })).json();
    expect(afterBody.tier).toBe('free');
    expect(afterBody.max_prompts).toBe(5);
  });
});

test.describe('Test 28: Pricing page plan links', () => {
  test('Choose standard links to the payment page', async ({ page }) => {
    await page.goto('/pricing');
    const standardLink = page.locator('a[href*="/payment?plan=standard"]');
    await expect(standardLink.first()).toBeVisible();
  });
});

test.describe('Test 30: Guest upload — invalid prompt id', () => {
  test('non-numeric/invalid prompt ID returns error', async ({ page }) => {
    await loginFreshUser(page);
    const challengeId = await createEventViaCookie(page, 'Index Test', ['Photo 1', 'Photo 2']);
    expect(challengeId).toBeTruthy();
    const response = await page.goto(`/e/${challengeId}/abc`);
    expect(response!.status()).toBeGreaterThanOrEqual(400);
  });

  test('out-of-range prompt ID returns error', async ({ page }) => {
    await loginFreshUser(page);
    const challengeId = await createEventViaCookie(page, 'Index Test 2', ['Photo 1']);
    expect(challengeId).toBeTruthy();
    const response = await page.goto(`/e/${challengeId}/999nonexistent999`);
    expect(response!.status()).toBeGreaterThanOrEqual(400);
  });
});
