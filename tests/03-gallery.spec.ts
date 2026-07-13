import { test, expect } from '@playwright/test';
import { ensureAnonymous, loginFreshUser, createEventViaCookie, authedRequest, randomEmail } from './helpers';

test.describe('Gallery creation and host dashboard', () => {
  test('authenticated user creates a gallery with one canonical QR', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, "Niclas & Emma's Wedding");
    expect(galleryId).toBeTruthy();
    await expect(page).toHaveURL(/\/overview\//);
    await expect(page.locator('.dashboard-qr img')).toHaveAttribute('src', `/qr-image/${galleryId}`);
  });

  test('anonymous create survives registration and finishes', async ({ page }) => {
    await ensureAnonymous(page);
    await page.goto('/create');
    await page.fill('#eventTitle', 'Anon Party');
    await page.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(/\/register/);

    await page.fill('input[name="email"]', randomEmail());
    await page.fill('input[name="password"]', 'password123');
    await page.fill('input[name="password_confirm"]', 'password123');
    await page.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(/\/overview\/[a-zA-Z0-9]+/);
  });

  test('create finish without pending data creates nothing', async ({ page }) => {
    await loginFreshUser(page);
    await page.context().clearCookies({ name: 'pending_event' });
    await page.goto('/create/finish');
    await expect(page).toHaveURL(/\/create/);
  });

  test('overview unifies QR sharing, uploads, download, and details', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Overview Test');
    await page.goto(`/overview/${galleryId}`);
    await expect(page.locator('h1')).toContainText('Overview Test');
    await expect(page.locator(`a[href="/poster/${galleryId}"]`)).toBeVisible();
    await expect(page.locator(`a[href="/qr-image/${galleryId}"]`)).toBeVisible();
    await expect(page.locator(`a[href="/edit/${galleryId}"]`)).toBeVisible();
    await expect(page.locator('.share-url')).toContainText(`/e/${galleryId}`);
    await expect(page.locator('#uploads')).toBeVisible();
    await expect(page.locator('input[name="design_id"], input[name="qr_mode"]')).toHaveCount(0);
  });

  test('poster endpoint renders the single-QR PDF', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'PDF Test');
    const { status, headers } = await authedRequest(page, `/poster/${galleryId}`);
    expect(status).toBe(200);
    expect(headers['content-type']).toContain('pdf');
    expect(headers['content-disposition']).toContain('qr-poster');
  });

  test('QR endpoint renders the only gallery code', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'QR Test');
    const { status, headers } = await authedRequest(page, `/qr-image/${galleryId}`);
    expect(status).toBe(200);
    expect(headers['content-type']).toContain('image/png');
  });

  test('empty dashboard renders a useful upload state', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Gallery Test');
    await page.goto(`/overview/${galleryId}#uploads`);
    await expect(page.locator('.empty-state')).toBeVisible();
    await expect(page.locator('.empty-state')).toContainText(/first upload/i);
  });

  test('owner ZIP endpoint returns 400 before the first upload', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'ZIP Test');
    const { status } = await authedRequest(page, `/download/${galleryId}`);
    expect(status).toBe(400);
  });
});
