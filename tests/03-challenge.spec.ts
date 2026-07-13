import { test, expect } from '@playwright/test';
import { ensureAnonymous, loginFreshUser, createEventViaCookie, authedRequest, randomEmail } from './helpers';

test.describe('Gallery creation and host dashboard', () => {
  test('authenticated user creates a one-QR gallery', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, "Niclas & Emma's Wedding", [
      'Photos and videos from our event',
    ], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await expect(page).toHaveURL(/\/overview\//);
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
    expect(page.url()).not.toMatch(/\/overview\/[a-zA-Z0-9]{10,}/);
  });

  test('overview shows QR sharing, gallery, download, and edit actions', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Overview Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await page.goto(`/overview/${galleryId}`);
    await expect(page.locator('h1')).toContainText('Overview Test');
    await expect(page.locator(`a[href="/poster/${galleryId}"]`)).toBeVisible();
    await expect(page.locator(`a[href="/gallery/${galleryId}"]`)).toBeVisible();
    await expect(page.locator(`a[href="/download/${galleryId}"]`)).toBeVisible();
    await expect(page.locator(`a[href="/edit/${galleryId}"]`)).toBeVisible();
    await expect(page.locator('.share-url')).toContainText(`/e/${galleryId}`);
  });

  test('poster endpoint renders a PDF', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'PDF Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    const { status, headers } = await authedRequest(page, `/poster/${galleryId}`);
    expect(status).toBe(200);
    expect(headers['content-type']).toContain('pdf');
    expect(headers['content-disposition']).toContain('poster');
  });

  test('empty owner gallery renders a useful state', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Gallery Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await page.goto(`/gallery/${galleryId}`);
    await expect(page.locator('h1')).toContainText('Gallery Test');
    await expect(page.locator('.empty-state')).toBeVisible();
  });

  test('owner ZIP endpoint returns 400 before the first upload', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'ZIP Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    const { status } = await authedRequest(page, `/download/${galleryId}`);
    expect(status).toBe(400);
  });
});
