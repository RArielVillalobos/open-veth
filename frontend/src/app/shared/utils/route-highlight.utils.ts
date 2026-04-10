import { parseNetworkAddress } from './network-utils';

const IP_REGEX = /(\d+\.\d+\.\d+\.\d+\/\d+)/;

/**
 * Returns true if an edge (identified by its endpoint labels) belongs to the given subnet.
 *
 * Rules:
 * - At least one endpoint must have an IP in the subnet.
 * - The other endpoint must either have no IP, or also be in the same subnet.
 * - If one endpoint has an IP in a DIFFERENT subnet, it's not a match (avoids false positives
 *   when an interface appears in the label of an edge it's not part of).
 */
export function edgeMatchesSubnet(sourceInt: string, targetInt: string, subnet: string): boolean {
  const srcMatch = sourceInt.match(IP_REGEX);
  const tgtMatch = targetInt.match(IP_REGEX);
  const srcNet = srcMatch ? parseNetworkAddress(srcMatch[1]) : null;
  const tgtNet = tgtMatch ? parseNetworkAddress(tgtMatch[1]) : null;
  const inSubnet = (net: string | null) => net !== null && net === subnet;
  const compatible = (net: string | null) => net === null || net === subnet;
  return (inSubnet(srcNet) && compatible(tgtNet)) || (inSubnet(tgtNet) && compatible(srcNet));
}

/**
 * Returns true if an edge label contains the given gateway IP.
 * Uses "gateway/" suffix to avoid partial IP matches (e.g. 10.0.2.2 vs 10.0.2.22).
 */
export function edgeMatchesGateway(sourceInt: string, targetInt: string, gateway: string): boolean {
  const prefix = gateway + '/';
  return sourceInt.includes(prefix) || targetInt.includes(prefix);
}
