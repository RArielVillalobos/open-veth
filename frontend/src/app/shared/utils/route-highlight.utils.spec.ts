import { describe, it, expect } from 'vitest';
import { edgeMatchesSubnet, edgeMatchesGateway } from './route-highlight.utils';

// Helpers to build edge labels as they appear in Cytoscape data
const label = (iface: string, ip?: string) => ip ? `${iface}\n${ip}` : iface;

describe('edgeMatchesSubnet', () => {
  describe('both endpoints have IPs in the same subnet', () => {
    it('matches when both sides are in the target subnet', () => {
      expect(edgeMatchesSubnet(
        label('eth2', '10.0.2.1/24'),
        label('eth1', '10.0.2.2/24'),
        '10.0.2.0/24'
      )).toBe(true);
    });

    it('does not match when both sides are in a different subnet', () => {
      expect(edgeMatchesSubnet(
        label('eth1', '10.0.1.1/24'),
        label('eth1', '10.0.1.2/24'),
        '10.0.2.0/24'
      )).toBe(false);
    });
  });

  describe('one endpoint has IP, other has no IP (partial label)', () => {
    it('matches when source has IP in subnet and target has no IP', () => {
      expect(edgeMatchesSubnet(
        label('eth2', '10.0.2.1/24'),
        label('eth1'),
        '10.0.2.0/24'
      )).toBe(true);
    });

    it('matches when target has IP in subnet and source has no IP', () => {
      expect(edgeMatchesSubnet(
        label('eth2'),
        label('eth1', '10.0.2.2/24'),
        '10.0.2.0/24'
      )).toBe(true);
    });
  });

  describe('false positive prevention', () => {
    // Regresión: R2-PC2 edge falsamente iluminaba cuando se hacía hover en 10.0.2.0/24,
    // porque el label de R2 (10.0.2.2/24) aparecía en el target_int del edge R2-PC2.
    it('does not match when one side is in subnet but other is in a DIFFERENT subnet', () => {
      expect(edgeMatchesSubnet(
        label('eth1', '10.0.3.2/24'), // PC2's IP → 10.0.3.0/24
        label('eth1', '10.0.2.2/24'), // R2's eth1 IP → 10.0.2.0/24 (wrong endpoint label)
        '10.0.2.0/24'
      )).toBe(false);
    });

    it('does not match when both sides are in different subnets, neither matching target', () => {
      expect(edgeMatchesSubnet(
        label('eth1', '10.0.1.1/24'),
        label('eth1', '10.0.3.1/24'),
        '10.0.2.0/24'
      )).toBe(false);
    });
  });

  describe('edge cases', () => {
    it('does not match when neither endpoint has an IP', () => {
      expect(edgeMatchesSubnet(label('eth1'), label('eth2'), '10.0.2.0/24')).toBe(false);
    });

    it('does not match when both labels are empty strings', () => {
      expect(edgeMatchesSubnet('', '', '10.0.2.0/24')).toBe(false);
    });

    it('works with /30 subnets', () => {
      expect(edgeMatchesSubnet(
        label('eth0', '192.168.1.1/30'),
        label('eth0', '192.168.1.2/30'),
        '192.168.1.0/30'
      )).toBe(true);
    });

    it('works with /32 host routes', () => {
      expect(edgeMatchesSubnet(
        label('eth0', '10.0.0.1/32'),
        label('eth0'),
        '10.0.0.1/32'
      )).toBe(true);
    });

    it('does not confuse host IPs with network addresses', () => {
      // 10.0.2.1/24 → network 10.0.2.0/24, not 10.0.2.1/24
      expect(edgeMatchesSubnet(
        label('eth2', '10.0.2.1/24'),
        label('eth1', '10.0.2.2/24'),
        '10.0.2.1/24' // querying host address, not network
      )).toBe(false);
    });

    it('matches when both endpoints have the same IP (misconfigured link)', () => {
      expect(edgeMatchesSubnet(
        label('eth1', '10.0.2.1/24'),
        label('eth1', '10.0.2.1/24'),
        '10.0.2.0/24'
      )).toBe(true);
    });
  });
});

describe('edgeMatchesGateway', () => {
  it('matches when source label contains the gateway IP', () => {
    expect(edgeMatchesGateway(
      label('eth1', '10.0.2.2/24'),
      label('eth2', '10.0.3.1/24'),
      '10.0.2.2'
    )).toBe(true);
  });

  it('matches when target label contains the gateway IP', () => {
    expect(edgeMatchesGateway(
      label('eth2', '10.0.2.1/24'),
      label('eth1', '10.0.2.2/24'),
      '10.0.2.2'
    )).toBe(true);
  });

  it('does not match when neither label contains the gateway IP', () => {
    expect(edgeMatchesGateway(
      label('eth1', '10.0.1.1/24'),
      label('eth1', '10.0.1.2/24'),
      '10.0.2.2'
    )).toBe(false);
  });

  it('does not match partial IPs (10.0.2.2 should not match 10.0.2.22)', () => {
    expect(edgeMatchesGateway(
      label('eth1', '10.0.2.22/24'),
      label('eth1'),
      '10.0.2.2'
    )).toBe(false);
  });

  it('matches when endpoint has no prefix in label but IP matches', () => {
    // Edge case: label is just "eth1\n10.0.2.2/24"
    expect(edgeMatchesGateway(
      label('eth1'),
      label('eth1', '10.0.2.2/24'),
      '10.0.2.2'
    )).toBe(true);
  });

  it('matches when both endpoints contain the gateway IP', () => {
    expect(edgeMatchesGateway(
      label('eth1', '10.0.2.2/24'),
      label('eth1', '10.0.2.2/24'),
      '10.0.2.2'
    )).toBe(true);
  });

  it('does not match when both labels have no IP at all', () => {
    expect(edgeMatchesGateway(label('eth1'), label('eth2'), '10.0.2.2')).toBe(false);
  });

  it('does not match when both labels are empty strings', () => {
    expect(edgeMatchesGateway('', '', '10.0.2.2')).toBe(false);
  });
});
