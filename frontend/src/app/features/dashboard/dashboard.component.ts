import { Component, inject, signal, OnInit, OnDestroy, computed, effect, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TopologyStore } from '../../state/topology.store';
import { UIStore } from '../../state/ui.store';
import { TerminalStore } from '../../state/terminal.store';
import { CaptureStore } from '../../state/capture.store';
import { TopologyService } from '../../core/services/topology.service';
import { LayoutService } from '../../core/services/layout.service';
import { ToastService } from '../../core/services/toast.service';
import { NetworkEventsService } from '../../core/services/network-events.service';
import { NodePaletteComponent } from './components/node-palette/node-palette.component';
import { PropertiesPanelComponent } from './components/properties-panel/properties-panel.component';
import { CaptureContainerComponent } from './components/capture-container/capture-container.component';
import { TopologyCanvasComponent } from '../../shared/components/topology-canvas/topology-canvas.component';
import { TerminalPanelComponent } from '../../shared/components/terminal-panel/terminal-panel.component';
import { ToastComponent } from '../../shared/components/toast/toast.component';
import { LabManagerComponent } from './components/lab-manager/lab-manager.component';
import { WelcomeModalComponent } from './components/welcome-modal/welcome-modal.component';
import { firstValueFrom } from 'rxjs';
import { FileService } from '../../core/services/file.service';
import { TracerouteResponse } from '../../models/topology.model';

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
export class DashboardComponent implements OnInit, OnDestroy {
  readonly store = inject(TopologyStore);
  readonly ui = inject(UIStore);
  readonly terminalStore = inject(TerminalStore);
  readonly captureStore = inject(CaptureStore);
  private service = inject(TopologyService);
  private layoutService = inject(LayoutService);
  private toast = inject(ToastService);
  private fileService = inject(FileService);
  private networkEvents = inject(NetworkEventsService);

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
    const activeTabId = this.terminalStore.activeNodeId();
    return this.store.topology().nodes.find(n => n.id === activeTabId)?.name || null;
  });

  @ViewChild(TopologyCanvasComponent) canvasComponent!: TopologyCanvasComponent;
  @ViewChild(TerminalPanelComponent) terminalPanel!: TerminalPanelComponent;

  tracerouteResult = signal<TracerouteResponse | null>(null);
  readonly DEFAULT_TERMINAL_HEIGHT = 256;
  terminalHeight = signal(this.DEFAULT_TERMINAL_HEIGHT);

  private lastLabId: string | null = null;
  private readonly MIN_TERMINAL_HEIGHT = 120;
  private readonly MAX_TERMINAL_HEIGHT_RATIO = 0.8;
  private resizing = false;
  private resizeStartY = 0;
  private resizeStartHeight = 0;
  private boundMouseMove = this.onResizeMove.bind(this);
  private boundMouseUp = this.onResizeEnd.bind(this);

  constructor() {
    effect(() => {
      const currentId = this.store.topology().id;
      if (this.lastLabId !== null && this.lastLabId !== currentId) {
        this.ui.resetSession();
        this.terminalStore.closeAll();
        this.captureStore.closeAll();
      }
      this.lastLabId = currentId;
    });
  }

  ngOnInit() {
    this.store.loadTopology();
    // Connect to real-time network events
    this.networkEvents.connect();
  }

  ngOnDestroy() {
    this.networkEvents.disconnect();
    document.removeEventListener('mousemove', this.boundMouseMove);
    document.removeEventListener('mouseup', this.boundMouseUp);
  }

  onResizeStart(event: MouseEvent) {
    this.resizing = true;
    this.resizeStartY = event.clientY;
    this.resizeStartHeight = this.terminalHeight();
    document.addEventListener('mousemove', this.boundMouseMove);
    document.addEventListener('mouseup', this.boundMouseUp);
    document.body.style.cursor = 'ns-resize';
    document.body.style.userSelect = 'none';
    event.preventDefault();
  }

  private onResizeMove(event: MouseEvent) {
    if (!this.resizing) return;
    const delta = this.resizeStartY - event.clientY;
    const maxHeight = window.innerHeight * this.MAX_TERMINAL_HEIGHT_RATIO;
    const newHeight = Math.min(maxHeight, Math.max(this.MIN_TERMINAL_HEIGHT, this.resizeStartHeight + delta));
    this.terminalHeight.set(newHeight);
  }

  private onResizeEnd() {
    if (!this.resizing) return;
    this.resizing = false;
    document.removeEventListener('mousemove', this.boundMouseMove);
    document.removeEventListener('mouseup', this.boundMouseUp);
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    requestAnimationFrame(() => this.terminalPanel?.fitAll());
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

  onAddNode(event: { type: 'router' | 'host' | 'switch' | 'hub' | 'cloud' | 'linux'; name: string }) {
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
    this.terminalStore.closeTerminal(id);
  }

  onDeleteLink(id: string) {
    this.store.removeLink(id);
    this.ui.clearSelection();
  }

  onToggleLink(id: string) {
    this.store.toggleLink(id);
  }

  onRename() {
    this.ui.toggleLabManager(true);
  }

  async onExport() {
    try {
      const lab = this.store.topology();
      const blob = await firstValueFrom(this.service.exportTopology(lab.id));
      this.fileService.downloadBlob(blob, `${lab.name}.yaml`);
    } catch (err) {
      console.error('Failed to export topology', err);
      this.toast.error('Export failed');
    }
  }

  async onImport(event: any) {
    this.ui.markInteraction();
    const file = event.target.files[0];
    if (!file) return;

    try {
      const content = await this.fileService.readAsText(file);
      const labId = this.store.topology().id;
      const result = await firstValueFrom(this.service.importTopology(content, labId));
      if (result.errors?.length) {
        this.toast.error(`Import partial: ${result.errors.length} error(s) — ${result.errors[0]}`);
      } else {
        this.toast.success(`Topology imported: ${result.nodes} nodes, ${result.links} links`);
      }
      await this.store.loadTopology();
      setTimeout(() => this.canvasComponent?.fitToView());
    } catch (err: any) {
      console.error('Failed to import topology', err);
      this.toast.error('Import failed: ' + (err.error?.error || err.message));
    } finally {
      event.target.value = '';
    }
  }

  openTerminal(nodeId: string) {
    const node = this.store.topology().nodes.find(n => n.id === nodeId);
    if (node) {
      this.terminalStore.openTerminal(nodeId, node.name);
    }
  }

  onOpenCapture(iface: string) {
    const node = this.selectedNode();
    if (node) {
      this.captureStore.openCapture(node.id, node.name, iface);
    }
  }

  onCleanupLab() {
    const labName = this.store.topology().name;
    if (confirm(`Are you sure you want to delete all nodes and links in "${labName}"? This cannot be undone.`)) {
      this.store.cleanupCurrentLab();
      this.ui.resetSession();
      this.terminalStore.closeAll();
      this.captureStore.closeAll();
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

  async onTraceroute(event: { nodeId: string; destination: string }) {
    try {
      const result = await firstValueFrom(
        this.service.runTraceroute(event.nodeId, event.destination)
      );
      this.tracerouteResult.set(result);
      this.canvasComponent?.onTracerouteResult(result);
    } catch (err: any) {
      console.error('Traceroute failed', err);
      this.toast.error('Traceroute failed: ' + (err.error?.error || err.message));
      this.canvasComponent?.onTracerouteError();
    }
  }

  clearTraceroute() {
    this.tracerouteResult.set(null);
  }
}

