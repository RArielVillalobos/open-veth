import { Node, Link, Laboratory, InterfaceInfo, RouteInfo, DomainsResponse, TracerouteResponse } from '../app/models/topology.model';

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

export function createMockDomainsResponse(overrides: Partial<DomainsResponse> = {}): DomainsResponse {
  return {
    broadcast_domains: [
      { id: 0, node_ids: ['node-1', 'node-2', 'node-3'], link_ids: ['link-1', 'link-2'] },
    ],
    collision_domains: [
      { id: 0, node_ids: ['node-1', 'node-3'], link_ids: ['link-1'] },
      { id: 1, node_ids: ['node-2', 'node-3'], link_ids: ['link-2'] },
    ],
    ...overrides,
  };
}

export function createMockTracerouteResponse(overrides: Partial<TracerouteResponse> = {}): TracerouteResponse {
  return {
    hops: [
      { hop: 1, ip: '10.0.1.1', rtt: '0.016 ms', node_id: 'node-2' },
      { hop: 2, ip: '10.0.2.2', rtt: '0.009 ms', node_id: 'node-3' },
      { hop: 3, ip: '10.0.3.2', rtt: '0.010 ms', node_id: 'node-4' },
    ],
    node_ids: ['node-1', 'node-2', 'node-3', 'node-4'],
    link_ids: ['link-1', 'link-2', 'link-3'],
    ...overrides,
  };
}
