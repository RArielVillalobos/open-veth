import { Page } from '@playwright/test';
import { MOCK_LAB, MOCK_NODES, MOCK_LINKS, MOCK_INTERFACES, MOCK_ROUTES } from './test-data';

interface MockState {
  nodes: any[];
  links: any[];
  labs: any[];
}

/**
 * Sets up API route mocks for all /api/v1/* endpoints.
 * Returns a mutable state object so tests can inspect what was "created".
 */
export async function setupApiMocks(page: Page, options?: { empty?: boolean }): Promise<MockState> {
  const state: MockState = {
    nodes: options?.empty ? [] : [...MOCK_NODES],
    links: options?.empty ? [] : [...MOCK_LINKS],
    labs: [{ ...MOCK_LAB }],
  };

  // --- Nodes ---
  await page.route('**/api/v1/nodes?**', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ json: state.nodes });
    } else {
      await route.continue();
    }
  });

  await page.route('**/api/v1/nodes', async (route) => {
    if (route.request().method() === 'POST') {
      const body = route.request().postDataJSON();
      const node = { ...body, id: body.id || `node-${Date.now()}`, status: 'running' };
      state.nodes.push(node);
      await route.fulfill({ json: node });
    } else if (route.request().method() === 'GET') {
      await route.fulfill({ json: state.nodes });
    } else {
      await route.continue();
    }
  });

  await page.route('**/api/v1/nodes/*/interfaces', async (route) => {
    await route.fulfill({ json: MOCK_INTERFACES });
  });

  await page.route('**/api/v1/nodes/*/routes', async (route) => {
    await route.fulfill({ json: MOCK_ROUTES });
  });

  await page.route('**/api/v1/nodes/*/position', async (route) => {
    await route.fulfill({ json: null });
  });

  await page.route('**/api/v1/nodes/*', async (route) => {
    if (route.request().method() === 'DELETE') {
      const url = route.request().url();
      const id = url.split('/nodes/')[1];
      state.nodes = state.nodes.filter((n) => n.id !== id);
      state.links = state.links.filter((l) => l.source !== id && l.target !== id);
      await route.fulfill({ json: null });
    } else {
      await route.continue();
    }
  });

  // --- Links ---
  await page.route('**/api/v1/links?**', async (route) => {
    await route.fulfill({ json: state.links });
  });

  await page.route('**/api/v1/links', async (route) => {
    if (route.request().method() === 'POST') {
      const body = route.request().postDataJSON();
      const link = { ...body, id: body.id || `link-${Date.now()}` };
      state.links.push(link);
      await route.fulfill({ json: link });
    } else if (route.request().method() === 'GET') {
      await route.fulfill({ json: state.links });
    } else {
      await route.continue();
    }
  });

  await page.route('**/api/v1/links/*', async (route) => {
    if (route.request().method() === 'DELETE') {
      const url = route.request().url();
      const id = url.split('/links/')[1];
      state.links = state.links.filter((l) => l.id !== id);
      await route.fulfill({ json: null });
    } else {
      await route.continue();
    }
  });

  // --- Laboratories ---
  await page.route('**/api/v1/laboratories', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ json: state.labs });
    } else if (route.request().method() === 'POST') {
      const body = route.request().postDataJSON();
      const lab = { ...body, id: body.id || `lab-${Date.now()}`, status: 'inactive', created_at: new Date().toISOString() };
      state.labs.push(lab);
      await route.fulfill({ json: lab });
    } else {
      await route.continue();
    }
  });

  await page.route('**/api/v1/laboratories/*/activate', async (route) => {
    const url = route.request().url();
    const id = url.split('/laboratories/')[1].split('/activate')[0];
    state.labs = state.labs.map((l) => ({ ...l, status: l.id === id ? 'active' : 'inactive' }));
    await route.fulfill({ json: null });
  });

  await page.route('**/api/v1/laboratories/*/save-state', async (route) => {
    await route.fulfill({ json: { message: 'ok', ips_saved: 2, routes_saved: 1 } });
  });

  await page.route('**/api/v1/laboratories/*/cleanup', async (route) => {
    state.nodes = [];
    state.links = [];
    await route.fulfill({ json: null });
  });

  await page.route('**/api/v1/laboratories/*', async (route) => {
    const method = route.request().method();
    const url = route.request().url();
    const id = url.split('/laboratories/')[1];

    if (method === 'PATCH') {
      const body = route.request().postDataJSON();
      state.labs = state.labs.map((l) => (l.id === id ? { ...l, ...body } : l));
      const updated = state.labs.find((l) => l.id === id);
      await route.fulfill({ json: updated });
    } else if (method === 'DELETE') {
      state.labs = state.labs.filter((l) => l.id !== id);
      await route.fulfill({ json: null });
    } else {
      await route.continue();
    }
  });

  // --- Topology ---
  await page.route('**/api/v1/topology/export', async (route) => {
    await route.fulfill({
      body: 'name: E2E Test Lab\nnodes:\n  - id: node-r1\n    name: R1\nlinks: []',
      headers: { 'Content-Type': 'application/x-yaml' },
    });
  });

  await page.route('**/api/v1/topology/import', async (route) => {
    await route.fulfill({ json: { message: 'Topology imported successfully' } });
  });

  await page.route('**/api/v1/topology/deploy', async (route) => {
    await route.fulfill({ json: { status: 'ok' } });
  });

  // --- System ---
  await page.route('**/api/v1/system/health', async (route) => {
    await route.fulfill({ json: { status: 'ok' } });
  });

  return state;
}
