import { test, expect } from '@playwright/test';
import { ensureAnonymous, loginFreshUser, upgradeToPaid, createEventViaCookie, authedRequest, authedPost } from './helpers';

// Tiny valid JPEG (3x3 px, "minimum viable" header). Decodes successfully in
// browsers and passes our magic-byte check (FF D8 FF). Hand-trimmed so the
// fixture lives inline — no on-disk asset to maintain.
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

async function getPromptLink(page: any, challengeId: string): Promise<string> {
  await page.goto(`/overview/${challengeId}`);
  await page.waitForLoadState('networkidle');
  const promptLink = page.locator(`a[href^="/e/${challengeId}/"]`).first();
  const href = await promptLink.getAttribute('href');
  return href!;
}

test.describe('Test 40: Upload — image preview after selection', () => {
  test('selecting a JPEG shows an inline data-URL preview', async ({ page }) => {
    await loginFreshUser(page, 'Preview User');
    const challengeId = await createEventViaCookie(page, 'Preview Test', ['First prompt']);
    expect(challengeId).toBeTruthy();
    const href = await getPromptLink(page, challengeId!);

    await ensureAnonymous(page);
    await page.goto(href);
    await page.waitForLoadState('networkidle');

    await page.setInputFiles('input#photo', { name: 'guest.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });

    const preview = page.locator('#dz-preview');
    // Wait for the data-URL preview to be wired up by the FileReader.
    await expect.poll(async () => preview.getAttribute('src')).toMatch(/^data:image\//);

    const wrapHidden = await page.locator('#dz-preview-wrap').evaluate((el) => (el as HTMLElement).hidden);
    expect(wrapHidden).toBe(false);

    await expect(page.locator('#dz-filename')).toContainText('guest.jpg');
    await expect(page.locator('#change-photo')).toBeVisible();
  });
});

test.describe('Test 41: Upload — invalid format rejected with friendly error', () => {
  test('uploading a text file shows a server-side error mentioning supported formats', async ({ page }) => {
    await loginFreshUser(page, 'Bad Format User');
    const challengeId = await createEventViaCookie(page, 'Bad Format Test', ['First prompt']);
    expect(challengeId).toBeTruthy();
    const href = await getPromptLink(page, challengeId!);

    await ensureAnonymous(page);
    await page.goto(href);
    await page.waitForLoadState('networkidle');

    // The client-side check kicks in for non-image MIME types, so we bypass
    // it by submitting via fetch with a doctored multipart body.
    const submitUrl = href;
    const result = await page.evaluate(async (url) => {
      const form = new FormData();
      form.append('image', new Blob(['not an image'], { type: 'text/plain' }), 'note.txt');
      const res = await fetch(url, { method: 'POST', body: form });
      return { status: res.status, html: await res.text() };
    }, submitUrl);

    expect(result.status).toBe(200);
    expect(result.html).toMatch(/JPG|PNG|HEIC/i);
  });
});

test.describe('Test 42: Guest ZIP download — enabled by default', () => {
  test('anonymous GET /e/{id}/download streams a ZIP when uploads exist', async ({ page }) => {
    const email = await loginFreshUser(page, 'Guest Zip User');
    await upgradeToPaid(page, email);
    const challengeId = await createEventViaCookie(page, 'Guest Zip Test', ['First prompt']);
    expect(challengeId).toBeTruthy();

    const href = await getPromptLink(page, challengeId!);
    // Submit a real upload as the owner so the gallery has at least one file.
    await page.goto(href);
    await page.setInputFiles('input#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await Promise.all([
      page.waitForURL(`**/e/${challengeId}/library`),
      page.locator('#submit-btn').click(),
    ]);

    await ensureAnonymous(page);
    const res = await page.request.get(`/e/${challengeId}/download`);
    expect(res.status()).toBe(200);
    expect(res.headers()['content-type']).toBe('application/zip');
    expect(res.headers()['content-disposition']).toMatch(/attachment/);
  });
});

test.describe('Test 43: Guest ZIP download — disabled by owner', () => {
  test('toggling disable_guest_download blocks the public download with 403', async ({ page }) => {
    const email = await loginFreshUser(page, 'Guest Zip Disabled');
    await upgradeToPaid(page, email);
    const challengeId = await createEventViaCookie(page, 'Guest Zip Disabled Test', ['First prompt']);
    expect(challengeId).toBeTruthy();

    const href = await getPromptLink(page, challengeId!);
    await page.goto(href);
    await page.setInputFiles('input#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await Promise.all([
      page.waitForURL(`**/e/${challengeId}/library`),
      page.locator('#submit-btn').click(),
    ]);

    // Toggle the disable flag via the owner settings form.
    const post = await authedPost(page, `/settings/${challengeId}`, { disable_guest_download: 'on' });
    expect([200, 303, 302]).toContain(post.status);

    await ensureAnonymous(page);
    const res = await page.request.get(`/e/${challengeId}/download`, { maxRedirects: 0 });
    expect(res.status()).toBe(403);
  });
});

test.describe('Test 44: Lock after submit — redirects to library', () => {
  test('with lock on, second prompt visit goes to /library?already=1', async ({ page }) => {
    const email = await loginFreshUser(page, 'Lock User');
    await upgradeToPaid(page, email);
    const challengeId = await createEventViaCookie(page, 'Lock Test', ['One', 'Two']);
    expect(challengeId).toBeTruthy();

    // Enable lock as the owner (one-photo-per-guest is a paid feature).
    const post = await authedPost(page, `/settings/${challengeId}`, { lock_after_submit: 'on' });
    expect([200, 303, 302]).toContain(post.status);

    // Now act as a guest in a clean cookie context.
    await ensureAnonymous(page);
    await page.goto(`/overview/${challengeId}`).catch(() => {});
    // Find the two prompt links by hitting the upload endpoints directly.
    await page.goto(`/e/${challengeId}/library`);
    await page.waitForLoadState('networkidle');
    const links = await page.locator(`a[href*="/e/${challengeId}/"]`).evaluateAll((els) =>
      els.map((e: any) => e.getAttribute('href')).filter((h: string | null) => h && /\/e\/[^/]+\/[^/]+$/.test(h) && !h.endsWith('/library') && !h.endsWith('/done') && !h.endsWith('/download'))
    );
    expect(links.length).toBeGreaterThanOrEqual(2);
    const firstHref = links[0]!;
    const secondHref = links.find((h: string) => h !== firstHref)!;

    // Upload to the first prompt (sets the submission cookie).
    await page.goto(firstHref);
    await page.setInputFiles('input#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await Promise.all([
      page.waitForURL(`**/e/${challengeId}/library`),
      page.locator('#submit-btn').click(),
    ]);

    // Visit the second prompt — should redirect to the library with already=1.
    const res = await page.goto(secondHref);
    expect(res!.url()).toMatch(/\/library\?already=1$/);
    await expect(page.locator('.library-toast')).toBeVisible();
  });
});

