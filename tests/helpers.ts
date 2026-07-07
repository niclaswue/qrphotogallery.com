import { Page, expect } from '@playwright/test';

// Superuser credentials the server is booted with (see .github/workflows/build.yml
// and the local test setup). Used by upgradeToPaid to promote a user via the
// PocketBase admin API. Falls back to the CI defaults for local runs.
const SUPERUSER_EMAIL = process.env.PB_SUPERUSER_EMAIL || 'test@test.com';
const SUPERUSER_PASSWORD = process.env.PB_SUPERUSER_PASSWORD || 'testpassword123';

export async function ensureAnonymous(page: Page) {
  await page.context().clearCookies();
}

// upgradeToPaid promotes the given user to a paid tier using the PocketBase
// superuser admin API. The server must have been booted with PB_SUPERUSER_EMAIL
// / PB_SUPERUSER_PASSWORD so the superuser exists. Leaves the user's own session
// cookie untouched (the admin token is only used for these direct API calls).
export async function upgradeToPaid(page: Page, email: string, tier: string = 'premium'): Promise<void> {
  const auth = await page.request.post('/api/collections/_superusers/auth-with-password', {
    data: { identity: SUPERUSER_EMAIL, password: SUPERUSER_PASSWORD },
  });
  if (!auth.ok()) throw new Error(`superuser auth failed (${auth.status()}): ${await auth.text()}`);
  const token = (await auth.json()).token as string;

  const filter = encodeURIComponent(`email="${email}"`);
  const lookup = await page.request.get(`/api/collections/users/records?filter=${filter}`, {
    headers: { Authorization: token },
  });
  if (!lookup.ok()) throw new Error(`user lookup failed (${lookup.status()}): ${await lookup.text()}`);
  const items = (await lookup.json()).items as Array<{ id: string }>;
  if (!items.length) throw new Error(`no user found for ${email}`);

  const patch = await page.request.patch(`/api/collections/users/records/${items[0].id}`, {
    headers: { Authorization: token },
    data: { tier },
  });
  if (!patch.ok()) throw new Error(`tier update failed (${patch.status()}): ${await patch.text()}`);
}

// gotoCreateReady navigates to the create page and waits until create.js has
// finished initialising. The init IIFE wires the Add/Save click handlers and
// then, as one of its last steps, stamps the custom-prompt counter with "0/120"
// (the server renders that element empty). Waiting for that text guarantees the
// #addCustomBtn / form handlers are bound before a fast test interacts with
// them — otherwise an early click lands before binding and is a silent no-op.
export async function gotoCreateReady(page: Page, path: string = '/create') {
  await page.goto(path);
  await expect(page.locator('#customQuestionCount')).toHaveText('0/120');
}

export async function registerUser(page: Page, email: string, password: string) {
  await page.goto('/register');
  await page.fill('input[name="email"]', email);
  await page.fill('input[name="password"]', password);
  await page.fill('input[name="password_confirm"]', password);
  await page.click('button[type="submit"]');
  await page.waitForLoadState('networkidle');
}

export async function loginUser(page: Page, email: string, password: string) {
  await page.goto('/login');
  await page.fill('input[name="identity"]', email);
  await page.fill('input[name="password"]', password);
  await page.click('button[type="submit"]');
  await page.waitForLoadState('networkidle');
}

// loginFreshUser registers a new account and returns the email it used. The
// optional first argument is ignored (kept for backwards compatibility with
// older specs that passed a display name).
export async function loginFreshUser(page: Page, _label?: string): Promise<string> {
  const email = randomEmail();
  await registerUser(page, email, 'password123');
  if (!page.url().match(/\/(overview|create)/)) {
    await loginUser(page, email, 'password123');
  }
  return email;
}

// createEventViaCookie writes the same cookie the /create POST handler would
// for an anonymous visitor, then visits /create/finish which finalises the
// event for the authenticated user. Returns the new event id, or null if the
// flow didn't land on /overview/{id}.
export async function createEventViaCookie(
  page: Page,
  title: string,
  prompts: string[],
  designId: string = 'classic',
  qrMode: string = 'cards'
): Promise<string | null> {
  const pendingData = JSON.stringify({
    title: title,
    prompts: prompts.join('\n'),
    design_id: designId,
    qr_mode: qrMode,
  });
  const b64 = Buffer.from(pendingData).toString('base64');
  await page.context().addCookies([
    { name: 'pending_event', value: b64, domain: 'localhost', path: '/' },
  ]);
  await page.goto('/create/finish');
  await page.waitForLoadState('networkidle');
  const url = page.url();
  const match = url.match(/\/overview\/([a-zA-Z0-9]+)/);
  return match ? match[1] : null;
}

export async function authedRequest(page: Page, url: string): Promise<{ status: number; headers: Record<string, string> }> {
  const cookies = await page.context().cookies();
  const pbAuth = cookies.find(c => c.name === 'pb_auth');
  const response = await page.request.get(url, {
    headers: pbAuth ? { Cookie: `pb_auth=${pbAuth.value}` } : {},
  });
  return {
    status: response.status(),
    headers: response.headers() as Record<string, string>,
  };
}

export async function authedPost(page: Page, url: string, form?: Record<string, string>): Promise<{ status: number; headers: Record<string, string> }> {
  const cookies = await page.context().cookies();
  const pbAuth = cookies.find(c => c.name === 'pb_auth');
  const response = await page.request.post(url, {
    headers: pbAuth ? { Cookie: `pb_auth=${pbAuth.value}` } : {},
    form: form,
  });
  return {
    status: response.status(),
    headers: response.headers() as Record<string, string>,
  };
}

export function randomEmail(): string {
  return `test_${Date.now()}_${Math.random().toString(36).slice(2, 8)}@example.com`;
}

export const TEST_USER = {
  email: 'testuser@example.com',
  password: 'password123',
};

export const TEST_USER_2 = {
  email: 'user2@example.com',
  password: 'password123',
};
