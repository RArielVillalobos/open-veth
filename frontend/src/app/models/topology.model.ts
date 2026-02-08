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

export interface Node {
  id: string;
  name: string;
  type: 'router' | 'switch' | 'host';
  // Image removed: Backend managed
  x?: number;
  y?: number;
  status?: 'pending' | 'running' | 'error';
  interfaces?: InterfaceInfo[]; // Runtime info
}

export interface Link {
  id: string;
  source: string;
  target: string;
  source_int: string;
  target_int: string;
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
}

