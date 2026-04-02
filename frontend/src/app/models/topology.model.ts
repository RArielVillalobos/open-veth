export interface IPAddress {
  local: string;
  prefixlen: number;
}

export interface InterfaceInfo {
  ifname: string;
  addr_info: IPAddress[];
}

export interface RouteInfo {
  dst: string;
  gateway?: string;
  dev: string;
  protocol: string;
  scope: string;
  prefsrc?: string;
  metric?: number;
}

export interface MacEntry {
  mac: string;
  port: string;
  type: 'static' | 'dynamic';
}

export type NodeType = 'router' | 'switch' | 'host' | 'hub' | 'cloud' | 'linux' | 'server' | 'monitor' | 'tester';

export interface Node {
  id: string;
  name: string;
  type: NodeType;
  // Image removed: Backend managed
  x?: number;
  y?: number;
  status?: 'pending' | 'running' | 'stopped' | 'error';
  snapshot_image?: string; // Set after "Save State" — node will boot from this image
  container_id?: string; // Runtime info
  interfaces?: InterfaceInfo[]; // Runtime info
  service_ports?: Record<string, number>; // Runtime info — only for monitor nodes
}

export interface Link {
  id: string;
  source: string;
  target: string;
  source_int: string;
  target_int: string;
  enabled?: boolean;
}

export interface Topology {
  id: string;
  name: string;
  nodes: Node[];
  links: Link[];
}

export interface Laboratory {
  id: string;
  name: string;
  status: string;
  created_at?: string;
}

export interface LaboratoryCreate {
  id: string;
  name: string;
}

export interface SaveStateResponse {
  message: string;
  ips_saved: number;
  routes_saved: number;
  snapshots_saved: number;
}

export interface Domain {
  id: number;
  node_ids: string[];
  link_ids: string[];
}

export interface DomainsResponse {
  broadcast_domains: Domain[];
  collision_domains: Domain[];
}

export interface TracerouteHop {
  hop: number;
  ip: string;
  rtt: string;
  node_id?: string;
}

export interface TracerouteResponse {
  hops: TracerouteHop[];
  node_ids: string[];
  link_ids: string[];
  unreachable?: boolean;
}

