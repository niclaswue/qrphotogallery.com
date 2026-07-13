import { test, expect } from '@playwright/test';
import { ensureAnonymous, loginFreshUser, upgradeToPaid, createEventViaCookie, authedPost } from './helpers';

const TINY_JPEG = Buffer.from([
  0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
  0x00, 0x01, 0x00, 0x00, 0xff, 0xdb, 0x00, 0x43, 0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08,
  0x07, 0x07, 0x07, 0x09, 0x09, 0x08, 0x0a, 0x0c, 0x14, 0x0d, 0x0c, 0x0b, 0x0b, 0x0c, 0x19, 0x12,
  0x13, 0x0f, 0x14, 0x1d, 0x1a, 0x1f, 0x1e, 0x1d, 0x1a, 0x1c, 0x1c, 0x20, 0x24, 0x2e, 0x27, 0x20,
  0x22, 0x2c, 0x23, 0x1c, 0x1c, 0x28, 0x37, 0x29, 0x2c, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1f, 0x27,
  0x39, 0x3d, 0x38, 0x32, 0x3c, 0x2e, 0x33, 0x34, 0x32, 0xff, 0xc0, 0x00, 0x0b, 0x08, 0x00, 0x03,
  0x00, 0x03, 0x01, 0x01, 0x11, 0x00, 0xff, 0xc4, 0x00, 0x1f, 0x00, 0x00, 0x01, 0x05, 0x01, 0x01,
  0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04,
  0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0xff, 0xc4, 0x00, 0xb5, 0x10, 0x00, 0x02, 0x01, 0x03,
  0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7d, 0x01, 0x02, 0x03, 0x00,
  0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32,
  0x81, 0x91, 0xa1, 0x08, 0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0, 0x24, 0x33, 0x62, 0x72,
  0x82, 0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3f, 0x00, 0xfb, 0xd0, 0xff, 0xd9,
]);

const TINY_MP4 = Buffer.from([
  0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70, 0x69, 0x73, 0x6f, 0x6d,
  0x00, 0x00, 0x00, 0x00, 0x69, 0x73, 0x6f, 0x6d,
]);

const uploadPath = (galleryId: string) => `/e/${galleryId}`;

async function submitSelectedFiles(page: any, galleryId: string) {
  await Promise.all([
    page.waitForURL(`**/e/${galleryId}/done?mode=qr`),
    page.locator('#submit-btn').click(),
  ]);
}