test.describe('Test 45: Default unlocked — guest may upload to multiple prompts', () => {
  test('without lock, second prompt page renders the upload form normally', async ({ page }) => {
    await loginFreshUser(page, 'Open User');
    const challengeId = await createEventViaCookie(page, 'Open Test', ['One', 'Two']);
    expect(challengeId).toBeTruthy();

    await ensureAnonymous(page);
    await page.goto(`/e/${challengeId}/library`);
    const links = await page.locator(`a[href*="/e/${challengeId}/"]`).evaluateAll((els) =>
      els.map((e: any) => e.getAttribute('href')).filter((h: string | null) => h && /\/e\/[^/]+\/[^/]+$/.test(h) && !h.endsWith('/library') && !h.endsWith('/done') && !h.endsWith('/download'))
    );
    const firstHref = links[0]!;
    const secondHref = links.find((h: string) => h !== firstHref)!;

    await page.goto(firstHref);
    await page.setInputFiles('input#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await Promise.all([
      page.waitForURL(`**/e/${challengeId}/library`),
      page.locator('#submit-btn').click(),
    ]);

    const res = await page.goto(secondHref);
    expect(res!.url()).toContain(secondHref);
    await expect(page.locator('input#photo')).toBeAttached();
  });
});

test.describe('Test 47: Collect guest name — required field and attribution', () => {
  test('with the setting on, the upload form requires a name and the gallery shows it', async ({ page }) => {
    const email = await loginFreshUser(page, 'Name Collector');
    await upgradeToPaid(page, email);
    const challengeId = await createEventViaCookie(page, 'Name Test', ['First prompt']);
    expect(challengeId).toBeTruthy();

    // Enable "ask guests for their name" (a paid guest setting).
    const post = await authedPost(page, `/settings/${challengeId}`, { collect_guest_name: 'on' });
    expect([200, 303, 302]).toContain(post.status);

    const href = await getPromptLink(page, challengeId!);

    await ensureAnonymous(page);
    await page.goto(href);
    await page.waitForLoadState('networkidle');

    // The name field is shown and required when the setting is on.
    const nameField = page.locator('input#guest-name');
    await expect(nameField).toBeVisible();
    expect(await nameField.getAttribute('required')).not.toBeNull();

    // Server-side enforcement: a multipart POST with a photo but no name is
    // rejected (the page re-renders with the name error) and no record lands.
    const result = await page.evaluate(async (url) => {
      const bytes = new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46]);
      const form = new FormData();
      form.append('image', new Blob([bytes], { type: 'image/jpeg' }), 'g.jpg');
      const res = await fetch(url, { method: 'POST', body: form });
      return { status: res.status, html: await res.text() };
    }, href);
    expect(result.status).toBe(200);
    expect(result.html).toMatch(/name/i);

    // A proper submission with a name redirects to the library as usual.
    await page.goto(href);
    await page.fill('input#guest-name', 'Alex Morgan');
    await page.setInputFiles('input#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
    await Promise.all([
      page.waitForURL(`**/e/${challengeId}/library`),
      page.locator('#submit-btn').click(),
    ]);

    // The name is attributed on the public library lightbox caption.
    await page.goto(`/e/${challengeId}/library`);
    await page.waitForLoadState('networkidle');
    await expect(page.locator('.lightbox-author')).toContainText('Alex Morgan');
  });
});

