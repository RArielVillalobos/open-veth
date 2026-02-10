import { test, expect } from '@playwright/test';
import { setupApiMocks } from './fixtures/api-mocks';
import { MOCK_LAB_2 } from './fixtures/test-data';

test.describe('Lab Management', () => {
  test.beforeEach(async ({ page }) => {
    await setupApiMocks(page);
    await page.goto('/');
    await expect(page.locator('text=OpenVeth')).toBeVisible();
    // Dismiss welcome modal if present
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});
  });

  test('clicking lab name opens Lab Manager modal', async ({ page }) => {
    // Click the lab name button in the header
    await page.locator('button[title="Manage Laboratories"]').click();

    // Lab Manager modal should be visible
    await expect(page.locator('text=My Laboratories')).toBeVisible();
    await expect(page.locator('text=Create New Project')).toBeVisible();
  });

  test('create a new laboratory', async ({ page }) => {
    await page.locator('button[title="Manage Laboratories"]').click();
    await expect(page.locator('text=My Laboratories')).toBeVisible();

    const postPromise = page.waitForRequest((req) =>
      req.url().includes('/api/v1/laboratories') &&
      req.method() === 'POST' &&
      !req.url().includes('/activate')
    );

    // Type lab name and create
    const labInput = page.locator('input[placeholder*="Project Name"]');
    await labInput.fill('BGP-Lab');
    await page.locator('button:has-text("Create Laboratory")').click();

    // Verify API call
    const req = await postPromise;
    const body = req.postDataJSON();
    expect(body.name).toBe('BGP-Lab');
  });

  test('switch to a different laboratory', async ({ page }) => {
    // Pre-populate a second lab in the mock state
    const state = await setupApiMocks(page);
    state.labs.push({ ...MOCK_LAB_2 });

    // Re-navigate to pick up updated state
    await page.goto('/');
    await expect(page.locator('text=OpenVeth')).toBeVisible();

    await page.locator('button[title="Manage Laboratories"]').click();
    await expect(page.locator('text=My Laboratories')).toBeVisible();

    // The second lab should show "Open Project"
    await expect(page.locator('text=Second Lab')).toBeVisible();

    const activatePromise = page.waitForRequest((req) =>
      req.url().includes('/activate') && req.method() === 'POST'
    );

    await page.locator('button:has-text("Open Project")').click();
    await activatePromise;
  });

  test('close Lab Manager modal', async ({ page }) => {
    await page.locator('button[title="Manage Laboratories"]').click();
    await expect(page.locator('text=My Laboratories')).toBeVisible();

    // Close via the X button (SVG with close icon)
    await page.locator('.fixed.inset-0 button').first().click();

    // Modal should disappear - wait for it to be hidden
    // The modal has the class .fixed.inset-0
    await expect(page.locator('text=My Laboratories')).not.toBeVisible({ timeout: 3000 });
  });

  test('active lab shows Active badge instead of Open Project', async ({ page }) => {
    await page.locator('button[title="Manage Laboratories"]').click();
    await expect(page.locator('text=My Laboratories')).toBeVisible();

    // The current lab should show "Active" instead of "Open Project"
    await expect(page.locator('button:has-text("Active")')).toBeVisible();
  });
});

test.describe('Terminal Panel', () => {
  test.beforeEach(async ({ page }) => {
    await setupApiMocks(page);
    // Intercept WebSocket connections so they don't fail
    await page.route('**/api/v1/terminal**', async (route) => {
      await route.abort();
    });
    await page.goto('/');
    await expect(page.locator('text=OpenVeth')).toBeVisible();
    // Dismiss welcome modal if present
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});
  });

  test('terminal panel is hidden by default', async ({ page }) => {
    // The terminal panel should not be visible initially
    await expect(page.locator('app-terminal-panel')).not.toBeVisible();
  });

  test('opening a terminal shows the panel and creates a tab', async ({ page }) => {
    // Dismiss welcome modal
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});

    // Right click on node R1 position (mocked at 300, 200)
    // We target the .cy-container which is where Cytoscape renders
    await page.locator('.cy-container').click({ button: 'right', position: { x: 300, y: 200 } });
    
    // Click "Open Terminal" in the context menu (DOM real)
    await page.locator('.context-menu__item:has-text("Open Terminal")').click();

    // Panel should now be visible
    await expect(page.locator('app-terminal-panel')).toBeVisible();
    
    // Tab for R1 should be present and active
    const tab = page.locator('.tab--active');
    await expect(tab).toContainText('R1');
  });

  test('switching tabs between two nodes', async ({ page }) => {
    // Dismiss welcome modal
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});

    // Open terminal for R1 (at 300, 200)
    await page.locator('.cy-container').click({ button: 'right', position: { x: 300, y: 200 } });
    await page.locator('.context-menu__item:has-text("Open Terminal")').click();

    // Open terminal for PC1 (at 500, 200)
    await page.locator('.cy-container').click({ button: 'right', position: { x: 500, y: 200 } });
    await page.locator('.context-menu__item:has-text("Open Terminal")').click();

    // Verify two tabs exist
    await expect(page.locator('.tab')).toHaveCount(2);

    // PC1 should be active (last opened)
    await expect(page.locator('.tab--active')).toContainText('PC1');

    // Click R1 tab to switch
    await page.locator('.tab:has-text("R1")').click();

    // R1 should now be active
    await expect(page.locator('.tab--active')).toContainText('R1');
  });
});
