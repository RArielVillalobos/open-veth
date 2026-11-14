import { test, expect } from '@playwright/test';
import { setupApiMocks } from './fixtures/api-mocks';
import { MOCK_ALL_TYPES_NODES } from './fixtures/test-data';

/**
 * Helper to query the Cytoscape instance exposed on the canvas container.
 * Returns basic canvas state for assertions.
 */
async function getCyState(page: import('@playwright/test').Page) {
  return page.evaluate(() => {
    const container = document.querySelector('.cy-container') as any;
    const cy = container?.__cy;
    if (!cy) return null;
    return {
      zoom: cy.zoom() as number,
      minZoom: cy.minZoom() as number,
      maxZoom: cy.maxZoom() as number,
      nodeCount: cy.nodes().length as number,
      edgeCount: cy.edges().length as number,
      nodes: cy.nodes().map((n: any) => ({
        id: n.id(),
        type: n.data('type'),
        label: n.data('label'),
        width: n.width(),
        height: n.height(),
      })),
    };
  });
}

test.describe('Topology Canvas', () => {
  test.beforeEach(async ({ page }) => {
    await setupApiMocks(page);
    await page.goto('/');
    await expect(page.locator('text=OpenVeth')).toBeVisible();
    // Dismiss welcome modal if present
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});
  });

  test('canvas initializes with mock nodes and link', async ({ page }) => {
    // Wait for canvas to render
    await page.waitForSelector('.cy-container canvas');

    const state = await getCyState(page);
    expect(state).not.toBeNull();
    expect(state!.nodeCount).toBe(2); // R1 + PC1
    expect(state!.edgeCount).toBe(1); // link-1
  });

  test('zoom is bounded between minZoom and maxZoom', async ({ page }) => {
    await page.waitForSelector('.cy-container canvas');

    const state = await getCyState(page);
    expect(state).not.toBeNull();
    expect(state!.minZoom).toBeCloseTo(0.3, 1);
    expect(state!.maxZoom).toBeCloseTo(1.5, 1);
    expect(state!.zoom).toBeGreaterThanOrEqual(state!.minZoom);
    expect(state!.zoom).toBeLessThanOrEqual(state!.maxZoom);
  });

  test('all node types render with consistent dimensions', async ({ page }) => {
    // Override mock to include all 5 node types
    const state = await setupApiMocks(page);
    state.nodes = [...MOCK_ALL_TYPES_NODES];
    state.links = [];

    await page.goto('/');
    await expect(page.locator('text=OpenVeth')).toBeVisible();
    await page.locator('text=New Topology').click({ timeout: 2000 }).catch(() => {});
    await page.waitForSelector('.cy-container canvas');

    const cyState = await getCyState(page);
    expect(cyState).not.toBeNull();
    expect(cyState!.nodeCount).toBe(5);

    // Verify each node type is present
    const types = cyState!.nodes.map((n: any) => n.type).sort();
    expect(types).toEqual(['cloud', 'host', 'hub', 'router', 'switch']);

    // All nodes should have consistent 52x52 dimensions
    for (const node of cyState!.nodes) {
      expect(node.width).toBe(52);
      expect(node.height).toBe(52);
    }
  });

  test('adding a node updates the canvas', async ({ page }) => {
    await page.waitForSelector('.cy-container canvas');

    // Verify initial state
    let cyState = await getCyState(page);
    expect(cyState!.nodeCount).toBe(2);

    // Add a new router via palette
    await page.getByRole('button', { name: 'Add Router', exact: true }).click();
    const nameInput = page.locator('input[placeholder*="R1"]');
    await expect(nameInput).toBeVisible();
    await nameInput.fill('R2');
    await page.locator('button:has-text("Add")').click();

    // Wait for canvas to update
    await page.waitForTimeout(500);

    cyState = await getCyState(page);
    expect(cyState!.nodeCount).toBe(3);
  });

  test('fit respects maxZoom with few nodes', async ({ page }) => {
    await page.waitForSelector('.cy-container canvas');

    const state = await getCyState(page);
    expect(state).not.toBeNull();

    // Even with only 2 close nodes, zoom should not exceed maxZoom
    expect(state!.zoom).toBeLessThanOrEqual(1.5);
  });
});


test('cable labels adapt to selection and zoom without changing routing metadata', async ({ page }) => {
  const state = await setupApiMocks(page);
  state.nodes = state.nodes.map((n, i) => ({ ...n, interfaces: [{ ifname: 'eth0', addr_info: [{ local: '10.0.0.' + (i + 1), prefixlen: 24 }] }] })) as any;
  await page.goto('/');
  await page.waitForFunction(() => (document.querySelector('.cy-container') as any)?.__cy?.nodes().length === 2);
  await page.evaluate(() => { const cy = (document.querySelector('.cy-container') as any).__cy; cy.zoom(1); cy.center(); });
  await expect(page.locator('.endpoint-ip')).toHaveCount(2);
  await page.getByRole('combobox', { name: 'Cable labels' }).selectOption('interfaces');
  await expect(page.locator('.endpoint-ip')).toHaveCount(0);
  await expect(page.locator('.endpoint-interface')).toHaveCount(2);
  await page.evaluate(() => (document.querySelector('.cy-container') as any).__cy.getElementById('node-r1').emit('tap'));
  await expect(page.locator('.endpoint-ip')).toHaveCount(2);
  await page.getByRole('button', { name: 'Close properties' }).click();
  await expect(page.locator('.endpoint-ip')).toHaveCount(0);
  await page.getByRole('combobox', { name: 'Cable labels' }).selectOption('ip');
  await page.evaluate(() => (document.querySelector('.cy-container') as any).__cy.zoom(0.55));
  await expect(page.locator('.endpoint-ip')).toHaveCount(0);
  await page.evaluate(() => (document.querySelector('.cy-container') as any).__cy.getElementById('link-1').emit('tap'));
  await expect(page.locator('.endpoint-ip')).toHaveCount(2);
  expect(await page.evaluate(() => {
    const edge = (document.querySelector('.cy-container') as any).__cy.getElementById('link-1');
    return edge.data('source_int').includes('/24') && !edge.classes().some((c: string) => c.startsWith('subnet-'));
  })).toBe(true);
});

