import { Component, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TopologyStore } from '../../state/topology.store';
import { TopologyToolbarComponent } from '../../shared/components/topology-toolbar/topology-toolbar.component';
import { TopologyCanvasComponent } from '../../shared/components/topology-canvas/topology-canvas.component';
import { TerminalPanelComponent } from '../../shared/components/terminal-panel/terminal-panel.component';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, TopologyToolbarComponent, TopologyCanvasComponent, TerminalPanelComponent],
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.scss'
})
export class DashboardComponent {
  readonly store = inject(TopologyStore);
  
  // Estado para gestión de terminales (Tabs)
  activeTerminals = signal<string[]>([]);
  activeTab = signal<string | null>(null);

  onAddNode(type: 'router' | 'host' | 'switch') {
    this.store.addNode({
      id: 'node-' + Math.random().toString(36).substr(2, 5),
      name: type.toUpperCase() + '-' + (this.store.topology().nodes.length + 1),
      type: type,
      image: type === 'router' ? 'openveth/router:latest' : 'openveth/host:latest'
    });
  }

  openTerminal(nodeName: string) {
    // Si no está abierta, añadirla
    if (!this.activeTerminals().includes(nodeName)) {
      this.activeTerminals.update(list => [...list, nodeName]);
    }
    // Enfocarla
    this.activeTab.set(nodeName);
  }

  closeTerminal(nodeName: string) {
    this.activeTerminals.update(list => list.filter(n => n !== nodeName));
    
    // Si cerramos la activa, cambiar foco
    if (this.activeTab() === nodeName) {
      const remaining = this.activeTerminals();
      this.activeTab.set(remaining.length > 0 ? remaining[remaining.length - 1] : null);
    }
  }

  setActiveTab(nodeName: string) {
    this.activeTab.set(nodeName);
  }
}
