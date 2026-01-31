import { Component, inject, signal, OnInit, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TopologyStore } from '../../state/topology.store';
import { NodePaletteComponent } from './components/node-palette/node-palette.component';
import { PropertiesPanelComponent } from './components/properties-panel/properties-panel.component';
import { TopologyCanvasComponent } from '../../shared/components/topology-canvas/topology-canvas.component';
import { TerminalPanelComponent } from '../../shared/components/terminal-panel/terminal-panel.component';
import { ToastComponent } from '../../shared/components/toast/toast.component';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, NodePaletteComponent, PropertiesPanelComponent, TopologyCanvasComponent, TerminalPanelComponent, ToastComponent],
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.scss'
})
export class DashboardComponent implements OnInit {
  readonly store = inject(TopologyStore);
  
  // Estado para gestión de terminales (Tabs)
  activeTerminals = signal<string[]>([]);
  activeTab = signal<string | null>(null);
  
  // Selección de nodo y link
  selectedNodeId = signal<string | null>(null);
  selectedLinkId = signal<string | null>(null);
  
  selectedNode = computed(() => 
    this.store.topology().nodes.find(n => n.id === this.selectedNodeId()) || null
  );

  selectedLink = computed(() => 
    this.store.topology().links.find(l => l.id === this.selectedLinkId()) || null
  );

  ngOnInit() {
    this.store.loadTopology();
  }

  onNodeSelected(id: string | null) {
    this.selectedNodeId.set(id);
    this.selectedLinkId.set(null); // Mutuamente exclusivo
    
    if (id) {
      this.store.fetchNodeInterfaces(id);
    }
  }

  onLinkSelected(id: string | null) {
    this.selectedLinkId.set(id);
    this.selectedNodeId.set(null); // Mutuamente exclusivo
  }

  onAddNode(type: 'router' | 'host' | 'switch') {
    this.store.addNode({
      id: 'node-' + Math.random().toString(36).substr(2, 5),
      name: type.toUpperCase() + '-' + (this.store.topology().nodes.length + 1),
      type: type,
      image: type === 'router' ? 'openveth/router:latest' : 'openveth/host:latest',
      x: 100, // Posición inicial por defecto
      y: 100
    });
  }

  onDeleteNode(id: string) {
    this.store.removeNode(id);
    this.selectedNodeId.set(null); // Deseleccionar
  }

  onDeleteLink(id: string) {
    this.store.removeLink(id);
    this.selectedLinkId.set(null);
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
