import { TestBed } from '@angular/core/testing';
import { LayoutService } from './layout.service';
import { describe, it, expect, beforeEach } from 'vitest';
import { createMockNode, createMockLink } from '../../../testing/mock-factories';

describe('LayoutService', () => {
  let service: LayoutService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(LayoutService);
  });

  describe('getNextNodePosition', () => {
    it('should return first grid position for empty topology', () => {
      const pos = service.getNextNodePosition([], []);
      expect(pos).toEqual({ x: 200, y: 150 });
    });

    it('should skip occupied positions', () => {
      const nodes = [createMockNode({ x: 200, y: 150 })];
      const pos = service.getNextNodePosition(nodes, []);
      expect(pos).toEqual({ x: 350, y: 150 });
    });

    it('should wrap to next row after 5 columns', () => {
      const nodes = Array.from({ length: 5 }, (_, i) =>
        createMockNode({ id: `n-${i}`, x: 200 + i * 150, y: 150 })
      );
      const pos = service.getNextNodePosition(nodes, []);
      expect(pos).toEqual({ x: 200, y: 270 });
    });

    it('should avoid positions near link segments', () => {
      const nodes = [
        createMockNode({ id: 'a', x: 200, y: 150 }),
        createMockNode({ id: 'b', x: 350, y: 150 }),
      ];
      const links = [createMockLink({ source: 'a', target: 'b' })];
      const pos = service.getNextNodePosition(nodes, links);
      // Should skip (200,150) and (350,150) occupied by nodes,
      // and (500,150) should be available
      expect(pos.x).toBeGreaterThanOrEqual(500);
    });

    it('should return fallback when all 100 positions exhausted', () => {
      // Create nodes covering all grid positions
      const nodes = Array.from({ length: 100 }, (_, i) =>
        createMockNode({
          id: `n-${i}`,
          x: 200 + (i % 5) * 150,
          y: 150 + Math.floor(i / 5) * 120,
        })
      );
      const pos = service.getNextNodePosition(nodes, []);
      expect(pos).toEqual({ x: 200, y: 150 });
    });
  });

  it('places a node at the center of a distant visible viewport', () => {
    expect(service.getNextNodePosition([], [], { x1: 1000, y1: -600, x2: 1600, y2: -200, zoom: 1 })).toEqual({ x: 1300, y: -400 });
  });

  it('finds free visible space instead of placing a node over a cable', () => {
    const nodes = [createMockNode({ id: 'a', x: 1000, y: 200 }), createMockNode({ id: 'b', x: 1600, y: 200 })];
    const pos = service.getNextNodePosition(nodes, [createMockLink({ source: 'a', target: 'b' })], { x1: 1050, y1: 50, x2: 1550, y2: 350, zoom: 1 });
    expect(pos.x).toBeGreaterThanOrEqual(1050); expect(pos.x).toBeLessThanOrEqual(1550);
    expect(Math.abs(pos.y - 200)).toBeGreaterThanOrEqual(55);
  });

  describe('getNextInterface', () => {
    it('should return eth0 when no links exist for node', () => {
      expect(service.getNextInterface('node-1', [])).toBe('eth0');
    });

    it('should return eth1 when eth0 is in use', () => {
      const links = [createMockLink({ source: 'node-1', source_int: 'eth0' })];
      expect(service.getNextInterface('node-1', links)).toBe('eth1');
    });

    it('should fill gaps in numbering', () => {
      const links = [
        createMockLink({ id: 'l1', source: 'node-1', source_int: 'eth0' }),
        createMockLink({ id: 'l2', source: 'node-1', source_int: 'eth2' }),
      ];
      expect(service.getNextInterface('node-1', links)).toBe('eth1');
    });

    it('should count both source and target links', () => {
      const links = [
        createMockLink({ id: 'l1', source: 'node-1', source_int: 'eth0' }),
        createMockLink({ id: 'l2', source: 'node-2', target: 'node-1', target_int: 'eth1' }),
      ];
      expect(service.getNextInterface('node-1', links)).toBe('eth2');
    });

    it('should handle interface names with IP suffix', () => {
      const links = [
        createMockLink({ id: 'l1', source: 'node-1', source_int: 'eth0\n10.0.0.1/24' }),
      ];
      expect(service.getNextInterface('node-1', links)).toBe('eth1');
    });
  });
});
