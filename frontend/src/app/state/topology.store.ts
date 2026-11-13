import { signalStore, withState, withMethods, patchState, withHooks } from '@ngrx/signals';
import { Topology, Node, Link, Laboratory, DomainsResponse } from '../models/topology.model';
import { DestroyRef, inject, effect, untracked } from '@angular/core';
import { firstValueFrom, forkJoin } from 'rxjs';
import { TopologyService } from '../core/services/topology.service';
import { ToastService } from '../core/services/toast.service';
import { UIStore } from './ui.store';
import { PositionSaveService } from '../core/services/position-save.service';

export interface TopologyState {
  topology: Topology;
  laboratories: Laboratory[]; // List of available labs
  currentLabId: string;
  isLoading: boolean;
  pendingLabId: string | null;
  activationPhase: 'idle' | 'requesting' | 'activating' | 'failed' | 'uncertain';
  activationError: string | null;
  error: string | null;
  domains: DomainsResponse | null;
  activeDomainOverlay: 'broadcast' | 'collision' | null;
  togglingNodes: string[]; // Node IDs with a pending power toggle
  serviceMessages: Record<string, string>; // nodeId → current startup stage message
}

const LAB_ID_KEY = 'openveth_current_lab_id';

const getInitialLabId = () => {
  try {
    return (typeof localStorage !== 'undefined' && localStorage.getItem(LAB_ID_KEY)) || 'lab-1';
  } catch {
    return 'lab-1';
  }
};

const initialState: TopologyState = {
  topology: {
    id: 'lab-1',
    name: 'Default Laboratory',
    nodes: [],
    links: []
  },
  laboratories: [],
  currentLabId: getInitialLabId(),
  isLoading: false,
  pendingLabId: null, activationPhase: 'idle', activationError: null,
  error: null,
  domains: null,
  activeDomainOverlay: null,
  togglingNodes: [],
  serviceMessages: {}
};

