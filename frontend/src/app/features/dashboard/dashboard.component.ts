import { Component, inject, signal, OnInit, computed, effect } from '@angular/core';
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

  private lastLabId: string | null = null;

  constructor() {
    effect(() => {
      const currentId = this.store.topology().id;
      if (this.lastLabId !== null && this.lastLabId !== currentId) {
        this.clearSessionState();
      }
      this.lastLabId = currentId;
    });
  }

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
    const { x, y } = this.nextNodePosition();
    this.store.addNode({
      id: 'node-' + Math.random().toString(36).substring(2, 7),
      name: event.name,
      type: event.type,
      x,
      y
    });
  }

  private nextNodePosition(): { x: number; y: number } {
    const { nodes, links } = this.store.topology();
    const cols = 5;
    const spacingX = 150;
    const spacingY = 120;
    const minNodeDist = 80;
    const minLinkDist = 50;

    for (let i = 0; i < 100; i++) {
      const x = 200 + (i % cols) * spacingX;
      const y = 150 + Math.floor(i / cols) * spacingY;

      if (nodes.some(n => Math.abs((n.x ?? 0) - x) < minNodeDist && Math.abs((n.y ?? 0) - y) < minNodeDist)) {
        continue;
      }

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

  private pointToSegmentDist(px: number, py: number, ax: number, ay: number, bx: number, by: number): number {
    const dx = bx - ax;
    const dy = by - ay;
    const lenSq = dx * dx + dy * dy;
    if (lenSq === 0) return Math.hypot(px - ax, py - ay);
    const t = Math.max(0, Math.min(1, ((px - ax) * dx + (py - ay) * dy) / lenSq));
    return Math.hypot(px - (ax + t * dx), py - (ay + t * dy));
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

  onCleanupLab() {
    const labName = this.store.topology().name;
    if (confirm(`Are you sure you want to delete all nodes and links in "${labName}"? This cannot be undone.`)) {
      this.store.cleanupCurrentLab();
      this.clearSessionState();
    }
  }

  async onSaveState() {
    const labId = this.store.topology().id;
    try {
      const result = await firstValueFrom(this.service.saveLabState(labId));
      this.toast.success(`State saved: ${result.configs_saved} IP configurations persisted`);
    } catch (err: any) {
      console.error('Failed to save lab state', err);
      this.toast.error('Save failed: ' + (err.error?.error || err.message));
    }
  }

  private clearSessionState() {
    this.activeTerminals.set([]);
    this.activeTab.set(null);
    this.selectedNodeId.set(null);
    this.selectedLinkId.set(null);
  }
}
