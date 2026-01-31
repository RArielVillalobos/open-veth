export interface IPAddress {
  local: string;
  prefixlen: number;
}

export interface InterfaceInfo {
  ifname: string;
  addr_info: IPAddress[];
}

export interface Node {
  id: string;
  name: string;
  type: 'router' | 'switch' | 'host';
  image: string;
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