test('short diagonal cables keep endpoint labels clear of node icons', async ({ page }, testInfo) => {
  const state = await setupApiMocks(page);
  state.nodes = state.nodes.map((n, i) => ({ ...n, x: 300 + 120 * i, y: 200 + 70 * i, interfaces: [{ ifname: 'eth0', addr_info: [{ local: '192.168.100.' + (10 + i), prefixlen: 24 }] }] })) as any;
  await page.goto('/');
  await page.waitForFunction(() => (document.querySelector('.cy-container') as any)?.__cy?.nodes().length === 2);
  await page.evaluate(() => { const cy = (document.querySelector('.cy-container') as any).__cy; cy.zoom(1); cy.center(); });
  await expect(page.locator('.endpoint-ip')).toHaveCount(2);
  expect(await page.evaluate(() => {
    const container = document.querySelector('.cy-container')!;
    const origin = container.getBoundingClientRect();
    const cy = (container as any).__cy;
    const rectangles = [...document.querySelectorAll('.endpoint-label')].map(el => {
      const r = el.getBoundingClientRect(); return { x1: r.left - origin.left, y1: r.top - origin.top, x2: r.right - origin.left, y2: r.bottom - origin.top };
    });
    const overlaps = (a: any, b: any) => a.x1 < b.x2 && a.x2 > b.x1 && a.y1 < b.y2 && a.y2 > b.y1;
    return rectangles.every(r => cy.nodes().every((n: any) => !overlaps(r, n.renderedBoundingBox()))) && !overlaps(rectangles[0], rectangles[1]);
  })).toBe(true);
  await page.screenshot({ path: testInfo.outputPath('short-diagonal-labels.png'), fullPage: true });
});


test('domain overlays identify links without merging the server visually', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1500, height: 850 });
  const state = await setupApiMocks(page);
  const ids = ['pc-a', 'sw-a', 'server', 'sw-b', 'pc-b'];
  const types = ['host', 'switch', 'server', 'switch', 'host'];
  state.nodes = ids.map((id, i) => ({ id, name: ['PC-A', 'SW-A', 'SERVER', 'SW-B', 'PC-B'][i], type: types[i], x: 120 + i * 240, y: 280, status: 'running', lab_id: state.labs[0].id }));
  state.links = ids.slice(1).map((id, i) => ({ id: 'l' + (i + 1), source: ids[i], target: id, source_int: 'eth1', target_int: 'eth0', enabled: i !== 3, lab_id: state.labs[0].id }));
  await page.route('**/api/v1/laboratories/*/domains', route => route.fulfill({ json: {
    broadcast_domains: [{ id: 0, node_ids: ids.slice(0, 3), link_ids: ['l1', 'l2'] }, { id: 1, node_ids: ids.slice(2), link_ids: ['l3', 'l4'] }],
    collision_domains: state.links.map((l, i) => ({ id: i, node_ids: [l.source, l.target], link_ids: [l.id] })),
  } }));
  await page.goto('/');
  await page.waitForFunction(() => (document.querySelector('.cy-container') as any)?.__cy?.edges().length === 4);
  await page.getByRole('button', { name: 'Broadcast 2', exact: true }).click();
  const legend = page.getByRole('region', { name: 'Domain legend' });
  await expect(legend.getByText('BD1', { exact: true })).toBeVisible();
  await expect(legend.getByText('BD2', { exact: true })).toBeVisible();
  expect(await page.evaluate(() => {
    const cy = (document.querySelector('.cy-container') as any).__cy;
    return cy.nodes().every((n: any) => !n.classes().some((c: string) => c.startsWith('domain-'))) && cy.getElementById('l1').data('domain_label') === 'BD1' && cy.getElementById('l4').data('domain_label') === 'BD2' && cy.getElementById('l4').style('label') === 'Disabled · BD2';
  })).toBe(true);
  await legend.getByText('Model scope', { exact: true }).click();
  await expect(legend).toContainText('including disabled links and stopped nodes');
  await page.screenshot({ path: testInfo.outputPath('broadcast-domains.png'), fullPage: true });
  await page.getByRole('button', { name: 'Collision 4', exact: true }).click();
  await expect(legend).toContainText('Full-duplex links do not collide');
  await expect(legend.getByText('CD4', { exact: true })).toBeVisible();
  expect(await page.evaluate(() => (document.querySelector('.cy-container') as any).__cy.getElementById('l4').style('label'))).toBe('Disabled · CD4');
  await page.screenshot({ path: testInfo.outputPath('collision-domains.png'), fullPage: true });
  await page.getByRole('button', { name: 'Collision 4', exact: true }).click();
  await expect(legend).not.toBeVisible();
  expect(await page.evaluate(() => (document.querySelector('.cy-container') as any).__cy.getElementById('l4').style('label'))).toBe('Disabled');
  expect(await page.evaluate(() => (document.querySelector('.cy-container') as any).__cy.edges().every((e: any) => !e.data('domain_label') && !e.classes().some((c: string) => c.startsWith('domain-'))))).toBe(true);
});
