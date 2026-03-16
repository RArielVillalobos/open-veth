import { Component, ElementRef, ViewChild, AfterViewInit, input, output, effect, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { DragDropModule } from '@angular/cdk/drag-drop';
import cytoscape from 'cytoscape';
import { Node as TopologyNode, Link, DomainsResponse, TracerouteResponse } from '../../../models/topology.model';
import { DOMAIN_COLORS } from './domain-colors';
import { CYTOSCAPE_STYLES } from './cytoscape-styles';

@Component({
  selector: 'app-topology-canvas',
  standalone: true,
  imports: [CommonModule, FormsModule, DragDropModule],
  templateUrl: './topology-canvas.component.html',
  styleUrl: './topology-canvas.component.scss'
})
export class TopologyCanvasComponent implements AfterViewInit, OnDestroy {
  @ViewChild('cyContainer') container!: ElementRef;
  @ViewChild('domainCanvas') domainCanvas!: ElementRef<HTMLCanvasElement>;
  
  nodes = input.required<TopologyNode[]>();
  links = input.required<Link[]>();
  terminalNode = input<string | null>(null);
  domains = input<DomainsResponse | null>(null);
  activeDomainOverlay = input<'broadcast' | 'collision' | null>(null);
  traceroutePath = input<TracerouteResponse | null>(null);
  linkRequest = output<{source: string, target: string}>();
  openTerminalRequest = output<string>();
  openSniffRequest = output<{nodeId: string, nodeName: string, iface: string}>();
  tracerouteRequest = output<{nodeId: string, destination: string}>();
  tracerouteClose = output<void>();
  nodeMoved = output<{id: string, x: number, y: number}>();
  nodeSelected = output<string | null>();
  linkSelected = output<string | null>();
  nodeDelete = output<string>();
  linkDelete = output<string>();
  linkToggle = output<string>();

  private cy!: cytoscape.Core;
  private canvasReady = false;
  private resizeObserver!: ResizeObserver;
  private overlayCtx: CanvasRenderingContext2D | null = null;
  sourceNodeId: string | null = null;
  
  // Context menu state
  contextMenu = {
    visible: false,
    x: 0,
    y: 0,
    elementId: '',
    elementName: '',
    elementType: '',
    linkEnabled: true,
    // Optional data for links
    sourceNode: { nodeId: '', nodeName: '', iface: '' },
    targetNode: { nodeId: '', nodeName: '', iface: '' }
  };

  // Traceroute dialog state
  showTracerouteDialog = false;
  tracerouteNodeId = '';
  tracerouteNodeName = '';
  tracerouteDestIP = '';
  tracerouteLoading = false;
  tracerouteResult: TracerouteResponse | null = null;
  private tracerouteHopMap = new Map<string, string>(); // nodeId -> label ("S", "1", "2"...)

  constructor() {
    effect(() => {
      this.nodes();
      if (this.cy) {
        this.updateCanvas();
      }
    });

    effect(() => {
      const nodeId = this.terminalNode();
      if (!this.cy) return;
      this.cy.nodes().removeClass('terminal-active');
      if (nodeId) {
        const match = this.cy.getElementById(nodeId);
        if (!match.empty()) match.addClass('terminal-active');
      }
    });

    // Domain overlay effect
    effect(() => {
      const domains = this.domains();
      const overlay = this.activeDomainOverlay();
      if (!this.cy) return;
      this.clearDomainOverlay();
      if (!overlay || !domains) return;
      this.applyDomainOverlay(domains, overlay);
    });

    // Traceroute path effect
    effect(() => {
      const path = this.traceroutePath();
      if (!this.cy) return;
      this.clearTraceroutePath();
      if (!path) return;
      path.node_ids?.forEach(nodeId => {
        const el = this.cy.getElementById(nodeId);
        if (!el.empty()) el.addClass('traceroute-path');
      });
      path.link_ids?.forEach(linkId => {
        const el = this.cy.getElementById(linkId);
        if (!el.empty()) el.addClass('traceroute-path');
      });
      // Build hop map: source = "S", then hop numbers
      this.tracerouteHopMap.clear();
      if (path.node_ids.length > 0) {
        this.tracerouteHopMap.set(path.node_ids[0], 'S');
      }
      path.hops.forEach(hop => {
        if (hop.node_id) {
          this.tracerouteHopMap.set(hop.node_id, String(hop.hop));
        }
      });
      this.drawTracerouteHopBadges();
    });
  }

  ngAfterViewInit() {
    this.initCytoscape();
  }

  ngOnDestroy() {
    this.resizeObserver?.disconnect();
    if (this.cy) this.cy.destroy();
  }

  getNodeName(id: string): string {
    return this.nodes().find(n => n.id === id)?.name || id;
  }

  cancelLinking() {
    this.sourceNodeId = null;
    this.cy.nodes().removeClass('selected-source');
  }

  closeContextMenu() {
    this.contextMenu = { ...this.contextMenu, visible: false };
  }

  onContextMenuAction(action: 'terminal' | 'delete' | 'properties' | 'traceroute' | 'toggle') {
    if (action === 'terminal') {
      this.openTerminalRequest.emit(this.contextMenu.elementId);
    } else if (action === 'traceroute') {
      this.openTracerouteDialog(this.contextMenu.elementId, this.contextMenu.elementName);
    } else if (action === 'properties') {
      if (this.contextMenu.elementType === 'edge') {
        this.linkSelected.emit(this.contextMenu.elementId);
      } else {
        this.nodeSelected.emit(this.contextMenu.elementId);
      }
    } else if (action === 'toggle') {
      this.linkToggle.emit(this.contextMenu.elementId);
    } else if (action === 'delete') {
      if (this.contextMenu.elementType === 'edge') {
        this.linkDelete.emit(this.contextMenu.elementId);
      } else {
        this.nodeDelete.emit(this.contextMenu.elementId);
      }
    }
    this.closeContextMenu();
  }

  triggerOpenSniff(data: {nodeId: string, nodeName: string, iface: string}) {
    this.openSniffRequest.emit(data);
    this.closeContextMenu();
  }

  openTracerouteDialog(nodeId: string, nodeName: string) {
    this.tracerouteNodeId = nodeId;
    this.tracerouteNodeName = nodeName;
    this.tracerouteDestIP = '';
    this.tracerouteResult = null;
    this.tracerouteLoading = false;
    this.showTracerouteDialog = true;
  }

  runTraceroute() {
    if (!this.tracerouteDestIP || this.tracerouteLoading) return;
    const ipv4 = /^(\d{1,3}\.){3}\d{1,3}$/;
    const ipv6 = /^[0-9a-fA-F:]+$/;
    if (!ipv4.test(this.tracerouteDestIP) && !ipv6.test(this.tracerouteDestIP)) {
      alert('Please enter a valid IP address (e.g. 10.0.0.1)');
      return;
    }
    this.tracerouteLoading = true;
    this.tracerouteRequest.emit({
      nodeId: this.tracerouteNodeId,
      destination: this.tracerouteDestIP
    });
  }

  fitToView() {
    if (this.cy) {
      this.cy.fit(undefined, 50);
    }
  }

  onTracerouteResult(result: TracerouteResponse) {
    this.tracerouteResult = result;
    this.tracerouteLoading = false;
  }

  onTracerouteError() {
    this.tracerouteLoading = false;
  }

  clearTraceroute() {
    this.showTracerouteDialog = false;
    this.tracerouteResult = null;
    this.clearTraceroutePath();
    this.tracerouteClose.emit();
  }

  private clearTraceroutePath(): void {
    if (!this.cy) return;
    this.cy.elements().removeClass('traceroute-path');
    this.tracerouteHopMap.clear();
    this.redrawOverlayCanvas();
  }

  private initCytoscape() {
    this.cy = cytoscape({
      container: this.container.nativeElement,
      style: CYTOSCAPE_STYLES,
      pixelRatio: 'auto',
      minZoom: 0.3,
      maxZoom: 1.5,
      wheelSensitivity: 0.3
    });

    // Expose for e2e testing
    (this.container.nativeElement as any).__cy = this.cy;

    // Recalculate canvas when container resizes
    this.resizeObserver = new ResizeObserver(() => {
      this.cy.resize();
      this.redrawOverlayCanvas();
    });
    this.resizeObserver.observe(this.container.nativeElement);

    // --- Event Listeners ---

    // Tap on Node (Start/End Link)
    this.cy.on('tap', 'node', (evt) => {
      const clickedNode = evt.target;
      const clickedId = clickedNode.id();

      if (!this.sourceNodeId) {
        // Mode: Start Link
        this.sourceNodeId = clickedId;
        clickedNode.addClass('selected-source');
      } else {
        // Mode: End Link
        if (this.sourceNodeId !== clickedId) {
          this.linkRequest.emit({
            source: this.sourceNodeId,
            target: clickedId
          });
          this.cancelLinking();
        }
      }
    });

    // Tap on Edge (Select Link)
    this.cy.on('tap', 'edge', (evt) => {
      const edge = evt.target;
      this.linkSelected.emit(edge.id());
    });

    // Right Click (Context Menu) - Node
    this.cy.on('cxttap', 'node', (evt) => {
      const node = evt.target;
      const pos = evt.renderedPosition;

      this.contextMenu = {
        visible: true,
        x: pos.x + 20,
        y: pos.y + 20,
        elementId: node.id(),
        elementName: node.data('name'),
        elementType: node.data('type'),
        linkEnabled: true,
        sourceNode: { nodeId: '', nodeName: '', iface: '' },
        targetNode: { nodeId: '', nodeName: '', iface: '' }
      };
    });

    // Right Click (Context Menu) - Edge
    this.cy.on('cxttap', 'edge', (evt) => {
      const edge = evt.target;
      const data = edge.data();
      const pos = evt.renderedPosition;

      // Look up full node objects to get names
      const sNode = this.nodes().find(n => n.id === data.source);
      const tNode = this.nodes().find(n => n.id === data.target);
      const linkObj = this.links().find(l => l.id === edge.id());

      this.contextMenu = {
        visible: true,
        x: pos.x + 20,
        y: pos.y + 20,
        elementId: edge.id(),
        elementName: 'Link',
        elementType: 'edge',
        linkEnabled: linkObj?.enabled ?? true,
        // Populate with correct structure for the event
        sourceNode: {
            nodeId: data.source,
            nodeName: sNode?.name || 'A',
            iface: data.source_int.split('\n')[0]
        },
        targetNode: {
            nodeId: data.target,
            nodeName: tNode?.name || 'B',
            iface: data.target_int.split('\n')[0]
        }
      };
    });

    // Tap Background (Close everything)
    this.cy.on('tap', (evt) => {
      this.closeContextMenu();
      if (evt.target === this.cy) {
        this.cancelLinking();
        this.nodeSelected.emit(null);
        this.linkSelected.emit(null);
        if (!this.showTracerouteDialog) {
          this.clearTraceroutePath();
        }
      }
    });

    this.cy.on('zoom pan', () => {
      this.closeContextMenu();
      this.redrawOverlayCanvas();
    });

    this.cy.on('drag', 'node', () => this.redrawOverlayCanvas());

    // Drag End (Update Position)
    this.cy.on('dragfree', 'node', (evt) => {
      const node = evt.target;
      const pos = node.position();
      this.nodeMoved.emit({
        id: node.id(),
        x: pos.x,
        y: pos.y
      });
    });
  }

  private clearDomainOverlay(): void {
    const classPattern = /\bdomain-(?:bd|cd)-\d+\b/g;
    this.cy.elements().forEach(ele => {
      const classes = ele.classes();
      classes.forEach(cls => {
        if (classPattern.test(cls)) {
          ele.removeClass(cls);
        }
        classPattern.lastIndex = 0;
      });
    });
    this.clearDomainCanvas();
  }

  private applyDomainOverlay(domains: DomainsResponse, mode: 'broadcast' | 'collision'): void {
    const prefix = mode === 'broadcast' ? 'bd' : 'cd';
    const domainList = mode === 'broadcast' ? domains.broadcast_domains : domains.collision_domains;

    domainList.forEach((domain, index) => {
      const colorIndex = index % DOMAIN_COLORS.length;
      const className = `domain-${prefix}-${colorIndex}`;

      // Color links in both modes
      domain.link_ids.forEach(linkId => {
        const el = this.cy.getElementById(linkId);
        if (!el.empty()) el.addClass(className);
      });

      // In CD mode, only color links
      if (mode === 'collision') return;

      // In BD mode, also color nodes
      domain.node_ids.forEach(nodeId => {
        const el = this.cy.getElementById(nodeId);
        if (!el.empty()) el.addClass(className);
      });
    });

    // Draw polygons for broadcast domains
    if (mode === 'broadcast') {
      this.drawDomainPolygons(domainList);
    }
  }

  private redrawOverlayCanvas(): void {
    this.redrawDomainPolygons();
    this.drawTracerouteHopBadges();
  }

  private redrawDomainPolygons(): void {
    const domains = this.domains();
    const overlay = this.activeDomainOverlay();
    if (overlay === 'broadcast' && domains) {
      this.drawDomainPolygons(domains.broadcast_domains);
    } else {
      this.clearDomainCanvas();
    }
  }

  private clearDomainCanvas(): void {
    if (!this.domainCanvas) return;
    const canvas = this.domainCanvas.nativeElement;
    const ctx = this.overlayCtx ?? canvas.getContext('2d');
    if (ctx) ctx.clearRect(0, 0, canvas.width, canvas.height);
  }

  private setupOverlayCanvas(): CanvasRenderingContext2D | null {
    if (!this.domainCanvas) return null;
    const canvas = this.domainCanvas.nativeElement;
    if (!canvas.parentElement) return null;
    const rect = canvas.parentElement.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return null;
    const dpr = window.devicePixelRatio || 1;
    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;
    canvas.style.width = rect.width + 'px';
    canvas.style.height = rect.height + 'px';
    if (!this.overlayCtx) {
      this.overlayCtx = canvas.getContext('2d');
    }
    if (!this.overlayCtx) return null;
    this.overlayCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
    return this.overlayCtx;
  }

  private drawTracerouteHopBadges(): void {
    if (!this.cy || this.tracerouteHopMap.size === 0) return;
    const ctx = this.setupOverlayCanvas();
    if (!ctx) return;
    ctx.save();

    const radius = 12;
    this.tracerouteHopMap.forEach((label, nodeId) => {
      const el = this.cy.getElementById(nodeId);
      if (el.empty()) return;

      const pos = el.renderedPosition();

      // Position badge at top-right of the node icon
      const bx = pos.x + 22;
      const by = pos.y - 22;

      // Circle background
      ctx.beginPath();
      ctx.arc(bx, by, radius, 0, Math.PI * 2);
      ctx.fillStyle = '#16a34a';
      ctx.fill();
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 2;
      ctx.stroke();

      // Label text
      ctx.fillStyle = '#ffffff';
      ctx.font = 'bold 11px sans-serif';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(label, bx, by);
    });

    ctx.restore();
  }

  private drawDomainPolygons(domainList: DomainsResponse['broadcast_domains']): void {
    if (!this.cy) return;
    const ctx = this.setupOverlayCanvas();
    if (!ctx) return;

    domainList.forEach((domain, index) => {
      const color = DOMAIN_COLORS[index % DOMAIN_COLORS.length];

      // Gather rendered positions of domain nodes
      const points: { x: number; y: number }[] = [];
      domain.node_ids.forEach(nodeId => {
        const el = this.cy.getElementById(nodeId);
        if (!el.empty()) {
          const pos = el.renderedPosition();
          points.push({ x: pos.x, y: pos.y });
        }
      });

      if (points.length < 2) return; // Need at least 2 nodes for a polygon

      const hull = this.convexHull(points);
      const padding = 40;

      ctx.beginPath();
      if (hull.length === 2) {
        // Draw a rounded rectangle between 2 points
        this.drawRoundedSegment(ctx, hull[0], hull[1], padding);
      } else {
        // Draw expanded convex hull with rounded corners
        this.drawExpandedHull(ctx, hull, padding);
      }

      // Fill
      ctx.fillStyle = color + '15'; // ~8% opacity
      ctx.fill();

      // Stroke
      ctx.strokeStyle = color + '40'; // ~25% opacity
      ctx.lineWidth = 2;
      ctx.setLineDash([6, 4]);
      ctx.stroke();
      ctx.setLineDash([]);
    });
  }

  private convexHull(points: { x: number; y: number }[]): { x: number; y: number }[] {
    if (points.length <= 2) return [...points];

    // Andrew's monotone chain algorithm
    const sorted = [...points].sort((a, b) => a.x - b.x || a.y - b.y);
    const cross = (O: any, A: any, B: any) =>
      (A.x - O.x) * (B.y - O.y) - (A.y - O.y) * (B.x - O.x);

    const lower: any[] = [];
    for (const p of sorted) {
      while (lower.length >= 2 && cross(lower[lower.length - 2], lower[lower.length - 1], p) <= 0)
        lower.pop();
      lower.push(p);
    }
    const upper: any[] = [];
    for (const p of [...sorted].reverse()) {
      while (upper.length >= 2 && cross(upper[upper.length - 2], upper[upper.length - 1], p) <= 0)
        upper.pop();
      upper.push(p);
    }
    upper.pop();
    lower.pop();
    return lower.concat(upper);
  }

  private drawRoundedSegment(ctx: CanvasRenderingContext2D, a: { x: number; y: number }, b: { x: number; y: number }, padding: number): void {
    const dx = b.x - a.x;
    const dy = b.y - a.y;
    const len = Math.sqrt(dx * dx + dy * dy);
    const nx = -dy / len * padding;
    const ny = dx / len * padding;

    // Draw a stadium shape (rectangle with semicircle caps)
    const angle = Math.atan2(dy, dx);
    ctx.arc(a.x, a.y, padding, angle + Math.PI / 2, angle - Math.PI / 2);
    ctx.arc(b.x, b.y, padding, angle - Math.PI / 2, angle + Math.PI / 2);
    ctx.closePath();
  }

  private drawExpandedHull(ctx: CanvasRenderingContext2D, hull: { x: number; y: number }[], padding: number): void {
    // Expand hull outward and draw with rounded corners
    const n = hull.length;
    const expanded: { x: number; y: number }[] = [];

    for (let i = 0; i < n; i++) {
      const curr = hull[i];
      const prev = hull[(i - 1 + n) % n];
      const next = hull[(i + 1) % n];

      // Outward direction at this vertex (average of two edge normals)
      const dx1 = curr.x - prev.x, dy1 = curr.y - prev.y;
      const dx2 = next.x - curr.x, dy2 = next.y - curr.y;
      const nx1 = -dy1, ny1 = dx1;
      const nx2 = -dy2, ny2 = dx2;
      const len1 = Math.sqrt(nx1 * nx1 + ny1 * ny1) || 1;
      const len2 = Math.sqrt(nx2 * nx2 + ny2 * ny2) || 1;
      const ox = (nx1 / len1 + nx2 / len2) / 2;
      const oy = (ny1 / len1 + ny2 / len2) / 2;
      const olen = Math.sqrt(ox * ox + oy * oy) || 1;

      expanded.push({
        x: curr.x + (ox / olen) * padding,
        y: curr.y + (oy / olen) * padding
      });
    }

    // Draw with quadratic curves for rounded corners
    ctx.moveTo(
      (expanded[n - 1].x + expanded[0].x) / 2,
      (expanded[n - 1].y + expanded[0].y) / 2
    );
    for (let i = 0; i < n; i++) {
      const next = (i + 1) % n;
      ctx.quadraticCurveTo(
        expanded[i].x, expanded[i].y,
        (expanded[i].x + expanded[next].x) / 2,
        (expanded[i].y + expanded[next].y) / 2
      );
    }
    ctx.closePath();
  }

  private updateCanvas(): void {
    // Access signal value once
    const nodes = this.nodes();
    const newNodeIds: string[] = [];

    this.cy.batch(() => {
      // Build a lookup: nodeId -> { ifaceName -> ip }
      const nodeIfaceIPs = new Map<string, Map<string, string>>();
      nodes.forEach(node => {
        const ifaceMap = new Map<string, string>();
        if (node.interfaces) {
          for (const iface of node.interfaces) {
            if (iface.ifname === 'lo' || iface.ifname === 'mgmt0') continue;
            const ipv4 = iface.addr_info?.find(a => !a.local.includes(':'));
            if (ipv4) {
              ifaceMap.set(iface.ifname, `${ipv4.local}/${ipv4.prefixlen}`);
            }
          }
        }
        nodeIfaceIPs.set(node.id, ifaceMap);
      });

      // 1. Add/Update Nodes
      nodes.forEach(node => {
        const existing = this.cy.getElementById(node.id);
        if (existing.empty()) {
          this.cy.add({
            group: 'nodes',
            data: { id: node.id, label: node.name, name: node.name, type: node.type },
            position: { x: node.x || 100, y: node.y || 100 }
          });
          if (this.canvasReady) newNodeIds.push(node.id);
        } else {
          if (existing.data('label') !== node.name) {
            existing.data('label', node.name);
          }
        }
      });

      // 2. Add/Update Links
      // Create a set of valid node IDs for fast lookup
      const validNodeIds = new Set(nodes.map(n => n.id));

      this.links().forEach(link => {
        // Safety check: Ensure both endpoints exist
        if (!validNodeIds.has(link.source) || !validNodeIds.has(link.target)) {
          console.warn(`Skipping orphan link ${link.id}: source ${link.source} or target ${link.target} not found.`);
          return;
        }

        // Build edge labels: "eth1\n10.0.1.1/24" or just "eth1" if no IP
        const srcIP = nodeIfaceIPs.get(link.source)?.get(link.source_int);
        const tgtIP = nodeIfaceIPs.get(link.target)?.get(link.target_int);
        const srcLabel = srcIP ? `${link.source_int}\n${srcIP}` : link.source_int;
        const tgtLabel = tgtIP ? `${link.target_int}\n${tgtIP}` : link.target_int;

        const isDisabled = !(link.enabled ?? true);
        const existingLink = this.cy.getElementById(link.id);
        if (existingLink.empty()) {
          const added = this.cy.add({
            group: 'edges',
            data: {
              id: link.id,
              source: link.source,
              target: link.target,
              source_int: srcLabel,
              target_int: tgtLabel
            }
          });
          if (isDisabled) added.addClass('link-disabled');
        } else {
          // Update labels if IPs changed
          if (existingLink.data('source_int') !== srcLabel) {
            existingLink.data('source_int', srcLabel);
          }
          if (existingLink.data('target_int') !== tgtLabel) {
            existingLink.data('target_int', tgtLabel);
          }
          // Sync disabled state
          if (isDisabled) {
            existingLink.addClass('link-disabled');
          } else {
            existingLink.removeClass('link-disabled');
          }
        }
      });

      // 3. Remove deleted Nodes
      const currentIds = new Set(nodes.map(n => n.id));
      this.cy.nodes().forEach(ele => {
        if (!currentIds.has(ele.id())) {
          this.cy.remove(ele);
        }
      });

      // 4. Remove deleted Links
      const currentLinkIds = new Set(this.links().map(l => l.id));
      this.cy.edges().forEach(ele => {
        if (!currentLinkIds.has(ele.id())) {
          this.cy.remove(ele);
        }
      });
    });

    if (!this.canvasReady) {
      this.canvasReady = true;
      if (this.cy.nodes().length > 0) {
        this.cy.fit(undefined, 50);
      }
      return;
    }

    // Highlight newly added nodes
    newNodeIds.forEach(id => {
      const el = this.cy.getElementById(id);
      el.style({ 'border-width': 2, 'border-color': '#60a5fa', 'border-opacity': 1 });
      el.animate({
        style: { 'border-opacity': 0 },
        duration: 1500,
        complete: () => el.removeStyle('border-width border-color border-opacity'),
      });
    });
  }
}
