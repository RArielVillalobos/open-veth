import { test, expect, Page, WebSocketRoute } from '@playwright/test';
import { setupApiMocks } from './fixtures/api-mocks';
import { MOCK_LAB_2, MOCK_ROUTES } from './fixtures/test-data';

async function prepare(page: Page) {
  let events: WebSocketRoute | undefined;
  await page.routeWebSocket('**/api/v1/events', socket => { events = socket; });
  const state = await setupApiMocks(page, { autoActivate: false });
  return { state, send: (data: object) => events!.send(JSON.stringify(data)) };
}

async function openWorkspace(page: Page) {
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'Manage laboratories', exact: true })).toContainText('E2E Test Lab');
  await expect(page.locator('.cy-container canvas').first()).toBeVisible();
}

async function selectNode(page: Page, id: string) {
  await page.evaluate(nodeId => {
    const cy = (document.querySelector('.cy-container') as any).__cy;
    cy.getElementById(nodeId).emit('tap');
  }, id);
}

test('activation keeps the previous laboratory until completion is confirmed', async ({ page }) => {
  const mock = await prepare(page); mock.state.labs.push({ ...MOCK_LAB_2 });
  await openWorkspace(page);
  await page.getByRole('button', { name: 'Manage laboratories', exact: true }).click();
  await page.getByRole('button', { name: 'Open Laboratory', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Manage laboratories', exact: true })).toContainText('E2E Test Lab');
  await expect(page.locator('.canvas-progress')).toContainText('Activating Second Lab');
  expect(await page.evaluate(() => localStorage.getItem('openveth_current_lab_id'))).not.toBe('lab-e2e-2');
  mock.send({ type: 'lab:activated', labId: 'lab-e2e-2' });
  await expect(page.getByRole('button', { name: 'Manage laboratories', exact: true })).toContainText('Second Lab');
  await expect(page.locator('.canvas-progress')).toHaveCount(0);
});

test('a rejected activation retains the selected laboratory and allows retry', async ({ page }) => {
  const mock = await prepare(page); mock.state.labs.push({ ...MOCK_LAB_2 });
  await page.route('**/api/v1/laboratories/*/activate', route => route.fulfill({ status: 409, json: { error: 'busy' } }));
  await openWorkspace(page);
  await page.getByRole('button', { name: 'Manage laboratories', exact: true }).click();
  await page.getByRole('button', { name: 'Open Laboratory', exact: true }).click();
  await page.getByRole('button', { name: 'Close laboratory manager', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Manage laboratories', exact: true })).toContainText('E2E Test Lab');
  await expect(page.getByRole('button', { name: 'Retry activation' })).toBeVisible();
});

test('reset confirmation is keyboard accessible and cancellation preserves the laboratory', async ({ page }) => {
  const mock = await prepare(page); await openWorkspace(page);
  await page.getByRole('button', { name: 'Reset', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Reset laboratory?' });
  await expect(dialog).toContainText('E2E Test Lab');
  await expect(dialog.getByRole('button', { name: 'Cancel', exact: true })).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  expect(mock.state.nodes).toHaveLength(2);
  await expect(page.getByRole('button', { name: 'Reset', exact: true })).toBeFocused();
});

test('import requires a destination choice before sending a destructive request', async ({ page }) => {
  await prepare(page); await openWorkspace(page);
  let imported = 0;
  await page.route('**/api/v1/topology/import?**', route => { imported++; return route.fulfill({ json: { nodes: 2, links: 1, errors: [] } }); });
  await page.locator('input[type=file]').first().setInputFiles({ name: 'sample.yaml', mimeType: 'application/yaml', buffer: Buffer.from('name: Sample\nnodes: []\nlinks: []') });
  const dialog = page.getByRole('dialog', { name: 'Import topology' });
  await expect(dialog.getByRole('button', { name: 'Import into new laboratory' })).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Replace current laboratory' })).toBeVisible();
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  expect(imported).toBe(0);
});

test('import into a new laboratory waits for activation and preserves the previous topology', async ({ page }) => {
  const mock = await prepare(page); await openWorkspace(page);
  let target = '', importedInto = '';
  await page.route('**/api/v1/laboratories/*/activate', async route => {
    target = route.request().url().split('/laboratories/')[1].split('/activate')[0];
    await route.fulfill({ status: 202, json: { lab_id: target } });
  });
  await page.route('**/api/v1/topology/import?**', route => {
    importedInto = new URL(route.request().url()).searchParams.get('lab_id')!;
    return route.fulfill({ json: { nodes: 0, links: 0, errors: [] } });
  });
  await page.locator('input[type=file]').first().setInputFiles({ name: 'separate.yaml', mimeType: 'application/yaml', buffer: Buffer.from('name: Separate\nnodes: []\nlinks: []') });
  await page.getByRole('dialog', { name: 'Import topology' }).getByRole('button', { name: 'Import into new laboratory' }).click();
  await expect.poll(() => target).not.toBe('');
  expect(importedInto).toBe('');
  mock.send({ type: 'lab:activated', labId: target });
  await expect.poll(() => importedInto).toBe(target);
  expect(mock.state.nodes.filter(node => node.lab_id === 'lab-e2e-1')).toHaveLength(2);
});

test('a partial state save is reported as incomplete', async ({ page }) => {
  await prepare(page); await openWorkspace(page);
  await page.route('**/api/v1/laboratories/*/save-state', route => route.fulfill({ json: { ips_saved: 2, routes_saved: 1, snapshots_saved: 1 } }));
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.locator('header')).toContainText('Action failed');
  await expect(page.getByText('Network settings were saved, but some node snapshots failed. Save again to retry.', { exact: true })).toBeVisible();
  await expect(page.locator('header')).not.toContainText('State saved');
});

test('rapid drags persist the positions of both nodes', async ({ page }) => {
  await prepare(page); await openWorkspace(page);
  const saved: string[] = [];
  await page.route('**/api/v1/nodes/*/position', route => {
    saved.push(route.request().url().split('/nodes/')[1].split('/position')[0]);
    return route.fulfill({ status: 204 });
  });
  await page.evaluate(() => {
    const cy = (document.querySelector('.cy-container') as any).__cy;
    cy.getElementById('node-r1').position({ x: 200, y: 150 }).emit('dragfree');
    cy.getElementById('node-pc1').position({ x: 450, y: 250 }).emit('dragfree');
  });
  await expect.poll(() => saved).toEqual(['node-r1', 'node-pc1']);
});

test('selecting nodes opens the inspector without creating links', async ({ page }, testInfo) => {
  await prepare(page);
  await page.route('**/api/v1/nodes/*/routes', route => route.fulfill({ json: [...MOCK_ROUTES, { dst: '192.168.100.128/25', gateway: '192.168.100.254', dev: 'eth0', protocol: 'static' }] }));
  await openWorkspace(page);
  await selectNode(page, 'node-r1');
  const inspector = page.getByRole('region', { name: 'Selection properties' });
  await expect(inspector.getByRole('heading', { name: 'R1', exact: true })).toBeVisible();
  await expect(inspector.getByRole('status')).toHaveText('Running');
  await expect(inspector.getByRole('button', { name: 'Capture on eth0', exact: true })).toBeVisible();
  await expect(inspector.getByText('Node ID', { exact: true })).not.toBeVisible();
  await expect(inspector.getByRole('button', { name: 'Power off', exact: true })).toBeVisible();
  await expect(inspector.getByRole('button', { name: 'Traceroute', exact: true })).toBeVisible();
  await expect(inspector.getByText('192.168.100.128/25', { exact: true })).toBeVisible();
  await inspector.getByRole('button', { name: 'Widen inspector' }).click();
  await expect(page.locator('.inspector')).toHaveClass(/inspector--wide/);
  await page.getByRole('button', { name: 'Fit topology', exact: true }).click();
  await page.screenshot({ path: testInfo.outputPath('workspace-inspector.png'), fullPage: true });
  await page.setViewportSize({ width: 960, height: 720 });
  await page.screenshot({ path: testInfo.outputPath('workspace-notebook.png'), fullPage: true });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await selectNode(page, 'node-pc1');
  await expect(inspector.getByRole('heading', { name: 'PC1', exact: true })).toBeVisible();
  expect(await page.evaluate(() => (document.querySelector('.cy-container') as any).__cy.edges().length)).toBe(1);
});

test('a failed refresh remains visible instead of reporting saved', async ({ page }) => {
  await prepare(page); await openWorkspace(page);
  await page.route('**/api/v1/nodes?**', route => route.fulfill({ status: 503, json: { error: 'Docker unavailable' } }));
  await page.getByRole('button', { name: 'Refresh interfaces', exact: true }).click();
  await expect(page.locator('header')).toContainText('Action failed');
  await expect(page.locator('header')).not.toContainText('saved');
  await expect(page.getByRole('button', { name: 'Reload view', exact: true })).toBeVisible();
});


test('packet capture keeps filters and packet details usable', async ({ page }, testInfo) => {
  await setupApiMocks(page);
  let capture: WebSocketRoute | undefined;
  await page.routeWebSocket('**/api/v1/sniff?**', socket => { capture = socket; });
  await openWorkspace(page);
  await page.waitForFunction(() => (document.querySelector('.cy-container') as any)?.__cy?.nodes().length > 0);
  await selectNode(page, 'node-r1');
  await page.getByRole('button', { name: 'Capture on eth0' }).click();
  const dialog = page.getByRole('dialog', { name: 'Packet capture: R1 / eth0' });
  await expect(dialog.getByText('Capturing', { exact: true })).toBeVisible();
  capture!.send(JSON.stringify({ timestamp: '2026-11-13T16:00:00Z', source: '10.0.0.1', destination: '10.0.0.254', protocol: 'ICMP', length: 84, ttl: 64, info: 'echo request' }));
  await expect(dialog.getByRole('cell', { name: 'echo request', exact: true })).toBeVisible();
  await dialog.getByRole('button', { name: 'TCP', exact: true }).click();
  await expect(dialog.getByText('No matching packets', { exact: true })).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'TCP', exact: true })).toHaveAttribute('aria-pressed', 'true');
  await dialog.getByRole('button', { name: 'ALL', exact: true }).click();
  await dialog.getByRole('cell', { name: 'echo request', exact: true }).click();
  await expect(dialog.getByText('Packet Details', { exact: true })).toBeVisible();
  await dialog.getByRole('button', { name: 'Pause capture', exact: true }).click();
  await expect(dialog.getByText('Paused', { exact: true })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('packet-capture.png'), fullPage: true });
  await dialog.getByRole('button', { name: 'Close packet capture' }).click();
  await expect(dialog).not.toBeVisible();
});


for (const mode of ['automatic', 'manual']) {
  test(mode + ' reconnection recovers a missed activation event', async ({ page }) => {
    const state = await setupApiMocks(page, { autoActivate: false });
    state.labs.push({ ...MOCK_LAB_2 });
    let events: WebSocketRoute | undefined;
    await page.routeWebSocket('**/api/v1/events', socket => { events = socket; });
    await page.route('**/api/v1/laboratories/*/activate', route => route.fulfill({ status: 202, json: { lab_id: MOCK_LAB_2.id, operation_id: 'recovery-op' } }));
    await page.route('**/api/v1/laboratories/*/activation', route => route.fulfill({ json: { lab_id: MOCK_LAB_2.id, operation_id: 'recovery-op', status: 'succeeded' } }));
    await openWorkspace(page);
    await page.getByRole('button', { name: 'Manage laboratories', exact: true }).click();
    await page.getByRole('button', { name: 'Open Laboratory', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Manage laboratories', exact: true })).toContainText('E2E Test Lab');
    events!.close({ code: 1011, reason: 'test disconnect' });
    if (mode === 'manual') await page.getByRole('button', { name: 'Reconnect events', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Manage laboratories', exact: true })).toContainText(MOCK_LAB_2.name);
    await expect(page.getByRole('button', { name: 'Manage laboratories', exact: true })).toBeEnabled();
  });
}
