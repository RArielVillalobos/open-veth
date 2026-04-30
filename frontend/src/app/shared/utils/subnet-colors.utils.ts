import { Link } from '../../models/topology.model';
import { parseNetworkAddress } from './network-utils';
import { DOMAIN_COLORS } from '../components/topology-canvas/domain-colors';

export interface SubnetColorInfo {
  subnet: string;
  colorIndex: number;
}

/**
 * Resolves subnet colors for links given node interface IPs.
 * If a link's source node has no IP (e.g. a switch), it falls back to the target node's IP.
 */
export function resolveSubnetColors(
  links: Link[],
  nodeIfaceIPs: Map<string, Map<string, string>>
): Map<string, SubnetColorInfo> {
  const networkCache = new Map<string, string>();
  nodeIfaceIPs.forEach(ifaceMap => {
    ifaceMap.forEach((ipWithPrefix) => {
      const net = parseNetworkAddress(ipWithPrefix);
      if (net) networkCache.set(ipWithPrefix, net);
    });
  });

  const allSubnets = new Set(networkCache.values());
  if (allSubnets.size < 2) return new Map();

  const subnetColorIndex = new Map<string, number>();
  [...allSubnets].forEach((subnet, i) => subnetColorIndex.set(subnet, i % DOMAIN_COLORS.length));

  const result = new Map<string, SubnetColorInfo>();

  for (const link of links) {
    const srcIP = nodeIfaceIPs.get(link.source)?.get(link.source_int);
    const tgtIP = nodeIfaceIPs.get(link.target)?.get(link.target_int);
    const ip = srcIP || tgtIP;
    if (!ip) continue;
    const net = networkCache.get(ip);
    if (!net) continue;
    const colorIndex = subnetColorIndex.get(net);
    if (colorIndex === undefined) continue;
    result.set(link.id, { subnet: net, colorIndex });
  }

  return result;
}
