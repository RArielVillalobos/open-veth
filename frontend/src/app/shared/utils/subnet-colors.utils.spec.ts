import { describe, it, expect } from 'vitest';
import { resolveSubnetColors } from './subnet-colors.utils';
import { Link } from '../../models/topology.model';

function makeLink(id: string, source: string, target: string, source_int: string, target_int: string): Link {
  return { id, source, target, source_int, target_int };
}

describe('resolveSubnetColors', () => {
  it('returns empty map when there is only one subnet', () => {
    const links = [makeLink('l1', 'pc1', 'sw1', 'eth1', 'eth1')];
    const nodeIfaceIPs = new Map<string, Map<string, string>>([
      ['pc1', new Map([['eth1', '192.168.1.10/24']])],
      ['sw1', new Map([['eth1', '192.168.1.1/24']])],
    ]);
    expect(resolveSubnetColors(links, nodeIfaceIPs).size).toBe(0);
  });

  it('returns empty map when no node has an IP', () => {
    const links = [makeLink('l1', 'sw1', 'srv1', 'eth1', 'eth1')];
    const nodeIfaceIPs = new Map<string, Map<string, string>>([
      ['sw1', new Map()],
      ['srv1', new Map()],
    ]);
    expect(resolveSubnetColors(links, nodeIfaceIPs).size).toBe(0);
  });

  it('colors a link using the source IP when available', () => {
    const links = [makeLink('l1', 'pc1', 'sw1', 'eth1', 'eth1')];
    const nodeIfaceIPs = new Map<string, Map<string, string>>([
      ['pc1', new Map([['eth1', '192.168.1.10/24']])],
      ['sw1', new Map()],
      ['srv1', new Map([['eth1', '10.0.0.20/24']])],
    ]);
    const result = resolveSubnetColors(links, nodeIfaceIPs);
    expect(result.get('l1')).toEqual({ subnet: '192.168.1.0/24', colorIndex: 0 });
  });

  it('colors a link using the target IP when source has no IP (switch in the middle)', () => {
    const links = [
      makeLink('l1', 'pc1', 'sw1', 'eth1', 'eth1'),
      makeLink('l2', 'sw1', 'srv1', 'eth2', 'eth1'),
      makeLink('l3', 'r1', 'sw2', 'eth1', 'eth1'),
    ];
    const nodeIfaceIPs = new Map<string, Map<string, string>>([
      ['pc1', new Map([['eth1', '192.168.1.10/24']])],
      ['sw1', new Map()], // switch has no IP on access ports
      ['srv1', new Map([['eth1', '192.168.1.20/24']])],
      ['r1', new Map([['eth1', '10.0.0.1/24']])],
      ['sw2', new Map()],
    ]);
    const result = resolveSubnetColors(links, nodeIfaceIPs);
    // l1: source pc1 has IP -> 192.168.1.0/24
    expect(result.get('l1')).toEqual({ subnet: '192.168.1.0/24', colorIndex: 0 });
    // l2: source sw1 has no IP, fallback to target srv1 -> 192.168.1.0/24
    expect(result.get('l2')).toEqual({ subnet: '192.168.1.0/24', colorIndex: 0 });
    // l3: second subnet ensures coloring is active
    expect(result.get('l3')).toEqual({ subnet: '10.0.0.0/24', colorIndex: 1 });
  });

  it('assigns different color indexes for different subnets', () => {
    const links = [
      makeLink('l1', 'pc1', 'sw1', 'eth1', 'eth1'),
      makeLink('l2', 'r1', 'sw2', 'eth1', 'eth1'),
    ];
    const nodeIfaceIPs = new Map<string, Map<string, string>>([
      ['pc1', new Map([['eth1', '192.168.1.10/24']])],
      ['sw1', new Map()],
      ['r1', new Map([['eth1', '10.0.0.1/24']])],
      ['sw2', new Map()],
    ]);
    const result = resolveSubnetColors(links, nodeIfaceIPs);
    expect(result.get('l1')?.subnet).toBe('192.168.1.0/24');
    expect(result.get('l2')?.subnet).toBe('10.0.0.0/24');
    expect(result.get('l1')?.colorIndex).not.toBe(result.get('l2')?.colorIndex);
  });

  it('skips links where neither source nor target have an IP', () => {
    const links = [
      makeLink('l1', 'pc1', 'sw1', 'eth1', 'eth1'),
      makeLink('l2', 'sw1', 'sw2', 'eth2', 'eth1'),
      makeLink('l3', 'r1', 'sw3', 'eth1', 'eth1'),
    ];
    const nodeIfaceIPs = new Map<string, Map<string, string>>([
      ['pc1', new Map([['eth1', '192.168.1.10/24']])],
      ['sw1', new Map()],
      ['sw2', new Map()],
      ['r1', new Map([['eth1', '10.0.0.1/24']])],
      ['sw3', new Map()],
    ]);
    const result = resolveSubnetColors(links, nodeIfaceIPs);
    expect(result.has('l1')).toBe(true);
    expect(result.has('l2')).toBe(false);
    expect(result.has('l3')).toBe(true);
  });
});
