import { Node, Link, Laboratory, InterfaceInfo, RouteInfo } from '../app/models/topology.model';

export function createMockNode(overrides: Partial<Node> = {}): Node {
  return {
    id: 'node-1',
    name: 'R1',
    type: 'router',
    x: 200,
    y: 150,
    status: 'running',
    ...overrides,
  };
}

export function createMockLink(overrides: Partial<Link> = {}): Link {
  return {
    id: 'link-1',
    source: 'node-1',
    target: 'node-2',
    source_int: 'eth1',
    target_int: 'eth1',
    ...overrides,
  };
}

export function createMockLaboratory(overrides: Partial<Laboratory> = {}): Laboratory {
  return {
    id: 'lab-1',
    name: 'Default Laboratory',
    status: 'active',
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

export function createMockInterface(overrides: Partial<InterfaceInfo> = {}): InterfaceInfo {
  return {
    ifname: 'eth1',
    addr_info: [{ local: '10.0.0.1', prefixlen: 24 }],
    ...overrides,
  };
}

export function createMockRoute(overrides: Partial<RouteInfo> = {}): RouteInfo {
  return {
    dst: '10.0.0.0/24',
    dev: 'eth1',
    protocol: 'kernel',
    scope: 'link',
    ...overrides,
  };
}
