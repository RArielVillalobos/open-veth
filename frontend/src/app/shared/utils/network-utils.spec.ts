import { describe, it, expect } from 'vitest';
import { parseNetworkAddress } from './network-utils';

describe('parseNetworkAddress', () => {
  it('returns network address for /30', () => {
    expect(parseNetworkAddress('10.0.1.1/30')).toBe('10.0.1.0/30');
    expect(parseNetworkAddress('10.0.1.2/30')).toBe('10.0.1.0/30');
  });

  it('returns network address for /24', () => {
    expect(parseNetworkAddress('192.168.1.100/24')).toBe('192.168.1.0/24');
    expect(parseNetworkAddress('192.168.1.1/24')).toBe('192.168.1.0/24');
  });

  it('returns network address for /16', () => {
    expect(parseNetworkAddress('172.16.5.10/16')).toBe('172.16.0.0/16');
  });

  it('handles /0 (default route)', () => {
    expect(parseNetworkAddress('10.0.0.1/0')).toBe('0.0.0.0/0');
  });

  it('handles /32 (host route)', () => {
    expect(parseNetworkAddress('10.0.0.1/32')).toBe('10.0.0.1/32');
  });

  it('returns null for invalid input', () => {
    expect(parseNetworkAddress('invalid')).toBeNull();
    expect(parseNetworkAddress('10.0.0.1')).toBeNull();
    expect(parseNetworkAddress('')).toBeNull();
    expect(parseNetworkAddress('abc.def.ghi.jkl/24')).toBeNull();
  });
});
