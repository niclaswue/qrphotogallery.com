import { test, expect } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { ensureAnonymous } from './helpers';

const DEMO_JPEG = readFileSync(resolve(process.cwd(), '../pb_public/static/img/og-default.jpg'));

test.describe('Landing live gallery demo', () => {
  test.beforeEach(async ({ page }) => {
    await ensureAnonymous(page);
  });

  test('connects the phone, publishes a sample live, and finishes with a gentle checkout handoff', async ({ page, context }) => {
    await page.goto('/');

    const demo = page.locator('[data-live-demo]');
    const ready = demo.locator('[data-demo-panel="ready"]');
    await expect(ready).toBeVisible();
    await expect(ready.locator('[data-demo-qr]')).toHaveJSProperty('complete', true);

    const demoPath = await ready.locator('[data-demo-link]').getAttribute('href');
    expect(demoPath).toMatch(/^\/demo\/[a-zA-Z0-9]+\?lang=en$/);
    const qrPath = await ready.locator('[data-demo-qr]').getAttribute('src');
    expect(qrPath).toMatch(/^\/demo\/[a-zA-Z0-9]+\/qr\.png\?lang=en$/);

    const qrResponse = await page.request.get(qrPath!);
    expect(qrResponse.status()).toBe(200);
    expect(qrResponse.headers()['content-type']).toContain('image/png');

    const phone = await context.newPage();
    const phoneResponse = await phone.goto(demoPath!);
    expect(phoneResponse!.status()).toBe(200);
    await expect(phone.locator('#demo-upload-card')).toBeVisible();
    expect(await phone.locator('body').innerHTML()).not.toMatch(/cookie-consent|htmx|posthog/i);

    await expect(demo.locator('[data-demo-panel="scanned"]')).toBeVisible();
    await phone.locator('#demo-sample').click();
    await expect(phone.locator('#demo-success')).toBeVisible();

    const photoPanel = demo.locator('[data-demo-panel="photo"]');
    await expect(photoPanel).toBeVisible();
    await expect(photoPanel.locator('[data-demo-image]')).toHaveJSProperty('complete', true);
    await photoPanel.locator('[data-demo-photo]').click();
    await expect(demo.locator('[data-demo-lightbox]')).toBeVisible();

    const downloadPromise = page.waitForEvent('download');
    await demo.locator('[data-demo-download]').click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toBe('demo-photo.webp');

    const complete = demo.locator('[data-demo-panel="complete"]');
    await expect(complete).toBeVisible();
    await expect(complete).toContainText(/whole product/i);
    await expect(complete.locator('a[href="/payment?plan=standard"]')).toBeVisible();
    await expect(complete).toContainText(/account first/i);
    await phone.close();
  });

  test('accepts a real image and serves the original back for download', async ({ page, context }) => {
    await page.goto('/');
    const demo = page.locator('[data-live-demo]');
    const ready = demo.locator('[data-demo-panel="ready"]');
    await expect(ready).toBeVisible();
    const demoPath = await ready.locator('[data-demo-link]').getAttribute('href');

    const phone = await context.newPage();
    await phone.goto(demoPath!);
    await phone.setInputFiles('#demo-photo', { name: 'my-party.jpg', mimeType: 'image/jpeg', buffer: DEMO_JPEG });
    await expect(phone.locator('#demo-upload')).toBeEnabled();
    await phone.locator('#demo-upload').click();
    await expect(phone.locator('#demo-success')).toBeVisible();
    await expect(demo.locator('[data-demo-panel="photo"]')).toBeVisible();

    const downloadPath = await phone.locator('#demo-download').getAttribute('href');
    const response = await phone.request.get(downloadPath!);
    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('image/jpeg');
    expect(response.headers()['content-disposition']).toContain('my-party.jpg');
    expect((await response.body()).byteLength).toBe(DEMO_JPEG.byteLength);
    await phone.close();
  });

  test('gives separate browser visitors separate temporary galleries', async ({ browser }) => {
    const firstContext = await browser.newContext();
    const secondContext = await browser.newContext();
    const first = await firstContext.newPage();
    const second = await secondContext.newPage();
    await Promise.all([first.goto('/'), second.goto('/')]);

    const firstLink = first.locator('[data-demo-link]');
    const secondLink = second.locator('[data-demo-link]');
    await expect(firstLink).not.toHaveAttribute('href', '#');
    await expect(secondLink).not.toHaveAttribute('href', '#');
    expect(await firstLink.getAttribute('href')).not.toBe(await secondLink.getAttribute('href'));

    await firstContext.close();
    await secondContext.close();
  });
});
