export function parseNetworkAddress(ipWithPrefix: string): string | null {
  const [ip, prefixStr] = ipWithPrefix.split('/');
  if (!ip || !prefixStr) return null;
  const prefixlen = parseInt(prefixStr, 10);
  const parts = ip.split('.').map(Number);
  if (parts.length !== 4 || parts.some(isNaN)) return null;
  const mask = prefixlen === 0 ? 0 : (~((1 << (32 - prefixlen)) - 1)) >>> 0;
  const addr = ((parts[0] << 24) | (parts[1] << 16) | (parts[2] << 8) | parts[3]) >>> 0;
  const net = (addr & mask) >>> 0;
  return `${(net >>> 24) & 0xff}.${(net >>> 16) & 0xff}.${(net >>> 8) & 0xff}.${net & 0xff}/${prefixlen}`;
}
