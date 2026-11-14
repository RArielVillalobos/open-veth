import { test, expect } from '@playwright/test';
import { setupApiMocks } from './fixtures/api-mocks';

test.describe('Node CRUD', () => {
  test.beforeEach(async ({ page }) => {
    await setupApiMocks(page);
    await page.goto('/');
    // Wait for the app to load
    await expect(page.locator('text=OpenVeth')).toBeVisible();
  });

  test('page loads with topology and shows header', async ({ page }) => {
    await expect(page.locator('header')).toBeVisible();
    // Lab name should be visible in the header
    await expect(page.locator('text=E2E Test Lab')).toBeVisible();
    // Node palette should be visible (now called "Components")
    await expect(page.getByRole('heading', { name: 'Devices', exact: true })).toBeVisible();
  });

  test('node palette shows Router, Switch, Hub, Host, Cloud options', async ({ page }) => {
    // Dismiss welcome modal if present
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});

    await expect(page.getByRole('button', { name: 'Add Router', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Add Switch', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Add Hub', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Add Host', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Add Cloud', exact: true })).toBeVisible();
  });

  test('create a router node via palette', async ({ page }) => {
    // Dismiss welcome modal if present
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});

    // Intercept the POST to /nodes
    const postPromise = page.waitForRequest((req) =>
      req.url().includes('/api/v1/nodes') && req.method() === 'POST'
    );

    // Click Router button to start naming
    await page.getByRole('button', { name: 'Add Router', exact: true }).click();

    // Input should appear with placeholder
    const nameInput = page.locator('input[placeholder*="R1"]');
    await expect(nameInput).toBeVisible();

    // Type node name
    await nameInput.fill('MyRouter');

    // Click Add
    await page.locator('button:has-text("Add")').click();

    // Verify API call was made
    const postReq = await postPromise;
    const body = postReq.postDataJSON();
    expect(body.name).toBe('MyRouter');
    expect(body.type).toBe('router');
  });

  test('create a host node via palette', async ({ page }) => {
    // Dismiss welcome modal if present
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});

    const postPromise = page.waitForRequest((req) =>
      req.url().includes('/api/v1/nodes') && req.method() === 'POST'
    );

    await page.getByRole('button', { name: 'Add Host', exact: true }).click();
    const nameInput = page.locator('input[placeholder*="PC1"]');
    await expect(nameInput).toBeVisible();
    await nameInput.fill('PC2');
    await page.locator('button:has-text("Add")').click();

    const postReq = await postPromise;
    const body = postReq.postDataJSON();
    expect(body.name).toBe('PC2');
    expect(body.type).toBe('host');
  });

  test('create a cloud node via palette', async ({ page }) => {
    // Dismiss welcome modal if present
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});

    const postPromise = page.waitForRequest((req) =>
      req.url().includes('/api/v1/nodes') && req.method() === 'POST'
    );

    await page.getByRole('button', { name: 'Add Cloud', exact: true }).click();
    const nameInput = page.locator('input[placeholder*="INET"]');
    await expect(nameInput).toBeVisible();
    await nameInput.fill('INTERNET');
    await page.locator('button:has-text("Add")').click();

    const postReq = await postPromise;
    const body = postReq.postDataJSON();
    expect(body.name).toBe('INTERNET');
    expect(body.type).toBe('cloud');
  });

  test('create a hub node via palette', async ({ page }) => {
    // Dismiss welcome modal if present
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});

    const postPromise = page.waitForRequest((req) =>
      req.url().includes('/api/v1/nodes') && req.method() === 'POST'
    );

    await page.getByRole('button', { name: 'Add Hub', exact: true }).click();
    const nameInput = page.locator('input[placeholder*="HUB1"]');
    await expect(nameInput).toBeVisible();
    await nameInput.fill('HUB1');
    await page.locator('button:has-text("Add")').click();

    const postReq = await postPromise;
    const body = postReq.postDataJSON();
    expect(body.name).toBe('HUB1');
    expect(body.type).toBe('hub');
  });

  test('Reset Lab clears nodes after confirm dialog', async ({ page }) => {
    // Dismiss welcome modal if present
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});

    // Set up dialog handler to accept the confirm
    page.on('dialog', (dialog) => dialog.accept());

    const cleanupPromise = page.waitForRequest((req) =>
      req.url().includes('/cleanup') && req.method() === 'DELETE'
    );

    // Click Reset button (title contains "Reset Lab")
    await page.getByRole('button', { name: 'Reset', exact: true }).click();
    await page.getByRole('dialog', { name: 'Reset laboratory?' }).getByRole('button', { name: 'Reset laboratory', exact: true }).click();

    // Verify cleanup API was called
    await cleanupPromise;
  });
});


for (const empty of [true, false]) {
  test('new node stays visible and selected after panning' + (empty ? ' an empty canvas' : ' with a terminal open'), async ({ page }) => {
    await setupApiMocks(page, { empty });
    await page.goto('/');
    await page.waitForFunction(() => !!(document.querySelector('.cy-container') as any)?.__cy);
    if (empty) await page.getByRole('button', { name: /^New Topology/ }).click();
    else {
      await page.waitForFunction(() => (document.querySelector('.cy-container') as any).__cy.nodes().length > 0);
      await page.evaluate(() => { const cy = (document.querySelector('.cy-container') as any).__cy; const n = cy.getElementById('node-r1'); n.emit({ type: 'cxttap', renderedPosition: n.renderedPosition() }); });
      await page.locator('.context-menu__item:has-text("Open Terminal")').click();
      await expect(page.locator('app-terminal-panel')).toBeVisible();
    }
    await page.evaluate(() => { const cy = (document.querySelector('.cy-container') as any).__cy; cy.zoom(1.2); cy.pan({ x: -2000, y: 1300 }); });
    await page.getByRole('button', { name: 'Add Router', exact: true }).click();
    const input = page.locator('input[placeholder*="R1"]');
    await input.fill('VisibleRouter');
    await input.press('Enter');
    await expect(page.getByRole('region', { name: 'Selection properties' }).getByRole('heading', { name: 'VisibleRouter', exact: true })).toBeVisible();
    await expect.poll(() => page.evaluate(() => {
      const el = document.querySelector('.cy-container') as HTMLElement;
      const cy = (el as any).__cy;
      const n = cy.nodes().filter((node: any) => node.data('name') === 'VisibleRouter').first();
      const b = n.renderedBoundingBox();
      return Math.abs(cy.zoom() - 1.2) < 0.001 && b.x1 >= 0 && b.y1 >= 40 && b.x2 <= el.clientWidth && b.y2 <= el.clientHeight;
    })).toBe(true);
  });
}
