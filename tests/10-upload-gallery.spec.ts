import { test, expect } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { ensureAnonymous, loginFreshUser, upgradeToPaid, createEventViaCookie, authedPost } from './helpers';

// A real decodable image catches broken /api/files paths; the previous
// three-pixel pseudo-JPEG was accepted by the server but not by browsers.
const TINY_JPEG = readFileSync(resolve(process.cwd(), '../pb_public/static/img/og-default.jpg'));

const TINY_MP4 = Buffer.from([
  0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70, 0x69, 0x73, 0x6f, 0x6d,
  0x00, 0x00, 0x00, 0x00, 0x69, 0x73, 0x6f, 0x6d,
]);

async function submitSelectedFiles(page: any, galleryId: string) {
  await Promise.all([
    page.waitForURL((url: URL) => url.pathname === `/e/${galleryId}` && url.searchParams.get('uploaded') === '1'),
    page.locator('#submit-btn').click(),
  ]);
}

test.describe('Guest uploads and shared gallery', () => {
  test('selecting files renders a batch summary and enables upload', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Preview Test');
    await ensureAnonymous(page);
    await page.goto(`/e/${galleryId}`);
    await page.setInputFiles('#photo', { name: 'guest.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await expect(page.locator('.selected-file')).toContainText('guest.jpg');
    await expect(page.locator('#dz-filename')).toContainText('1 file selected');
    await expect(page.locator('#submit-btn')).toBeEnabled();
  });

  test('server rejects a renamed non-media file', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Bad Format Test');
    await ensureAnonymous(page);
    await page.goto(`/e/${galleryId}`);
    const formAction = await page.locator('#upload-form').getAttribute('action');
    const result = await page.evaluate(async (url) => {
      const form = new FormData();
      form.append('image', new Blob(['not media'], { type: 'text/plain' }), 'malware.mp4');
      const res = await fetch(url!, { method: 'POST', body: form });
      return { status: res.status, html: await res.text() };
    }, formAction);
    expect(result.status).toBe(200);
    expect(result.html).toMatch(/supported photo or video format/i);
  });

  test('photo upload returns to the same page and appears in the flat gallery', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Photo Upload Test');
    await ensureAnonymous(page);
    await page.goto(`/e/${galleryId}`);
    await page.setInputFiles('#photo', { name: 'guest.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await submitSelectedFiles(page, galleryId!);
    await expect(page.locator('.success-toast')).toBeVisible();
    await expect(page.locator('.media-tile img')).toHaveCount(1);
    await expect.poll(() => page.locator('.media-tile img').evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth > 0)).toBe(true);
    await expect(page.locator('#upload-form')).toBeVisible();
  });

  test('free preview enforces one total file rather than one file per task', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Free Preview Limit');
    await page.goto(`/e/${galleryId}`);
    await page.setInputFiles('#photo', { name: 'one.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await submitSelectedFiles(page, galleryId!);
    await page.setInputFiles('#photo', { name: 'two.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await page.locator('#submit-btn').click();
    await expect(page.locator('.upload-error:not([hidden])')).toContainText(/free preview/i);
    await expect(page.locator('.media-tile')).toHaveCount(1);
  });

  test('common MP4 container is accepted and rendered as video', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Video Upload Test');
    await page.goto(`/e/${galleryId}`);
    await page.setInputFiles('#photo', { name: 'clip.mp4', mimeType: 'video/mp4', buffer: TINY_MP4 });
    await submitSelectedFiles(page, galleryId!);
    await expect(page.locator('.media-tile video')).toHaveCount(1);
    await expect(page.locator('.video-badge')).toBeVisible();
  });

  test('paid guest ZIP download streams originals', async ({ page }) => {
    const email = await loginFreshUser(page);
    await upgradeToPaid(page, email, 'standard');
    const galleryId = await createEventViaCookie(page, 'Guest Zip Test');
    await page.goto(`/e/${galleryId}`);
    await page.setInputFiles('#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await submitSelectedFiles(page, galleryId!);
    await ensureAnonymous(page);
    const response = await page.request.get(`/e/${galleryId}/download`);
    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toBe('application/zip');
  });

  test('Commercial host can disable public ZIP downloads', async ({ page }) => {
    const email = await loginFreshUser(page);
    await upgradeToPaid(page, email);
    const galleryId = await createEventViaCookie(page, 'Guest Zip Disabled');
    await page.goto(`/e/${galleryId}`);
    await page.setInputFiles('#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await submitSelectedFiles(page, galleryId!);
    expect([200, 302, 303]).toContain((await authedPost(page, `/settings/${galleryId}`, { disable_guest_download: 'on' })).status);
    await ensureAnonymous(page);
    expect((await page.request.get(`/e/${galleryId}/download`, { maxRedirects: 0 })).status()).toBe(403);
  });

  test('Personal gallery accepts repeat and batch uploads', async ({ page }) => {
    const email = await loginFreshUser(page);
    await upgradeToPaid(page, email, 'standard');
    const galleryId = await createEventViaCookie(page, 'Open Test');
    await page.goto(`/e/${galleryId}`);
    await page.setInputFiles('#photo', [
      { name: 'one.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG },
      { name: 'two.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG },
    ]);
    await submitSelectedFiles(page, galleryId!);
    await expect(page.locator('.media-tile')).toHaveCount(2);
    await expect(page.locator('#photo')).toBeAttached();
  });

  test('Commercial name collection is server-required and attributed', async ({ page }) => {
    const email = await loginFreshUser(page);
    await upgradeToPaid(page, email);
    const galleryId = await createEventViaCookie(page, 'Name Test');
    expect([200, 302, 303]).toContain((await authedPost(page, `/settings/${galleryId}`, { collect_guest_name: 'on' })).status);
    await ensureAnonymous(page);
    await page.goto(`/e/${galleryId}`);
    await expect(page.locator('#guest-name')).toBeVisible();
    const formAction = await page.locator('#upload-form').getAttribute('action');
    const missingName = await page.evaluate(async (url) => {
      const form = new FormData();
      form.append('image', new Blob([new Uint8Array([0xff, 0xd8, 0xff])], { type: 'image/jpeg' }), 'g.jpg');
      return await (await fetch(url!, { method: 'POST', body: form })).text();
    }, formAction);
    expect(missingName).toMatch(/enter your name/i);

    await page.fill('#guest-name', 'Alex Morgan');
    await page.setInputFiles('#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await submitSelectedFiles(page, galleryId!);
    await page.locator('.media-tile').click();
    await expect(page.locator('.lightbox-author')).toContainText('Alex Morgan');
  });

  test('batch uploads open in the accessible lightbox and host overview', async ({ page }) => {
    const email = await loginFreshUser(page);
    await upgradeToPaid(page, email);
    const galleryId = await createEventViaCookie(page, 'Lightbox Test');
    await page.goto(`/e/${galleryId}`);
    await page.setInputFiles('#photo', [
      { name: 'one.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG },
      { name: 'two.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG },
    ]);
    await submitSelectedFiles(page, galleryId!);
    await page.locator('.media-tile').first().click();
    await expect(page.locator('#lightbox')).toBeVisible();
    await expect(page.locator('#lightbox-counter')).toContainText('1 / 2');
    await page.keyboard.press('ArrowRight');
    await expect(page.locator('#lightbox-counter')).toContainText('2 / 2');
    await page.keyboard.press('Escape');
    await page.goto(`/overview/${galleryId}#uploads`);
    await expect(page.locator('.owner-media-card')).toHaveCount(2);
  });
});
