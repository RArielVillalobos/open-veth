import { Component, input, output, inject, computed, effect, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Node, Link, RouteInfo } from '../../../../models/topology.model';
import { TopologyService } from '../../../../core/services/topology.service';

@Component({
  selector: 'app-properties-panel',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './properties-panel.component.html'
})
export class PropertiesPanelComponent {
  selectedNode = input<Node | null>(null);
  selectedLink = input<Link | null>(null);

  openTerminal = output<string>();
  openCapture = output<string>();
  deleteNode = output<string>();
  deleteLink = output<string>();
  close = output<void>();

  private service = inject(TopologyService);
  
  // Internal state
  routes = signal<RouteInfo[]>([]);
  loadingRoutes = signal(false);
  showHelp = signal(false);

  // Safe check for interfaces to use in template
  hasInterfaces = computed(() => {
    const n = this.selectedNode();
    return !!(n && n.interfaces && n.interfaces.length > 0);
  });

  constructor() {
    // Automatically load routes when a node is selected
    effect(() => {
      const node = this.selectedNode();
      // Reset state immediately
      this.routes.set([]);
      
      if (node && node.type !== 'switch') {
        // Use setTimeout to ensure we don't trigger change detection during render
        setTimeout(() => this.loadRoutes(node.id), 0);
      }
    });
  }

  loadRoutes(nodeId?: string) {
    const id = nodeId || this.selectedNode()?.id;
    if (!id) return;

    this.loadingRoutes.set(true);
    this.service.getNodeRoutes(id).subscribe({
      next: (data) => {
        // Sort: Connected (C) first, then Static (S), then by destination
        const sorted = (data || []).sort((a, b) => {
          const aIsKernel = a.protocol === 'kernel' ? 0 : 1;
          const bIsKernel = b.protocol === 'kernel' ? 0 : 1;
          if (aIsKernel !== bIsKernel) return aIsKernel - bIsKernel;
          return a.dst.localeCompare(b.dst);
        });
        this.routes.set(sorted);
        this.loadingRoutes.set(false);
      },
      error: () => {
        this.routes.set([]);
        this.loadingRoutes.set(false);
      }
    });
  }

  getIPv4(addrInfo: any[] | undefined): string {
    if (!addrInfo || addrInfo.length === 0) return 'No IP';
    const ipv4 = addrInfo.find((a: any) => !a.local.includes(':'));
    return ipv4 ? `${ipv4.local}/${ipv4.prefixlen}` : 'No IPv4';
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