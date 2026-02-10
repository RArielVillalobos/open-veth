import { test, expect } from '@playwright/test';
import { setupApiMocks } from './fixtures/api-mocks';
import { MOCK_TOPOLOGY_YAML } from './fixtures/test-data';

test.describe('Import / Export', () => {
  test.beforeEach(async ({ page }) => {
    await setupApiMocks(page);
    await page.goto('/');
    await expect(page.locator('text=OpenVeth')).toBeVisible();
    // Dismiss welcome modal if present
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});
  });

  test('Export button triggers download', async ({ page }) => {
    const exportPromise = page.waitForRequest((req) =>
      req.url().includes('/api/v1/topology/export') && req.method() === 'GET'
    );

    // Click Export button
    await page.locator('button:has-text("Export")').click();

    await exportPromise;
  });

  test('Import button opens file chooser', async ({ page }) => {
    // Verify hidden file input exists
    const fileInput = page.locator('input[type="file"][accept=".yaml,.yml"]');
    await expect(fileInput).toBeAttached();
  });

  test('Importing a YAML file calls the import API', async ({ page }) => {
    const importPromise = page.waitForRequest((req) =>
      req.url().includes('/api/v1/topology/import') && req.method() === 'POST'
    );

    // Use the hidden file input to set files
    const fileInput = page.locator('input[type="file"][accept=".yaml,.yml"]');
    await fileInput.setInputFiles({
      name: 'topology.yaml',
      mimeType: 'application/x-yaml',
      buffer: Buffer.from(MOCK_TOPOLOGY_YAML),
    });

    // Verify import API was called
    const req = await importPromise;
    expect(req.postData()).toContain('Imported Lab');
  });

  test('Import success shows toast', async ({ page }) => {
    const fileInput = page.locator('input[type="file"][accept=".yaml,.yml"]');
    await fileInput.setInputFiles({
      name: 'topology.yaml',
      mimeType: 'application/x-yaml',
      buffer: Buffer.from(MOCK_TOPOLOGY_YAML),
    });

    // Wait for success toast
    await expect(page.locator('text=Topology imported successfully')).toBeVisible({ timeout: 5000 });
  });

  test('Import error shows error toast', async ({ page }) => {
    // Override the import route to return an error
    await page.route('**/api/v1/topology/import', async (route) => {
      await route.fulfill({
        status: 400,
        json: { error: 'Invalid YAML format' },
      });
    });

    const fileInput = page.locator('input[type="file"][accept=".yaml,.yml"]');
    await fileInput.setInputFiles({
      name: 'bad.yaml',
      mimeType: 'application/x-yaml',
      buffer: Buffer.from('invalid: [yaml'),
    });

    // Wait for error toast
    await expect(page.locator('text=Import failed')).toBeVisible({ timeout: 5000 });
  });
});
