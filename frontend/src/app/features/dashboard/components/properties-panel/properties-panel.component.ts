import { Component, input, output, inject, computed, effect, signal, untracked } from '@angular/core';
import { CommonModule } from '@angular/common';
import { firstValueFrom } from 'rxjs';
import { Node, Link, RouteInfo, MacEntry } from '../../../../models/topology.model';
import { TopologyService } from '../../../../core/services/topology.service';
import { ToastService } from '../../../../core/services/toast.service';
import { TopologyStore } from '../../../../state/topology.store';
import { ROUTE_HIGHLIGHT_COLOR, ROUTE_HIGHLIGHT_STATIC_COLOR } from '../../../../shared/components/topology-canvas/domain-colors';

@Component({
  selector: 'app-properties-panel',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './properties-panel.component.html',
  styleUrl: './properties-panel.component.scss'
})
export class PropertiesPanelComponent {
  selectedNode = input<Node | null>(null);
  selectedLink = input<Link | null>(null);

  openTerminal = output<string>();
  openCapture = output<string>();
  deleteNode = output<string>();
  deleteLink = output<string>();
  close = output<void>();
  traceroute = output<void>();
  toggleLink = output<string>();
  expanded = signal(false);
  readonly directGatewayExample = [
    { title: 'Internet Gateway', commands: 'ip addr add 10.20.0.1/24 dev eth0\nip link set eth0 up' },
    { title: 'Host', commands: 'ip addr add 10.20.0.2/24 dev eth0\nip link set eth0 up\nip route replace default via 10.20.0.1' },
  ];
  readonly gatewayExample = [
    { title: 'Internet Gateway', commands: 'ip addr add 10.0.2.2/30 dev eth0\nip link set eth0 up\nip route replace 10.0.1.0/24 via 10.0.2.1' },
    { title: 'Router', commands: 'ip addr add 10.0.1.1/24 dev eth0\nip addr add 10.0.2.1/30 dev eth1\nip link set eth0 up\nip link set eth1 up\nsysctl -w net.ipv4.ip_forward=1\nip route replace default via 10.0.2.2' },
    { title: 'Host', commands: 'ip addr add 10.0.1.10/24 dev eth0\nip link set eth0 up\nip route replace default via 10.0.1.1' },
  ];
  routeHover = output<{ subnet: string | null; gateway: string | null } | null>();

  readonly routeHighlightColor = ROUTE_HIGHLIGHT_COLOR;
  readonly routeHighlightStaticColor = ROUTE_HIGHLIGHT_STATIC_COLOR;

  private service = inject(TopologyService);
  private toast = inject(ToastService);
  private store = inject(TopologyStore);
  
  readonly powering = computed(() => this.store.togglingNodes().includes(this.selectedNode()?.id ?? ''));
  private readonly selectedIdentity = computed(() => { const n = this.selectedNode(); return n ? n.id + ':' + n.status + ':' + n.type : null; });
  private routeRequest = 0;
  private macRequest = 0;
  routeError = signal<string | null>(null);
  macError = signal<string | null>(null);
  nodeName(id: string | undefined) { return this.store.topology().nodes.find(node => node.id === id)?.name ?? id; }
  togglePower() { const node = this.selectedNode(); if (node) void this.store.toggleNodePower(node.id); }

  // Internal state
  routes = signal<RouteInfo[]>([]);
  loadingRoutes = signal(false);
  macTable = signal<MacEntry[]>([]);
  loadingMacTable = signal(false);
  showHelp = signal(false);
  uploading = signal(false);
  uploadResult = signal<{ success: boolean; path: string } | null>(null);
  showConnect = signal(false);
  showInterfaces = signal(true);
  readonly nodeStatus = computed(() => {
    const status = this.selectedNode()?.status;
    if (this.powering()) return status === 'running' ? 'Stopping…' : 'Starting…';
    return ({ running: 'Running', stopped: 'Stopped', pending: 'Starting…', error: 'Error' } as Record<string, string>)[status ?? ''] ?? 'Unknown';
  });


  // Whether the selected node is a bridge-based node (switch/hub) — used for interface display
  isBridgeNode = computed(() => {
    const t = this.selectedNode()?.type;
    return t === 'switch' || t === 'hub';
  });

  // Hub is unmanaged (no terminal, no routes, no file transfer)
  isHubNode = computed(() => this.selectedNode()?.type === 'hub');

  isMonitorNode = computed(() => this.selectedNode()?.type === 'monitor');

  isHAProxyNode = computed(() => this.selectedNode()?.type === 'haproxy');

  isZabbixNode = computed(() => this.selectedNode()?.type === 'zabbix');

  grafanaUrl = computed(() => {
    const port = this.selectedNode()?.service_ports?.['grafana'];
    return port ? `http://localhost:${port}` : null;
  });

  prometheusUrl = computed(() => {
    const port = this.selectedNode()?.service_ports?.['prometheus'];
    return port ? `http://localhost:${port}` : null;
  });

  haproxyStatsUrl = computed(() => {
    const port = this.selectedNode()?.service_ports?.['stats'];
    return port ? `http://localhost:${port}/stats` : null;
  });

  zabbixUrl = computed(() => {
    const port = this.selectedNode()?.service_ports?.['zabbix'];
    return port ? `http://localhost:${port}/zabbix` : null;
  });

  nodeServiceMessage = computed(() => {
    const id = this.selectedNode()?.id;
    return id ? (this.store.serviceMessages()[id] ?? '') : '';
  });

  // Safe check for interfaces to use in template
  hasInterfaces = computed(() => {
    const n = this.selectedNode();
    return !!(n && n.interfaces && n.interfaces.length > 0);
  });

