import { Component, inject, signal, OnInit, computed, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TopologyStore } from '../../state/topology.store';
import { UIStore } from '../../state/ui.store';
import { TopologyService } from '../../core/services/topology.service';
import { LayoutService } from '../../core/services/layout.service';
import { ToastService } from '../../core/services/toast.service';
import { NodePaletteComponent } from './components/node-palette/node-palette.component';
import { PropertiesPanelComponent } from './components/properties-panel/properties-panel.component';
import { CaptureContainerComponent } from './components/capture-container/capture-container.component';
import { TopologyCanvasComponent } from '../../shared/components/topology-canvas/topology-canvas.component';
import { TerminalPanelComponent } from '../../shared/components/terminal-panel/terminal-panel.component';
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
    CaptureContainerComponent,
    TopologyCanvasComponent,
    TerminalPanelComponent,
    ToastComponent,
    LabManagerComponent,
    WelcomeModalComponent
  ],
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.scss'
})
export class DashboardComponent implements OnInit {
  readonly store = inject(TopologyStore);
  readonly ui = inject(UIStore);
  private service = inject(TopologyService);
  private layoutService = inject(LayoutService);
  private toast = inject(ToastService);

  // Mostrar modal solo cuando la carga terminó, la topología está vacía y el usuario no ha interactuado
  showWelcomeModal = computed(() =>
    !this.store.isLoading() && this.store.topology().nodes.length === 0 && !this.ui.hasUserInteracted()
  );

  selectedNode = computed(() =>
    this.store.topology().nodes.find(n => n.id === this.ui.selectedNodeId()) || null
  );

  selectedLink = computed(() =>
    this.store.topology().links.find(l => l.id === this.ui.selectedLinkId()) || null
  );

  activeTerminalNodeName = computed(() => {
    const activeTabId = this.ui.activeTabId();
    return this.store.topology().nodes.find(n => n.id === activeTabId)?.name || null;
  });

  private lastLabId: string | null = null;

  constructor() {
    effect(() => {
      const currentId = this.store.topology().id;
      if (this.lastLabId !== null && this.lastLabId !== currentId) {
        this.ui.resetSession();
      }
      this.lastLabId = currentId;
    });
  }

  ngOnInit() {
    this.store.loadTopology();
  }

  onNodeSelected(id: string | null) {
    this.ui.selectNode(id);
  }

  onLinkSelected(id: string | null) {
    this.ui.selectLink(id);
  }

  onLinkRequest(event: { source: string; target: string }) {
    const { links } = this.store.topology();
    const source_int = this.layoutService.getNextInterface(event.source, links);
    const target_int = this.layoutService.getNextInterface(event.target, links);

    this.store.addLink({
      id: 'link-' + crypto.randomUUID().substring(0, 8),
      source: event.source,
      target: event.target,
      source_int,
      target_int
    });
  }

  onAddNode(event: { type: 'router' | 'host' | 'switch'; name: string }) {
    this.ui.markInteraction();
    const { nodes, links } = this.store.topology();
    const { x, y } = this.layoutService.getNextNodePosition(nodes, links);
    this.store.addNode({
      id: 'node-' + crypto.randomUUID().substring(0, 8),
      name: event.name,
      type: event.type,
      x,
      y
    });
  }

  onDeleteNode(id: string) {
    this.store.removeNode(id);
    this.ui.clearSelection();
  }

  onDeleteLink(id: string) {
    this.store.removeLink(id);
    this.ui.clearSelection();
  }

  onRename() {
    this.ui.toggleLabManager(true);
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
    this.ui.markInteraction();
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

  openTerminal(nodeId: string) {
    const node = this.store.topology().nodes.find(n => n.id === nodeId);
    if (node) {
      this.ui.openTerminal(nodeId, node.name);
    }
  }

  onOpenCapture(iface: string) {
    const node = this.selectedNode();
    if (node) {
      this.ui.openCapture(node.id, node.name, iface);
    }
  }

  onCleanupLab() {
    const labName = this.store.topology().name;
    if (confirm(`Are you sure you want to delete all nodes and links in "${labName}"? This cannot be undone.`)) {
      this.store.cleanupCurrentLab();
      this.ui.resetSession();
    }
  }

  async onSaveState() {
    const labId = this.store.topology().id;
    try {
      const result = await firstValueFrom(this.service.saveLabState(labId));
      this.toast.success(`State saved: ${result.ips_saved} IPs, ${result.routes_saved} routes persisted`);
    } catch (err: any) {
      console.error('Failed to save lab state', err);
      this.toast.error('Save failed: ' + (err.error?.error || err.message));
    }
  }
}

