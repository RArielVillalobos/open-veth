import { Injectable } from '@angular/core';
import { Node, Link } from '../../models/topology.model';

@Injectable({
  providedIn: 'root'
})
export class LayoutService {

  /**
   * Calculates the next available position for a node to avoid overlapping
   */
  getNextNodePosition(nodes: Node[], links: Link[], viewport?: { x1: number; y1: number; x2: number; y2: number; zoom: number }): { x: number; y: number } {
    if (viewport) {
      const center = { x: (viewport.x1 + viewport.x2) / 2, y: (viewport.y1 + viewport.y2) / 2 };
      const stepX = 150 / viewport.zoom, stepY = 120 / viewport.zoom;
      const candidates: { x: number; y: number }[] = [];
      for (let ring = 0; ring <= 12; ring++) {
        for (let row = -ring; row <= ring; row++) for (let col = -ring; col <= ring; col++) {
          if (Math.max(Math.abs(row), Math.abs(col)) === ring) candidates.push({ x: center.x + col * stepX, y: center.y + row * stepY });
        }
      }
      candidates.sort((a, b) => Math.hypot(a.x - center.x, a.y - center.y) - Math.hypot(b.x - center.x, b.y - center.y));
      const free = (pos: { x: number; y: number }) => !nodes.some(n => Math.abs((n.x ?? 0) - pos.x) < 90 && Math.abs((n.y ?? 0) - pos.y) < 90)
        && !links.some(l => { const a = nodes.find(n => n.id === l.source), b = nodes.find(n => n.id === l.target);
          return a && b && this.pointToSegmentDist(pos.x, pos.y, a.x ?? 0, a.y ?? 0, b.x ?? 0, b.y ?? 0) < 55; });
      const visible = (pos: { x: number; y: number }) => pos.x >= viewport.x1 && pos.x <= viewport.x2 && pos.y >= viewport.y1 && pos.y <= viewport.y2;
      return candidates.find(pos => visible(pos) && free(pos)) ?? candidates.find(free) ?? { x: Math.max(...nodes.map(n => n.x ?? 0), center.x) + 180, y: center.y };
    }
    const cols = 5;
    const spacingX = 150;
    const spacingY = 120;
    const minNodeDist = 80;
    const minLinkDist = 50;

    for (let i = 0; i < 100; i++) {
      const x = 200 + (i % cols) * spacingX;
      const y = 150 + Math.floor(i / cols) * spacingY;

      // Avoid nodes
      if (nodes.some(n => Math.abs((n.x ?? 0) - x) < minNodeDist && Math.abs((n.y ?? 0) - y) < minNodeDist)) {
        continue;
      }

      // Avoid being exactly on top of a link
      const onLink = links.some(link => {
        const src = nodes.find(n => n.id === link.source);
        const tgt = nodes.find(n => n.id === link.target);
        if (!src || !tgt) return false;
        return this.pointToSegmentDist(x, y, src.x ?? 0, src.y ?? 0, tgt.x ?? 0, tgt.y ?? 0) < minLinkDist;
      });

      if (!onLink) return { x, y };
    }
    
    return { x: 200, y: 150 };
  }

  /**
   * Calculates distance from a point to a line segment
   */
  private pointToSegmentDist(px: number, py: number, ax: number, ay: number, bx: number, by: number): number {
    const dx = bx - ax;
    const dy = by - ay;
    const lenSq = dx * dx + dy * dy;
    
    if (lenSq === 0) return Math.hypot(px - ax, py - ay);
    
    const t = Math.max(0, Math.min(1, ((px - ax) * dx + (py - ay) * dy) / lenSq));
    return Math.hypot(px - (ax + t * dx), py - (ay + t * dy));
  }

  /**
   * Calculates a robust dynamic interface name (e.g., eth0, eth1) for a node
   */
  getNextInterface(nodeId: string, links: Link[]): string {
    const usedNames = links
      .filter(l => l.source === nodeId || l.target === nodeId)
      .map(l => (l.source === nodeId ? l.source_int : l.target_int));

    const usedNumbers = usedNames
      .map(name => {
        // Handle cases where label might include IP (e.g. "eth0\n10.0.0.1/24")
        const cleanName = name.split('\n')[0];
        return parseInt(cleanName.replace('eth', ''), 10);
      })
      .filter(n => !isNaN(n))
      .sort((a, b) => a - b);

    let nextNum = 0;
    for (const num of usedNumbers) {
      if (num === nextNum) {
        nextNum++;
      } else if (num > nextNum) {
        break;
      }
    }
    return `eth${nextNum}`;
  }
}