export const TopologyStore = signalStore(
  { providedIn: 'root' },
  withState(initialState),
  withMethods((store, service = inject(TopologyService), toast = inject(ToastService)) => {
    const positions = inject(PositionSaveService);
    let loadVersion = 0;
    let activationVersion = 0;
    let activationOperationId: string | null = null;
    let activationTimer: ReturnType<typeof setTimeout> | undefined;
    let resolveActivation: ((success: boolean) => void) | null = null;
    let activationResult: Promise<boolean> = Promise.resolve(false);
    const activationBusy = () => ['requesting', 'activating', 'uncertain'].includes(store.activationPhase());
    inject(DestroyRef).onDestroy(() => { clearTimeout(activationTimer); resolveActivation?.(false); });
    const rememberLab = (id: string) => { try { localStorage.setItem(LAB_ID_KEY, id); } catch {} };
    const mergePositions = (nodes: Node[], labId: string) => nodes.map(node => {
      const pending = positions.get(node.id);
      return pending?.labId === labId ? { ...node, x: pending.x, y: pending.y } : node;
    });

    return {
      // --- Initial Load ---
      async loadTopology(requestedLabId = store.currentLabId(), confirmedActivation = false) {
        if (activationBusy() && !confirmedActivation) return false;
        const version = ++loadVersion;
        patchState(store, { isLoading: true, error: null });
        try {
          const [initialNodes, initialLinks, labs] = await firstValueFrom(forkJoin([
            service.getNodes(true, requestedLabId), service.getLinks(requestedLabId), service.getLaboratories()
          ]));
          if (version !== loadVersion) return false;
          let lab = labs.find(item => item.id === requestedLabId);
          let nodes = initialNodes ?? [], links = initialLinks ?? [];
          if (!labs.length) {
            lab = await firstValueFrom(service.createLaboratory({ id: 'lab-' + crypto.randomUUID().substring(0, 8), name: 'Default Laboratory' }));
            labs.push(lab); nodes = []; links = [];
          } else if (!lab) {
            lab = labs[0];
            [nodes, links] = await firstValueFrom(forkJoin([service.getNodes(true, lab.id), service.getLinks(lab.id)]));
          }
          if (version !== loadVersion) return false;
          rememberLab(lab.id);
          patchState(store, { isLoading: false, laboratories: labs, currentLabId: lab.id,
            topology: { id: lab.id, name: lab.name, nodes: mergePositions(nodes ?? [], lab.id), links: links ?? [] } });
          await this.loadDomains();
          return true;
        } catch (err: any) {
          if (version !== loadVersion) return false;
          const message = err.error?.error || err.message || 'Could not load laboratory';
          patchState(store, { isLoading: false, error: message }); toast.error(message);
          return false;
        }
      },

      async switchLab(labId: string) {
        if (activationBusy() || !(await positions.flush())) return false;
        ++activationVersion;
        activationOperationId = null;
        ++loadVersion;
        clearTimeout(activationTimer);
        resolveActivation?.(false);
        activationResult = new Promise<boolean>(resolve => { resolveActivation = resolve; });
        patchState(store, { pendingLabId: labId, activationPhase: 'requesting', activationError: null, error: null, isLoading: false });
        activationTimer = setTimeout(() => {
          patchState(store, { activationPhase: 'uncertain', activationError: 'Completion has not been confirmed. The laboratory may still be starting. Reconnect events or reload the application.' });
          resolveActivation?.(false); resolveActivation = null;
        }, 180_000);
        try {
          const accepted = await firstValueFrom(service.activateLaboratory(labId));
          activationOperationId = accepted?.operation_id ?? null;
          if (store.pendingLabId() === labId && store.activationPhase() === 'requesting') patchState(store, { activationPhase: 'activating' });
          return true;
        } catch (err: any) {
          clearTimeout(activationTimer);
          const message = err.status === 409 ? 'Another laboratory operation is in progress. Wait and try again.' : (err.error?.error || err.message || 'Could not start activation');
          patchState(store, { activationPhase: 'failed', activationError: message });
          resolveActivation?.(false); resolveActivation = null;
          toast.error(message); return false;
        }
      },

      waitForActivation() { return activationResult; },

      async reconcileAfterReconnect() {
        const labId = store.pendingLabId();
        if (!labId || !activationBusy()) return this.loadTopology();
        // Only a matching server operation can confirm completion; running nodes alone cannot.
        const operationId = activationOperationId;
        const version = activationVersion;
        if (!operationId) return false;
        try {
          const result = await firstValueFrom(service.getLabActivation(labId));
          if (version !== activationVersion || store.pendingLabId() !== labId || !activationBusy() || result.operation_id !== operationId) return false;
          if (result.status === 'succeeded') { await this.onLabActivated(labId); return true; }
          if (result.status === 'failed') await this.onLabActivationFailed(labId, result.reason || 'Laboratory activation failed');
        } catch {
          // Keep the pending operation and its timeout; never infer success from a failed query.
        }
        return false;
      },

      async onLabActivated(labId: string) {
        if (store.pendingLabId() && store.pendingLabId() !== labId) return;
        // A WebSocket event and reconnect query may confirm the same operation together.
        if (activationBusy() && store.isLoading()) return;
        const version = activationVersion;
        const completion = resolveActivation;
        clearTimeout(activationTimer);
        const loaded = await this.loadTopology(labId, true);
        if (version !== activationVersion) return;
        patchState(store, loaded ? { pendingLabId: null, activationPhase: 'idle', activationError: null, activeDomainOverlay: null, serviceMessages: {} }
          : { activationPhase: 'uncertain', activationError: 'Activation finished, but the laboratory view could not be loaded. Reload the application.' });
        if (loaded) toast.success('Laboratory is now active');
        completion?.(loaded);
        if (resolveActivation === completion) resolveActivation = null;
      },

      async onLabActivationFailed(labId: string, reason: string) {
        if (store.pendingLabId() && store.pendingLabId() !== labId) return;
        clearTimeout(activationTimer);
        patchState(store, { isLoading: false, activationPhase: 'failed', activationError: reason });
        resolveActivation?.(false); resolveActivation = null;
        toast.error('Laboratory activation failed: ' + reason);
        await this.loadTopology();
      },

      dismissActivationError() { patchState(store, { pendingLabId: null, activationPhase: 'idle', activationError: null }); },
      clearError() { patchState(store, { error: null }); },

      async createLaboratory(name: string) {
        if (activationBusy() || !(await positions.flush())) return null;
        name = name.trim();
        if (!name || store.laboratories().some(lab => lab.name.toLowerCase() === name.toLowerCase())) {
          toast.error('A laboratory with this name already exists, or the name is empty.'); return null;
        }
        patchState(store, { isLoading: true, error: null });
        try {
          const lab = await firstValueFrom(service.createLaboratory({ id: 'lab-' + crypto.randomUUID().substring(0, 8), name }));
          patchState(store, state => ({ laboratories: [...state.laboratories, lab], isLoading: false }));
          return await this.switchLab(lab.id) ? lab.id : null;
        } catch (err: any) {
          patchState(store, { isLoading: false, error: err.error?.error || err.message });
          toast.error('Could not create laboratory'); return null;
        }
      },

      async deleteLaboratory(id: string) {
        if (activationBusy()) return;
         if (id === store.currentLabId()) {
             toast.error("Cannot delete active laboratory");
             return;
         }
         try {
             await firstValueFrom(service.deleteLaboratory(id));
             patchState(store, (state) => ({
                 laboratories: state.laboratories.filter(l => l.id !== id)
             }));
             toast.success('Laboratory deleted');
         } catch(err: any) {
             toast.error('Failed to delete lab');
         }
      },

      // --- Nodes ---
      async addNode(node: Node) {
        if (activationBusy() || !(await positions.flush())) return;
        patchState(store, { isLoading: true, error: null });
        try {
          // Inject current Lab ID
          const nodeWithLab = { ...node, lab_id: store.currentLabId() };
          const createdNode = await firstValueFrom(service.createNode(nodeWithLab));
          patchState(store, (state) => ({
            isLoading: false,
            topology: {
              ...state.topology,
              nodes: [...state.topology.nodes.filter(n => n.id !== createdNode.id), createdNode]
            }
          }));
          toast.success(`Node ${node.name} created`);
          this.loadDomains();
        } catch (err: any) {
          const msg = err.error?.error || err.message || 'Error creating node';
          patchState(store, { isLoading: false, error: msg });
          toast.error(msg);
        }
      },

      async removeNode(id: string) {
        if (activationBusy() || !(await positions.flush())) return;
        patchState(store, { isLoading: true, error: null });
        try {
          await firstValueFrom(service.deleteNode(id));
          positions.discard(id);
          patchState(store, (state) => ({
            isLoading: false,
            topology: {
              ...state.topology,
              nodes: state.topology.nodes.filter(n => n.id !== id),
              links: state.topology.links.filter(l => l.source !== id && l.target !== id)
            }
          }));
          toast.success('Node deleted');
          this.loadDomains();
          return true;
        } catch (err: any) {
          const msg = err.message || 'Error deleting node';
          patchState(store, { isLoading: false, error: msg });
          toast.error(msg);
          return false;
        }
      },

      async toggleNodePower(id: string) {
        if (activationBusy() || !(await positions.flush())) return;
        const node = store.topology().nodes.find(n => n.id === id);
        if (!node) return;
        if (store.togglingNodes().includes(id)) return;
        const shouldStart = node.status === 'stopped';
        patchState(store, (state) => ({ togglingNodes: [...state.togglingNodes, id] }));
        try {
          const updated = await firstValueFrom(
            shouldStart ? service.startNode(id) : service.stopNode(id)
          );
          patchState(store, (state) => ({
            topology: {
              ...state.topology,
              nodes: state.topology.nodes.map(n => n.id === id ? { ...n, status: updated.status } : n)
            }
          }));
          toast.success(shouldStart ? `Node ${node.name} started` : `Node ${node.name} stopped`);
        } catch (err: any) {
          const msg = err.error?.error || err.message || 'Error toggling node power';
          toast.error(msg);
        } finally {
          patchState(store, (state) => ({ togglingNodes: state.togglingNodes.filter(n => n !== id) }));
        }
      },

      setServiceMessage(nodeId: string, message: string) {
        patchState(store, (state) => {
          if (!message) {
            const { [nodeId]: _, ...rest } = state.serviceMessages;
            return { serviceMessages: rest };
          }
          return { serviceMessages: { ...state.serviceMessages, [nodeId]: message } };
        });
      },

      updateNodeStatus(id: string, status: 'running' | 'stopped') {
        patchState(store, (state) => ({
          topology: {
            ...state.topology,
            nodes: state.topology.nodes.map(n => n.id === id ? { ...n, status } : n)
          }
        }));
      },

      async fetchNodeInterfaces(id: string) {
        const node = untracked(() => store.topology().nodes.find(n => n.id === id));
        if (node?.status !== 'running') return;
        try {
          const interfaces = await firstValueFrom(service.getNodeInterfaces(id));
          patchState(store, (state) => ({
            topology: {
              ...state.topology,
              nodes: state.topology.nodes.map(n => 
                n.id === id ? { ...n, interfaces } : n
              )
            }
          }));
        } catch (err: any) {
          console.error('Failed to fetch interfaces for node', id, err);
        }
      },

      async updateNodePosition(id: string, x: number, y: number) {
        if (activationBusy()) return;
        patchState(store, state => ({ topology: { ...state.topology,
          nodes: state.topology.nodes.map(node => node.id === id ? { ...node, x, y } : node) } }));
        positions.enqueue(id, { labId: store.currentLabId(), x, y });
      },

      // --- Links ---
      async addLink(link: Link) {
        if (activationBusy() || !(await positions.flush())) return;
        patchState(store, { isLoading: true, error: null });
        try {
          // Inject current Lab ID
          const linkWithLab = { ...link, lab_id: store.currentLabId() };
          const createdLink = await firstValueFrom(service.createLink(linkWithLab));
          patchState(store, (state) => ({
            isLoading: false,
            topology: {
              ...state.topology,
              links: [...state.topology.links.filter(l => l.id !== createdLink.id), createdLink]
            }
          }));
          toast.success('Link created');
          this.loadDomains();
        } catch (err: any) {
          const msg = err.error?.error || err.message || 'Error creating link';
          patchState(store, { isLoading: false, error: msg });
          toast.error(msg);
        }
      },

      async toggleLink(id: string) {
        if (activationBusy() || !(await positions.flush())) return;
        const link = store.topology().links.find(l => l.id === id);
        if (!link) return;
        const newEnabled = !(link.enabled ?? true);
        try {
          const updated = await firstValueFrom(service.toggleLink(id, newEnabled));
          patchState(store, (state) => ({
            topology: {
              ...state.topology,
              links: state.topology.links.map(l => l.id === id ? updated : l)
            }
          }));
          toast.success(newEnabled ? 'Link enabled' : 'Link disabled');
          this.loadDomains();
        } catch (err: any) {
          const msg = err.error?.error || err.message || 'Error toggling link';
          toast.error(msg);
        }
      },

      async removeLink(id: string) {
        if (activationBusy() || !(await positions.flush())) return;
        patchState(store, { isLoading: true, error: null });
        try {
          await firstValueFrom(service.deleteLink(id));
          patchState(store, (state) => ({
            isLoading: false,
            topology: {
              ...state.topology,
              links: state.topology.links.filter(l => l.id !== id)
            }
          }));
          toast.success('Link deleted');
          this.loadDomains();
          return true;
        } catch (err: any) {
          const msg = err.message || 'Error deleting link';
          patchState(store, { isLoading: false, error: msg });
          toast.error(msg);
          return false;
        }
      },

      // --- System ---
      async syncState() {
        if (activationBusy()) return;
        const labId = store.currentLabId();
        patchState(store, { isLoading: true, error: null });
        try {
          // Sync only current lab nodes
          const nodes = await firstValueFrom(service.getNodes(true, labId));
          if (store.currentLabId() !== labId) return;
          patchState(store, (state) => ({
            isLoading: false,
            topology: {
              ...state.topology,
              nodes: mergePositions(nodes, labId)
            }
          }));
          toast.success('Network state synced');
        } catch (err: any) {
          if (store.currentLabId() !== labId) return;
          patchState(store, { isLoading: false, error: err.message });
          toast.error('Sync failed');
        }
      },

      async renameLaboratory(name: string) {
        if (activationBusy()) return;
        const labId = store.currentLabId();
        try {
          await firstValueFrom(service.updateLaboratory(labId, name));
          patchState(store, (state) => ({
            topology: state.currentLabId === labId ? { ...state.topology, name } : state.topology,
            // Also update the list
            laboratories: state.laboratories.map(l => l.id === labId ? { ...l, name } : l)
          }));
          toast.success('Laboratory renamed');
        } catch (err: any) {
          toast.error('Failed to rename: ' + err.message);
        }
      },

      async cleanupCurrentLab() {
        if (activationBusy() || !(await positions.flush())) return;
        const labId = store.currentLabId();
        const labName = store.topology().name;
        patchState(store, { isLoading: true });
        try {
          await firstValueFrom(service.cleanupLaboratory(labId));
          if (store.currentLabId() !== labId) return false;
          patchState(store, (state) => ({
            isLoading: false,
            topology: { ...state.topology, nodes: [], links: [] },
            domains: null
          }));
          toast.success(`Lab "${labName}" cleaned successfully`);
          return true;
        } catch (err: any) {
          const msg = err.message || 'Cleanup error';
          patchState(store, { isLoading: false, error: msg });
          toast.error(msg);
          return false;
        }
      },

      // --- Domains ---
      async loadDomains() {
        const labId = store.currentLabId();
        try {
          const domains = await firstValueFrom(service.getDomains(labId));
          if (store.currentLabId() === labId) patchState(store, { domains });
        } catch {
          if (store.currentLabId() === labId) patchState(store, { domains: null });
        }
      },

      setDomainOverlay(mode: 'broadcast' | 'collision') {
        const current = store.activeDomainOverlay();
        patchState(store, { activeDomainOverlay: current === mode ? null : mode });
      }
    };
  }),
  withHooks({
    onInit(store, ui = inject(UIStore)) {
      // Reactive side-effect: Whenever a node is selected, fetch its interfaces automatically
      effect(() => {
        const selectedId = ui.selectedNodeId();
        if (selectedId) {
          store.fetchNodeInterfaces(selectedId);
        }
      });
    }
  })
);