test.describe('Guest uploads and shared gallery', () => {
  test('selecting files renders a batch summary and enables upload', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Preview Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await ensureAnonymous(page);
    await page.goto(uploadPath(galleryId!));
    await page.setInputFiles('#photo', { name: 'guest.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await expect(page.locator('.selected-file')).toContainText('guest.jpg');
    await expect(page.locator('#dz-filename')).toContainText('1 file selected');
    await expect(page.locator('#submit-btn')).toBeEnabled();
    await expect(page.locator('#change-photo')).toBeVisible();
  });

  test('server rejects a renamed non-media file', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Bad Format Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await ensureAnonymous(page);
    await page.goto(uploadPath(galleryId!));
    const formAction = await page.locator('#upload-form').getAttribute('action');
    expect(formAction).toBeTruthy();
    const result = await page.evaluate(async (url) => {
      const form = new FormData();
      form.append('mode', 'qr');
      form.append('image', new Blob(['not media'], { type: 'text/plain' }), 'malware.mp4');
      const res = await fetch(url, { method: 'POST', body: form });
      return { status: res.status, html: await res.text() };
    }, formAction!);
    expect(result.status).toBe(200);
    expect(result.html).toMatch(/supported photo or video format/i);
  });

  test('photo upload reaches done page and appears in the public gallery', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Photo Upload Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await ensureAnonymous(page);
    await page.goto(uploadPath(galleryId!));
    await page.setInputFiles('#photo', { name: 'guest.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await submitSelectedFiles(page, galleryId!);
    await page.goto(`/e/${galleryId}/library`);
    await expect(page.locator('.media-tile img')).toHaveCount(1);
  });

  test('common MP4 container is accepted and rendered as video', async ({ page }) => {
    await loginFreshUser(page);
    const galleryId = await createEventViaCookie(page, 'Video Upload Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await ensureAnonymous(page);
    await page.goto(uploadPath(galleryId!));
    await page.setInputFiles('#photo', { name: 'clip.mp4', mimeType: 'video/mp4', buffer: TINY_MP4 });
    await submitSelectedFiles(page, galleryId!);
    await page.goto(`/e/${galleryId}/library`);
    await expect(page.locator('.media-tile video')).toHaveCount(1);
    await expect(page.locator('.video-badge')).toBeVisible();
  });

  test('paid guest ZIP download streams originals', async ({ page }) => {
    const email = await loginFreshUser(page);
    await upgradeToPaid(page, email);
    const galleryId = await createEventViaCookie(page, 'Guest Zip Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await page.goto(uploadPath(galleryId!));
    await page.setInputFiles('#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await submitSelectedFiles(page, galleryId!);
    await ensureAnonymous(page);
    const response = await page.request.get(`/e/${galleryId}/download`);
    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toBe('application/zip');
    expect(response.headers()['content-disposition']).toMatch(/attachment/);
  });

  test('Commercial host can disable public ZIP downloads', async ({ page }) => {
    const email = await loginFreshUser(page);
    await upgradeToPaid(page, email);
    const galleryId = await createEventViaCookie(page, 'Guest Zip Disabled', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await page.goto(uploadPath(galleryId!));
    await page.setInputFiles('#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await submitSelectedFiles(page, galleryId!);
    expect([200, 302, 303]).toContain((await authedPost(page, `/settings/${galleryId}`, { disable_guest_download: 'on' })).status);
    await ensureAnonymous(page);
    expect((await page.request.get(`/e/${galleryId}/download`, { maxRedirects: 0 })).status()).toBe(403);
  });

  test('one-upload lock redirects a returning guest to the gallery', async ({ page }) => {
    const email = await loginFreshUser(page);
    await upgradeToPaid(page, email);
    const galleryId = await createEventViaCookie(page, 'Lock Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    expect([200, 302, 303]).toContain((await authedPost(page, `/settings/${galleryId}`, { lock_after_submit: 'on' })).status);
    await ensureAnonymous(page);
    await page.goto(uploadPath(galleryId!));
    await page.setInputFiles('#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await submitSelectedFiles(page, galleryId!);
    await page.goto(uploadPath(galleryId!));
    await expect(page).toHaveURL(new RegExp(`/e/${galleryId}/library\\?already=1$`));
    await expect(page.locator('.library-toast')).toBeVisible();
  });

  test('paid unlocked gallery accepts another visit to the uploader', async ({ page }) => {
    const email = await loginFreshUser(page);
    await upgradeToPaid(page, email);
    const galleryId = await createEventViaCookie(page, 'Open Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await ensureAnonymous(page);
    await page.goto(uploadPath(galleryId!));
    await page.setInputFiles('#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await submitSelectedFiles(page, galleryId!);
    await page.goto(uploadPath(galleryId!));
    await expect(page.locator('#photo')).toBeAttached();
  });

  test('Commercial name collection is server-required and attributed', async ({ page }) => {
    const email = await loginFreshUser(page);
    await upgradeToPaid(page, email);
    const galleryId = await createEventViaCookie(page, 'Name Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    expect([200, 302, 303]).toContain((await authedPost(page, `/settings/${galleryId}`, { collect_guest_name: 'on' })).status);
    await ensureAnonymous(page);
    await page.goto(uploadPath(galleryId!));
    await expect(page.locator('#guest-name')).toBeVisible();
    const formAction = await page.locator('#upload-form').getAttribute('action');
    expect(formAction).toBeTruthy();

    const missingName = await page.evaluate(async (url) => {
      const form = new FormData();
      form.append('mode', 'qr');
      form.append('image', new Blob([new Uint8Array([0xff, 0xd8, 0xff])], { type: 'image/jpeg' }), 'g.jpg');
      const res = await fetch(url, { method: 'POST', body: form });
      return await res.text();
    }, formAction!);
    expect(missingName).toMatch(/enter your name/i);

    await page.fill('#guest-name', 'Alex Morgan');
    await page.setInputFiles('#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await submitSelectedFiles(page, galleryId!);
    await page.goto(`/e/${galleryId}/library`);
    await expect(page.locator('.lightbox-author')).toContainText('Alex Morgan');
  });

  test('batch uploads open in the accessible lightbox', async ({ page }) => {
    const email = await loginFreshUser(page);
    await upgradeToPaid(page, email);
    const galleryId = await createEventViaCookie(page, 'Lightbox Test', ['Gallery uploads'], 'classic', 'single');
    expect(galleryId).toBeTruthy();
    await page.goto(uploadPath(galleryId!));
    await page.setInputFiles('#photo', [
      { name: 'one.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG },
      { name: 'two.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG },
    ]);
    await submitSelectedFiles(page, galleryId!);
    await page.goto(`/e/${galleryId}/library`);
    await expect(page.locator('.media-tile')).toHaveCount(2);
    await page.locator('.media-tile').first().click();
    const lightbox = page.locator('#lightbox');
    await expect(lightbox).toBeVisible();
    await expect(lightbox).toHaveAttribute('role', 'dialog');
    await expect(page.locator('#lightbox-counter')).toContainText('1 / 2');
    await page.keyboard.press('ArrowRight');
    await expect(page.locator('#lightbox-counter')).toContainText('2 / 2');
  });
});