  // Count active interfaces (excluding docker0 and br0 for bridge nodes)
  activeInterfaceCount = computed(() => {
    const n = this.selectedNode();
    if (!n || !n.interfaces) return 0;
    return n.interfaces.filter(i =>
      i.ifname !== 'lo' && i.ifname !== 'docker0' && !(this.isBridgeNode() && i.ifname === 'br0')
    ).length;
  });

  constructor() {
    // Automatically load routes when a node is selected
    effect(() => {
      this.selectedIdentity();
      const node = untracked(() => this.selectedNode());
      ++this.routeRequest; ++this.macRequest;
      this.loadingRoutes.set(false); this.loadingMacTable.set(false);
      this.routeError.set(null); this.macError.set(null);
      // Reset state immediately
      this.routes.set([]);
      this.macTable.set([]);
      this.uploadResult.set(null);

      if (node && node.status === 'running' && node.type !== 'switch' && node.type !== 'hub') {
        setTimeout(() => this.loadRoutes(node.id), 0);
      }
      if (node?.type === 'switch' && node.status === 'running') {
        setTimeout(() => this.loadMacTable(node.id), 0);
      }
    });
  }

  async loadRoutes(nodeId?: string) {
    const id = nodeId || this.selectedNode()?.id;
    if (!id || this.selectedNode()?.id !== id) return;

    const request = ++this.routeRequest;
    this.routeError.set(null);
    this.loadingRoutes.set(true);
    try {
      const data = await firstValueFrom(this.service.getNodeRoutes(id));
      // Sort: Connected (C) first, then Static (S), then by destination
      const sorted = (data || []).sort((a, b) => {
        const aIsKernel = a.protocol === 'kernel' ? 0 : 1;
        const bIsKernel = b.protocol === 'kernel' ? 0 : 1;
        if (aIsKernel !== bIsKernel) return aIsKernel - bIsKernel;
        return a.dst.localeCompare(b.dst);
      });
      if (request === this.routeRequest && this.selectedNode()?.id === id) this.routes.set(sorted);
    } catch (err: any) {
      if (request !== this.routeRequest || this.selectedNode()?.id !== id) return;
      this.routes.set([]); this.routeError.set('Routes could not be loaded. Try Refresh.');
      this.toast.error('Failed to load routes: ' + (err.error?.error || err.message || 'unknown error'));
    } finally {
      if (request === this.routeRequest) this.loadingRoutes.set(false);
    }
  }

  async loadMacTable(nodeId?: string) {
    const id = nodeId || this.selectedNode()?.id;
    if (!id || this.selectedNode()?.id !== id) return;

    const request = ++this.macRequest;
    this.macError.set(null);
    this.loadingMacTable.set(true);
    try {
      const data = await firstValueFrom(this.service.getNodeMacTable(id));
      if (request === this.macRequest && this.selectedNode()?.id === id) this.macTable.set(data || []);
    } catch {
      if (request !== this.macRequest || this.selectedNode()?.id !== id) return;
      this.macTable.set([]); this.macError.set('MAC table could not be loaded. Try Refresh.');
    } finally {
      if (request === this.macRequest) this.loadingMacTable.set(false);
    }
  }

  copyToClipboard(text: string) {
    if (!navigator.clipboard?.writeText) {
      this.toast.error('Clipboard is unavailable. Select the command and copy it manually.');
      return;
    }
    navigator.clipboard.writeText(text.trim()).then(() => this.toast.success('Copied to clipboard')).catch(() => this.toast.error('Could not copy. Select the command and copy it manually.'));
  }

  async onFileSelected(event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    const nodeId = this.selectedNode()?.id;
    if (!file || !nodeId) return;

    this.uploading.set(true);
    this.uploadResult.set(null);
    try {
      const result = await firstValueFrom(this.service.uploadFile(nodeId, file));
      this.uploadResult.set({ success: true, path: result.path });
      this.toast.success(`Uploaded ${result.filename} to ${result.path}`);
    } catch (err: any) {
      this.uploadResult.set({ success: false, path: '' });
      this.toast.error('Upload failed: ' + (err.error?.error || err.message || 'unknown error'));
    } finally {
      this.uploading.set(false);
      input.value = '';
    }
  }

  getIPv4(addrInfo: any[] | undefined): string {
    if (!addrInfo || addrInfo.length === 0) return 'No IP';
    const ipv4 = addrInfo.find((a: any) => !a.local.includes(':'));
    return ipv4 ? `${ipv4.local}/${ipv4.prefixlen}` : 'No IPv4';
  }

  onRouteMouseEnter(r: RouteInfo): void {
    if (r.dst === 'default') return;
    this.routeHover.emit({
      subnet: r.protocol === 'kernel' ? r.dst : null,
      gateway: r.gateway || null
    });
  }

  onRouteMouseLeave(): void {
    this.routeHover.emit(null);
  }

  getNetwork(addrInfo: any[] | undefined): string | null {
    if (!addrInfo || addrInfo.length === 0) return null;
    const ipv4 = addrInfo.find((a: any) => !a.local.includes(':'));
    if (!ipv4) return null;
    const parts = ipv4.local.split('.').map(Number);
    const prefix = ipv4.prefixlen;
    const mask = ~((1 << (32 - prefix)) - 1) >>> 0;
    const ip = ((parts[0] << 24) | (parts[1] << 16) | (parts[2] << 8) | parts[3]) >>> 0;
    const net = ip & mask;
    return `${(net >>> 24) & 255}.${(net >>> 16) & 255}.${(net >>> 8) & 255}.${net & 255}/${prefix}`;
  }
}
