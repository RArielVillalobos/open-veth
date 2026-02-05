import { Component, inject, signal, OnInit, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TopologyStore } from '../../state/topology.store';
import { TopologyService } from '../../core/services/topology.service';
import { ToastService } from '../../core/services/toast.service';
import { NodePaletteComponent } from './components/node-palette/node-palette.component';
import { PropertiesPanelComponent } from './components/properties-panel/properties-panel.component';
import { TopologyCanvasComponent } from '../../shared/components/topology-canvas/topology-canvas.component';
import { TerminalPanelComponent } from '../../shared/components/terminal-panel/terminal-panel.component';
import { PacketCaptureWindowComponent } from '../../shared/components/packet-capture-window/packet-capture-window.component';
import { ToastComponent } from '../../shared/components/toast/toast.component';
import { LabManagerComponent } from './components/lab-manager/lab-manager.component';
import { WelcomeModalComponent } from './components/welcome-modal/welcome-modal.component';
import { firstValueFrom } from 'rxjs';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [
    CommonModule,
    NodePaletteComponent,
    PropertiesPanelComponent,
    TopologyCanvasComponent,
    TerminalPanelComponent,
    PacketCaptureWindowComponent,
    ToastComponent,
    LabManagerComponent,
    WelcomeModalComponent
  ],
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.scss'
})
export class DashboardComponent implements OnInit {
  readonly store = inject(TopologyStore);
  private service = inject(TopologyService);
  private toast = inject(ToastService);

  // Estado para gestión de terminales (Tabs)
  activeTerminals = signal<string[]>([]);
  activeTab = signal<string | null>(null);

  // Estado para gestión de Labs
  showLabManager = signal(false);

  // Control para ocultar el modal de bienvenida tras la primera acción
  userHasInteracted = signal(false);

  // Selección de nodo y link
  selectedNodeId = signal<string | null>(null);
  selectedLinkId = signal<string | null>(null);

  // Mostrar modal solo cuando la carga terminó, la topología está vacía y el usuario no ha interactuado
  showWelcomeModal = computed(() =>
    !this.store.isLoading() && this.store.topology().nodes.length === 0 && !this.userHasInteracted()
  );

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
    this.selectedLinkId.set(null);

    if (id) {
      this.store.fetchNodeInterfaces(id);
    }
  }

  onLinkSelected(id: string | null) {
    this.selectedLinkId.set(id);
    this.selectedNodeId.set(null);
  }

  onAddNode(event: { type: 'router' | 'host' | 'switch'; name: string }) {
    this.userHasInteracted.set(true);
    this.store.addNode({
      id: 'node-' + Math.random().toString(36).substring(2, 7),
      name: event.name,
      type: event.type,
      x: 100,
      y: 100
    });
  }

  onDeleteNode(id: string) {
    this.store.removeNode(id);
    this.selectedNodeId.set(null);
  }

  onDeleteLink(id: string) {
    this.store.removeLink(id);
    this.selectedLinkId.set(null);
  }

  onRename() {
    this.userHasInteracted.set(true);
    this.showLabManager.set(true);
  }

  async onExport() {
    try {
      const blob = await firstValueFrom(this.service.exportTopology());
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `topology-${new Date().getTime()}.yaml`;
      a.click();
      window.URL.revokeObjectURL(url);
    } catch (err) {
      console.error('Failed to export topology', err);
    }
  }

  async onImport(event: any) {
    this.userHasInteracted.set(true);
    const file = event.target.files[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = async (e: any) => {
      const content = e.target.result;
      try {
        await firstValueFrom(this.service.importTopology(content));
        this.toast.success('Topology imported successfully');
        this.store.loadTopology();
      } catch (err: any) {
        console.error('Failed to import topology', err);
        this.toast.error('Import failed: ' + (err.error?.error || err.message));
      }
    };
    reader.readAsText(file);

    event.target.value = '';
  }

  openTerminal(nodeName: string) {
    if (!this.activeTerminals().includes(nodeName)) {
      this.activeTerminals.update(list => [...list, nodeName]);
    }
    this.activeTab.set(nodeName);
  }

  closeTerminal(nodeName: string) {
    this.activeTerminals.update(list => list.filter(n => n !== nodeName));

    if (this.activeTab() === nodeName) {
      const remaining = this.activeTerminals();
      this.activeTab.set(remaining.length > 0 ? remaining[remaining.length - 1] : null);
    }
  }

  setActiveTab(nodeName: string) {
    this.activeTab.set(nodeName);
  }

  onOpenCapture(iface: string) {
    const node = this.selectedNode();
    if (node) {
      this.store.openCapture(node.id, node.name, iface);
    }
  }
}
