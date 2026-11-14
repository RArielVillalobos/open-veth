import { Component, inject, signal, OnInit, OnDestroy, computed, effect, ViewChild, HostListener } from '@angular/core';
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
import { PositionSaveService } from '../../core/services/position-save.service';
import { DecisionDialogService } from '../../core/services/decision-dialog.service';
import { TracerouteResponse, NodeType } from '../../models/topology.model';

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
  readonly networkEvents = inject(NetworkEventsService);

  readonly positions = inject(PositionSaveService);
  private decisions = inject(DecisionDialogService);
  readonly isImporting = signal(false);
  readonly operationError = signal<string | null>(null);
  readonly saveStatus = signal<'idle' | 'saving' | 'saved' | 'partial' | 'error'>('idle');
  readonly activationBusy = computed(() => ['requesting', 'activating', 'uncertain'].includes(this.store.activationPhase()));
  readonly busy = computed(() => this.store.isLoading() || this.activationBusy() || this.isSavingState() || this.isImporting());
  readonly pendingLabName = computed(() => this.store.laboratories().find(lab => lab.id === this.store.pendingLabId())?.name || 'laboratory');
  readonly statusText = computed(() => {
    if (this.store.activationPhase() === 'uncertain') return 'Activation unconfirmed';
    if (this.activationBusy()) return 'Activating laboratory';
    if (this.store.activationError()) return 'Activation failed';
    if (this.isImporting()) return 'Importing topology';
    if (this.isSavingState()) return 'Saving laboratory state';
    if (this.positions.error()) return 'Positions not saved';
    if (this.operationError() || this.store.error()) return 'Action failed';
    if (this.store.isLoading()) return 'Updating laboratory';
    if (this.positions.saving()) return 'Saving positions';
    if (this.positions.count()) return 'Positions pending';
    if (this.saveStatus() === 'saved') return 'State saved';
    return 'Ready';
  });

  // Mostrar modal solo cuando la carga terminó, la topología está vacía y el usuario no ha interactuado
  showWelcomeModal = computed(() =>
    !this.busy() && !this.store.error() && this.store.topology().nodes.length === 0 && !this.ui.hasUserInteracted()
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
  isSavingState = signal(false);
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
        this.saveStatus.set('idle');
        this.operationError.set(null);
        this.clearTraceroute();
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
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    document.removeEventListener('mousemove', this.boundMouseMove);
    document.removeEventListener('mouseup', this.boundMouseUp);
  }

  @HostListener('window:beforeunload', ['$event'])
  protectPendingChanges(event: BeforeUnloadEvent) {
    if (this.positions.count() || this.isSavingState() || this.isImporting()) { event.preventDefault(); event.returnValue = ''; }
  }

  reloadApplication() { window.location.reload(); }

  resizeTerminalByKey(event: KeyboardEvent) {
    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return;
    event.preventDefault();
    this.terminalHeight.update(height => Math.max(this.MIN_TERMINAL_HEIGHT, Math.min(window.innerHeight * this.MAX_TERMINAL_HEIGHT_RATIO, height + (event.key === 'ArrowUp' ? 32 : -32))));
    requestAnimationFrame(() => this.terminalPanel?.fitAll());
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
    if (this.busy()) return;
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

  async onAddNode(event: { type: NodeType; name: string }) {
    if (this.busy()) return;
    this.ui.markInteraction();
    const { nodes, links } = this.store.topology();
    const { x, y } = this.layoutService.getNextNodePosition(nodes, links, this.canvasComponent?.creationViewport());
    const id = 'node-' + crypto.randomUUID().substring(0, 8);
    this.canvasComponent?.prepareNodeReveal(id);
    const labId = this.store.currentLabId();
    const created = await this.store.addNode({
      id,
      name: event.name,
      type: event.type,
      x,
      y
    });
    if (created && labId === this.store.currentLabId()) {
      this.ui.selectNode(created.id);
      this.canvasComponent?.revealCreatedNode(created.id);
    } else {
      this.canvasComponent?.cancelNodeReveal(id);
    }
  }

  async onDeleteNode(id: string) {
    if (this.busy()) return;
    if (await this.store.removeNode(id)) { this.ui.clearSelection(); this.terminalStore.closeTerminal(id); }
  }

  async onDeleteLink(id: string) {
    if (!this.busy() && await this.store.removeLink(id)) this.ui.clearSelection();
  }

  onToggleLink(id: string) {
    this.store.toggleLink(id);
  }

  onRename() {
    this.ui.toggleLabManager(true);
  }

  async onExport() {
    try {
      if (!(await this.positions.flush())) return;
      const lab = this.store.topology();
      const blob = await firstValueFrom(this.service.exportTopology(lab.id));
      this.fileService.downloadBlob(blob, `${lab.name}.yaml`);
    } catch (err) {
      console.error('Failed to export topology', err);
      this.toast.error('Export failed');
    }
  }

  async onImport(event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file || this.busy()) return;
    this.ui.markInteraction();
    const current = this.store.topology();
    const baseName = file.name.replace(/\.ya?ml$/i, '').trim() || 'Imported laboratory';
    let newName = baseName, suffix = 2;
    while (this.store.laboratories().some(lab => lab.name.toLowerCase() === newName.toLowerCase())) newName = baseName + ' (' + suffix++ + ')';
    const choice = await this.decisions.choose({
      title: 'Import topology',
      message: 'File: ' + file.name + '\nCreate a new laboratory named "' + newName + '", or replace "' + current.name + '". Replacing deletes its existing nodes, links, saved network settings, and snapshots.',
      actions: [{ value: 'new', label: 'Import into new laboratory' }, { value: 'replace', label: 'Replace current laboratory', destructive: true }],
    });
    if (!choice || this.busy()) return;
    if (current.id !== this.store.currentLabId()) {
      this.toast.error('The selected laboratory changed. Choose the import destination again.');
      return;
    }
    this.isImporting.set(true); this.operationError.set(null);
    try {
      if (!(await this.positions.flush())) return;
      const content = await this.fileService.readAsText(file);
      let labId = current.id;
      if (choice === 'new') {
        const createdId = await this.store.createLaboratory(newName);
        if (!createdId || !(await this.store.waitForActivation())) return;
        labId = createdId;
      }
      const result = await firstValueFrom(this.service.importTopology(content, labId));
      if (result.errors?.length) this.operationError.set('Import incomplete: ' + result.errors.join('; '));
      else this.toast.success('Topology imported: ' + result.nodes + ' nodes, ' + result.links + ' links');
      await this.store.loadTopology();
      this.terminalStore.closeAll(); this.captureStore.closeAll(); this.ui.clearSelection();
      setTimeout(() => this.canvasComponent?.fitToView());
      this.saveStatus.set('idle');
    } catch (err: any) {
      this.operationError.set('Import failed: ' + (err.error?.error || err.message));
    } finally { this.isImporting.set(false); }
  }

  openTerminal(nodeId: string) {
    const node = this.store.topology().nodes.find(n => n.id === nodeId);
    if (node && node.status !== 'stopped' && !this.busy()) {
      this.terminalStore.openTerminal(nodeId, node.name);
    }
  }

  onOpenCapture(iface: string) {
    const node = this.selectedNode();
    if (node) {
      this.captureStore.openCapture(node.id, node.name, iface);
    }
  }

  async onCleanupLab() {
    if (this.busy()) return;
    const lab = this.store.topology();
    const confirmed = await this.decisions.confirm('Reset laboratory?', 'Delete all ' + lab.nodes.length + ' nodes, their links, saved settings, and snapshots in "' + lab.name + '"? The laboratory itself will remain. This cannot be undone.', 'Reset laboratory');
    if (confirmed && lab.id !== this.store.currentLabId()) {
      this.toast.error('The selected laboratory changed. Review the reset action again.');
      return;
    }
    if (confirmed && !this.busy() && await this.store.cleanupCurrentLab()) {
      this.ui.resetSession(); this.terminalStore.closeAll(); this.captureStore.closeAll(); this.saveStatus.set('idle');
    }
  }

  async onSaveState() {
    if (this.busy() || !(await this.positions.flush())) return;
    this.isSavingState.set(true); this.saveStatus.set('saving'); this.operationError.set(null);
    const labId = this.store.topology().id;
    try {
      const result = await firstValueFrom(this.service.saveLabState(labId));
      if (labId !== this.store.currentLabId()) return;
      if (result.snapshots_saved < this.store.topology().nodes.filter(node => node.status === 'running').length) {
        this.saveStatus.set('partial'); this.operationError.set('Network settings were saved, but some node snapshots failed. Save again to retry.');
      } else {
        this.saveStatus.set('saved');
        this.toast.success(`State saved: ${result.ips_saved} IPs, ${result.routes_saved} routes, ${result.snapshots_saved} snapshots`);
      }
    } catch (err: any) {
      console.error('Failed to save lab state', err);
      this.saveStatus.set('error'); this.operationError.set('Save failed: ' + (err.error?.error || err.message));
    } finally {
      this.isSavingState.set(false);
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

