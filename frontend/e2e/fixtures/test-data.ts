export const MOCK_LAB = {
  id: 'lab-e2e-1',
  name: 'E2E Test Lab',
  status: 'active',
  created_at: '2026-01-01T00:00:00Z',
};

export const MOCK_LAB_2 = {
  id: 'lab-e2e-2',
  name: 'Second Lab',
  status: 'inactive',
  created_at: '2026-01-02T00:00:00Z',
};

export const MOCK_NODES = [
  {
    id: 'node-r1',
    name: 'R1',
    type: 'router',
    x: 300,
    y: 200,
    status: 'running',
    lab_id: 'lab-e2e-1',
  },
  {
    id: 'node-pc1',
    name: 'PC1',
    type: 'host',
    x: 500,
    y: 200,
    status: 'running',
    lab_id: 'lab-e2e-1',
  },
];

export const MOCK_LINKS = [
  {
    id: 'link-1',
    source: 'node-r1',
    target: 'node-pc1',
    source_int: 'eth1',
    target_int: 'eth1',
    lab_id: 'lab-e2e-1',
  },
];

export const MOCK_INTERFACES = [
  { ifname: 'lo', addr_info: [{ local: '127.0.0.1', prefixlen: 8 }] },
  { ifname: 'mgmt0', addr_info: [{ local: '172.17.0.2', prefixlen: 16 }] },
  { ifname: 'eth1', addr_info: [{ local: '10.0.0.1', prefixlen: 24 }] },
];

export const MOCK_ROUTES = [
  { dst: '10.0.0.0/24', dev: 'eth1', protocol: 'kernel', scope: 'link' },
  { dst: 'default', dev: 'mgmt0', protocol: 'kernel', scope: 'global', gateway: '172.17.0.1' },
];

export const MOCK_TOPOLOGY_YAML = `name: Imported Lab
nodes:
  - id: node-imp1
    name: IMP-R1
    type: router
    x: 200
    y: 150
links: []
`;
