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
  deleteNode = output<string>();
  deleteLink = output<string>();
  close = output<void>();

  private service = inject(TopologyService);
  
  // Internal state
  routes = signal<RouteInfo[]>([]);
  loadingRoutes = signal(false);

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
        // CRITICAL: Handle null data from backend to avoid 'reading length' errors
        this.routes.set(data || []);
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
}