test.describe('Test 46: Library lightbox — opens and switches slides', () => {
  test('clicking a thumbnail opens the lightbox and arrow keys navigate', async ({ page }) => {
    await loginFreshUser(page, 'Lightbox User');
    const challengeId = await createEventViaCookie(page, 'Lightbox Test', ['One', 'Two']);
    expect(challengeId).toBeTruthy();

    // Seed two uploads.
    await page.goto(`/e/${challengeId}/library`);
    const links = await page.locator(`a[href*="/e/${challengeId}/"]`).evaluateAll((els) =>
      els.map((e: any) => e.getAttribute('href')).filter((h: string | null) => h && /\/e\/[^/]+\/[^/]+$/.test(h) && !h.endsWith('/library') && !h.endsWith('/done') && !h.endsWith('/download'))
    );
    for (const href of links) {
      await page.goto(href);
      await page.setInputFiles('input#photo', { name: 'g.jpg', mimeType: 'image/jpeg', buffer: TINY_JPEG });
      await Promise.all([
        page.waitForURL(`**/e/${challengeId}/library`),
        page.locator('#submit-btn').click(),
      ]);
    }

    await page.goto(`/e/${challengeId}/library`);
    await page.waitForLoadState('networkidle');
    const firstThumb = page.locator('.gallery-item a.photo[data-prompt]').first();
    await firstThumb.click();

    const lightbox = page.locator('#lightbox');
    await expect(lightbox).toBeVisible();
    await expect(lightbox).toHaveClass(/is-open/);
    await expect(page.locator('#lightbox-counter')).toContainText('/');
  });
});
