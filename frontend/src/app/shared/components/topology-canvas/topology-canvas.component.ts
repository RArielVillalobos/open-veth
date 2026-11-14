import { Component, ElementRef, ViewChild, AfterViewInit, input, output, effect, OnDestroy, signal, computed, HostListener } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { DragDropModule } from '@angular/cdk/drag-drop';
import cytoscape from 'cytoscape';
import { Node as TopologyNode, Link, DomainsResponse, TracerouteResponse, NODE_TYPES } from '../../../models/topology.model';
import { DOMAIN_COLORS } from './domain-colors';
import { SIGNAL, PENDING, UP_600 } from './theme-colors';
import { parseNetworkAddress } from '../../utils/network-utils';
import { edgeMatchesSubnet, edgeMatchesGateway } from '../../utils/route-highlight.utils';
import { getCytoscapeStyles } from './cytoscape-styles';

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
  selectedNodeId = input<string | null>(null);
  selectedLinkId = input<string | null>(null);
  showIPs = signal(true);
  endpointLabels = signal<{ id: string; x: number; y: number; iface: string; ip: string; focused: boolean }[]>([]);
  private labelFrame: number | null = null;
  setLabelMode(value: string) { this.showIPs.set(value === 'ip'); }
  togglingNodes = input<string[]>([]);
  terminalNode = input<string | null>(null);
  domains = input<DomainsResponse | null>(null);
  activeDomainOverlay = input<'broadcast' | 'collision' | null>(null);
  domainLegend = computed(() => {
    const mode = this.activeDomainOverlay(), domains = this.domains();
    const list = mode === 'broadcast' ? domains?.broadcast_domains : mode === 'collision' ? domains?.collision_domains : [];
    return (list ?? []).map(domain => ({ name: (mode === 'broadcast' ? 'BD' : 'CD') + (domain.id + 1), color: DOMAIN_COLORS[domain.id % DOMAIN_COLORS.length], links: domain.link_ids.length }));
  });
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
  nodePowerToggle = output<string>();

  private cy!: cytoscape.Core;
  private hasActiveHighlight = false;
  private canvasReady = false;
  private pendingRevealNodeId: string | null = null;
  private revealFrame: number | null = null;
  private destroyed = false;
  private viewportSize = { width: 0, height: 0 };
  private resizingViewport = false;
  private resizeObserver!: ResizeObserver;
  private overlayCtx: CanvasRenderingContext2D | null = null;
  private pulseInterval: ReturnType<typeof setInterval> | null = null;
  private previousTogglingNodes: string[] = [];
  linkMode = signal(false);
  sourceNodeId: string | null = null;
  toggleLinkMode() { this.linkMode.update(value => !value); this.cancelLinking(); }
  stopLinking() { this.linkMode.set(false); this.cancelLinking(); }
  @HostListener('document:keydown.escape') onEscape() {
    this.stopLinking(); this.closeContextMenu();
    if (this.showTracerouteDialog) this.clearTraceroute();
  }

  
  // Context menu state
  contextMenu = {
    visible: false,
    x: 0,
    y: 0,
    elementId: '',
    elementName: '',
    elementType: '',
    linkEnabled: true,
    nodeStatus: 'running',
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
    effect(() => { this.showIPs(); this.selectedNodeId(); this.selectedLinkId(); this.scheduleLabels(); });
    effect(() => {
      this.nodes();
      this.links();
      if (this.cy) {
        this.updateCanvas();
      }
    });

    effect(() => {
      const toggling = this.togglingNodes();
      if (!this.cy) return;

      if (this.pulseInterval) {
        clearInterval(this.pulseInterval);
        this.pulseInterval = null;
        // Restore only the nodes that were pulsing — avoids clobbering node-stopped opacity
        this.previousTogglingNodes.forEach(id => {
          const el = this.cy.getElementById(id);
          if (!el.empty()) el.removeStyle('opacity border-width border-color border-style');
        });
      }

      this.previousTogglingNodes = [...toggling];
      if (toggling.length === 0) return;

      let bright = true;
      this.pulseInterval = setInterval(() => {
        if (!this.cy) return;
        toggling.forEach(id => {
          const el = this.cy.getElementById(id);
          if (el.empty()) return;
          el.style({
            'opacity': bright ? 0.9 : 0.3,
            'border-width': 2,
            'border-color': PENDING,
            'border-style': 'solid',
          });
        });
        bright = !bright;
      }, 500);
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
    this.prerasterizeIcons().then(bg => { if (!this.destroyed) this.initCytoscape(bg); });
  }

  private prerasterizeIcons(): Promise<Record<string, string>> {
    const SIZE = 256;
    return Promise.all(NODE_TYPES.map(name => new Promise<[string, string]>(resolve => {
      const img = new Image();
      img.onload = () => {
        const canvas = document.createElement('canvas');
        canvas.width = SIZE;
        canvas.height = SIZE;
        canvas.getContext('2d')!.drawImage(img, 0, 0, SIZE, SIZE);
        resolve([name, canvas.toDataURL('image/png')]);
      };
      img.onerror = () => resolve([name, `assets/icons/${name}.svg`]);
      img.src = `assets/icons/${name}.svg`;
    }))).then(entries => Object.fromEntries(entries));
  }

  ngOnDestroy() {
    if (this.revealFrame !== null) cancelAnimationFrame(this.revealFrame);
    if (this.labelFrame !== null) cancelAnimationFrame(this.labelFrame);
    this.destroyed = true;
    this.resizeObserver?.disconnect();
    if (this.pulseInterval) clearInterval(this.pulseInterval);
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

  onContextMenuAction(action: 'terminal' | 'delete' | 'properties' | 'traceroute' | 'toggle' | 'power') {
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
    } else if (action === 'power') {
      this.nodePowerToggle.emit(this.contextMenu.elementId);
    } else if (action === 'delete') {
      if (this.contextMenu.elementType === 'edge') {
        this.linkDelete.emit(this.contextMenu.elementId);
      } else {
        this.nodeDelete.emit(this.contextMenu.elementId);
      }
    }
    this.closeContextMenu();
  }

  get isContextNodeStopped(): boolean {
    const node = this.nodes().find(n => n.id === this.contextMenu.elementId);
    return node?.status === 'stopped';
  }

  get isContextNodeToggling(): boolean {
    return this.togglingNodes().includes(this.contextMenu.elementId);
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

  creationViewport() {
    if (!this.cy) return undefined;
    const extent = this.cy.extent(), zoom = this.cy.zoom();
    const padding = 75 / zoom;
    const insetX = Math.min(padding, extent.w / 4), insetY = Math.min(padding, extent.h / 4);
    return { x1: extent.x1 + insetX, y1: extent.y1 + insetY, x2: extent.x2 - insetX, y2: extent.y2 - insetY, zoom };
  }

  prepareNodeReveal(id: string) { this.pendingRevealNodeId = id; }
  cancelNodeReveal(id: string) { if (this.pendingRevealNodeId === id) this.pendingRevealNodeId = null; }

  revealCreatedNode(id: string) {
    if (this.destroyed || this.pendingRevealNodeId !== id || this.revealFrame !== null) return;
    this.revealFrame = requestAnimationFrame(() => {
      this.revealFrame = null;
      if (this.destroyed || !this.cy || this.pendingRevealNodeId !== id) return;
      const node = this.cy.getElementById(id);
      if (node.empty()) return;
      // The inspector may have opened since creation; use its final available canvas size.
      this.cy.resize();
      const box = node.renderedBoundingBox();
      const width = this.container.nativeElement.clientWidth, height = this.container.nativeElement.clientHeight;
      const left = Math.min(50, width / 4), right = width - left;
      const top = Math.min(80, height / 3), bottom = height - Math.min(40, height / 4);
      const dx = box.x1 < left ? left - box.x1 : box.x2 > right ? right - box.x2 : 0;
      const dy = box.y1 < top ? top - box.y1 : box.y2 > bottom ? bottom - box.y2 : 0;
      if (dx || dy) this.cy.panBy({ x: dx, y: dy });
      this.pendingRevealNodeId = null;
    });
  }

  fitToView() {
    if (this.cy) {
      this.cy.fit(undefined, 50);
    }
  }

  highlightRoute(info: { subnet: string | null; gateway: string | null } | null): void {
    if (!this.cy) return;
    if (this.hasActiveHighlight) {
      this.cy.edges().removeClass('route-highlight route-highlight-static');
      this.hasActiveHighlight = false;
    }
    if (!info) return;

    if (info.subnet) {
      this.cy.edges().filter(e =>
        edgeMatchesSubnet(
          (e.data('source_int') as string) || '',
          (e.data('target_int') as string) || '',
          info.subnet!
        )
      ).addClass('route-highlight');
      this.hasActiveHighlight = true;
    } else if (info.gateway) {
      this.cy.edges().filter(e =>
        edgeMatchesGateway(
          (e.data('source_int') as string) || '',
          (e.data('target_int') as string) || '',
          info.gateway!
        )
      ).addClass('route-highlight-static');
      this.hasActiveHighlight = true;
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

  private initCytoscape(bg: Record<string, string>) {
    this.cy = cytoscape({
      container: this.container.nativeElement,
      style: getCytoscapeStyles(bg),
      pixelRatio: 'auto',
      minZoom: 0.3,
      maxZoom: 1.5,
      wheelSensitivity: 0.3
    });

    // Expose for e2e testing
    (this.container.nativeElement as any).__cy = this.cy;

    // Recalculate canvas when container resizes
    this.viewportSize = { width: this.cy.width(), height: this.cy.height() };
    this.resizeObserver = new ResizeObserver(() => {
      const pan = this.cy.pan(), zoom = this.cy.zoom();
      const center = { x: (this.viewportSize.width / 2 - pan.x) / zoom, y: (this.viewportSize.height / 2 - pan.y) / zoom };
      this.resizingViewport = true;
      this.cy.resize();
      this.viewportSize = { width: this.cy.width(), height: this.cy.height() };
      this.cy.pan({ x: this.viewportSize.width / 2 - center.x * zoom, y: this.viewportSize.height / 2 - center.y * zoom });
      this.resizingViewport = false;
      this.redrawOverlayCanvas();
    });
    this.resizeObserver.observe(this.container.nativeElement);

    // --- Event Listeners ---

    // Tap on Node (Start/End Link)
    this.cy.on('tap', 'node', (evt) => {
      const clickedNode = evt.target;
      const clickedId = clickedNode.id();
      if (!this.linkMode()) { this.nodeSelected.emit(clickedId); return; }

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
      const nodeData = this.nodes().find(n => n.id === node.id());

      this.contextMenu = {
        visible: true,
        x: pos.x + 20,
        y: pos.y + 20,
        elementId: node.id(),
        elementName: node.data('name'),
        elementType: node.data('type'),
        linkEnabled: true,
        nodeStatus: nodeData?.status || 'running',
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
        nodeStatus: 'running',
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

    this.cy.on('zoom pan resize position add remove', () => this.scheduleLabels());

    this.cy.on('zoom pan', () => {
      if (!this.resizingViewport) this.closeContextMenu();
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
    this.updateCanvas();
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
    this.cy.edges().removeData('domain_label');
    this.clearDomainCanvas();
    this.scheduleLabels();
  }

  private applyDomainOverlay(domains: DomainsResponse, mode: 'broadcast' | 'collision'): void {
    const prefix = mode === 'broadcast' ? 'bd' : 'cd';
    const domainList = mode === 'broadcast' ? domains.broadcast_domains : domains.collision_domains;

    domainList.forEach(domain => {
      const colorIndex = domain.id % DOMAIN_COLORS.length;
      domain.link_ids.forEach(linkId => {
        const edge = this.cy.getElementById(linkId);
        if (!edge.empty()) {
          edge.addClass(`domain-${prefix}-${colorIndex}`);
          edge.data('domain_label', (mode === 'broadcast' ? 'BD' : 'CD') + (domain.id + 1));
        }
      });
    });
    this.scheduleLabels();
  }

  private redrawOverlayCanvas(): void {
    this.clearDomainCanvas();
    this.drawTracerouteHopBadges();
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
      ctx.fillStyle = UP_600;
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

  private scheduleLabels() {
    if (!this.cy || this.destroyed || this.labelFrame !== null) return;
    this.labelFrame = requestAnimationFrame(() => { this.labelFrame = null; this.updateEndpointLabels(); });
  }

  private updateEndpointLabels() {
    if (!this.cy || this.destroyed) return;
    const labels: { id: string; x: number; y: number; iface: string; ip: string; focused: boolean }[] = [];
    const occupied = this.cy.nodes().map(node => node.renderedBoundingBox());
    for (const edge of this.cy.edges()) {
      const text = edge.style('label');
      if (!text) continue;
      const p = edge.renderedMidpoint();
      const w = Math.max(42, text.length * 6.6 + 8);
      occupied.push({ x1: p.x - w / 2, x2: p.x + w / 2, y1: p.y - 10, y2: p.y + 18, w, h: 28 });
    }

    const width = this.container.nativeElement.clientWidth, height = this.container.nativeElement.clientHeight;
    const zoom = this.cy.zoom();
    const edges = this.cy.edges().toArray().sort((a, b) => Number(this.labelFocused(b)) - Number(this.labelFocused(a)));
    for (const edge of edges) {
      const focused = this.labelFocused(edge);
      if (zoom < 0.45 && !focused) continue;
      for (const end of ['source', 'target'] as const) {
        const node = end === 'source' ? edge.source() : edge.target();
        const other = end === 'source' ? edge.target() : edge.source();
        const start = node.renderedPosition(), finish = other.renderedPosition();
        const dx = finish.x - start.x, dy = finish.y - start.y;
        const length = Math.hypot(dx, dy) || 1, ux = dx / length, uy = dy / length;
        const [iface, address = ''] = String(edge.data(end + '_int')).split('\n');
        const ip = (focused || (this.showIPs() && zoom >= 0.75)) ? address : '';
        const w = Math.max(iface.length * 6.6, ip.length * 7.2) + 12, h = ip ? 34 : 20;
        const box = node.renderedBoundingBox();
        const distance = Math.min(length * 0.4, Math.max(box.w, box.h) / 2 + Math.abs(ux) * w / 2 + Math.abs(uy) * h / 2 + 10);
        let best: { x: number; y: number; score: number } | undefined;
        for (const along of [distance, length * 0.3, length * 0.45]) {
          for (const offset of [0, 24, -24, 44, -44, 68, -68]) {
            const x = start.x + ux * along - uy * offset - w / 2;
            const y = start.y + uy * along + ux * offset - h / 2;
            let score = Math.abs(offset) + Math.abs(along - distance) * 0.2;
            if (x < 4 || y < 48 || x + w > width - 4 || y + h > height - 4) score += 100000;
            for (const rect of occupied) {
              const overlapX = Math.min(x + w + 4, rect.x2) - Math.max(x - 4, rect.x1);
              const overlapY = Math.min(y + h + 4, rect.y2) - Math.max(y - 4, rect.y1);
              if (overlapX > 0 && overlapY > 0) score += overlapX * overlapY * 100;
            }
            if (!best || score < best.score) best = { x, y, score };
          }
        }
        if (!best || (best.score >= 100000 && !focused)) continue;
        labels.push({ id: edge.id() + '-' + end, x: best.x, y: best.y, iface, ip, focused });
        occupied.push({ x1: best.x, y1: best.y, x2: best.x + w, y2: best.y + h, w, h });
      }
    }
    this.endpointLabels.set(labels);
  }

  private labelFocused(edge: cytoscape.EdgeSingular) {
    return edge.id() === this.selectedLinkId() || edge.source().id() === this.selectedNodeId() || edge.target().id() === this.selectedNodeId();
  }

  private updateCanvas(): void {
    // Access signal value once
    const nodes = this.nodes();
    const newNodeIds: string[] = [];

    // Build a lookup: nodeId -> { ifaceName -> ip }
    const nodeIfaceIPs = new Map<string, Map<string, string>>();
    nodes.forEach(node => {
      const ifaceMap = new Map<string, string>();
      if (node.interfaces) {
        for (const iface of node.interfaces) {
          if (iface.ifname === 'docker0') continue;
          const ipv4 = iface.addr_info?.find(a => !a.local.includes(':'));
          if (ipv4) {
            ifaceMap.set(iface.ifname, `${ipv4.local}/${ipv4.prefixlen}`);
          }
        }
      }
      nodeIfaceIPs.set(node.id, ifaceMap);
    });

    this.cy.batch(() => {

      // 1. Add/Update Nodes
      nodes.forEach(node => {
        const existing = this.cy.getElementById(node.id);
        if (existing.empty()) {
          const added = this.cy.add({
            group: 'nodes',
            data: { id: node.id, label: node.name, name: node.name, type: node.type, status: node.status },
            position: { x: node.x ?? 100, y: node.y ?? 100 }
          });
          if (node.status === 'stopped') added.addClass('node-stopped');
          if (node.snapshot_image) added.addClass('has-snapshot');
          if (this.canvasReady) newNodeIds.push(node.id);
        } else {
          if (existing.data('label') !== node.name) {
            existing.data('label', node.name);
          }
          existing.data('status', node.status);
          if (node.status === 'stopped') {
            existing.addClass('node-stopped');
          } else {
            existing.removeClass('node-stopped');
          }
          if (node.snapshot_image) {
            existing.addClass('has-snapshot');
          } else {
            existing.removeClass('has-snapshot');
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

        // Build edge labels: "eth0\n10.0.1.1/24" or just "eth0" if no IP
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

      this.scheduleLabels();
    });

    if (this.pendingRevealNodeId) this.revealCreatedNode(this.pendingRevealNodeId);

    if (!this.canvasReady) {
      this.canvasReady = this.cy.nodes().length > 0;
      if (this.cy.nodes().length > 0 && !this.pendingRevealNodeId) {
        this.cy.fit(undefined, 50);
      }
      return;
    }

    newNodeIds.forEach(id => {
      const el = this.cy.getElementById(id);
      el.style({ 'border-width': 2, 'border-color': SIGNAL, 'border-opacity': 1 });
      el.animate({
        style: { 'border-opacity': 0 },
        duration: 1500,
        complete: () => el.removeStyle('border-width border-color border-opacity'),
      });
    });
  }
}
