import { signalStore, withState, withMethods, patchState, withHooks } from '@ngrx/signals';
import { Topology, Node, Link, Laboratory, DomainsResponse } from '../models/topology.model';
import { inject, effect } from '@angular/core';
import { firstValueFrom, forkJoin } from 'rxjs';
import { TopologyService } from '../core/services/topology.service';
import { ToastService } from '../core/services/toast.service';
import { UIStore } from './ui.store';

export interface TopologyState {
  topology: Topology;
  laboratories: Laboratory[]; // List of available labs
  currentLabId: string;
  isLoading: boolean;
  error: string | null;
  domains: DomainsResponse | null;
  activeDomainOverlay: 'broadcast' | 'collision' | null;
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
  error: null,
  domains: null,
  activeDomainOverlay: null
};

export const TopologyStore = signalStore(
  { providedIn: 'root' },
  withState(initialState),
  withMethods((store, service = inject(TopologyService), toast = inject(ToastService)) => {
    let positionDebounceTimer: any = null;

    return {
      // --- Initial Load ---
      async loadTopology() {
        patchState(store, { isLoading: true, error: null });
        try {
          const labId = store.currentLabId();
          
          // Load labs list and current topology in parallel
          const [initialNodes, initialLinks, labs] = await firstValueFrom(
            forkJoin([
              service.getNodes(true, labId),
              service.getLinks(labId),
              service.getLaboratories()
            ])
          );

          let activeNodes = initialNodes;
          let activeLinks = initialLinks;
          let currentLab = labs.find(l => l.id === labId);

          // No labs at all (fresh install / nuked DB) — create a default lab automatically
          if (labs.length === 0) {
            const newId = 'lab-' + crypto.randomUUID().substring(0, 8);
            const newLab = await firstValueFrom(service.createLaboratory({ id: newId, name: 'Default Laboratory' }));
            localStorage.setItem(LAB_ID_KEY, newLab.id);
            patchState(store, { currentLabId: newLab.id, laboratories: [newLab], isLoading: false,
              topology: { id: newLab.id, name: newLab.name, nodes: [], links: [] } });
            await this.loadDomains();
            return;
          }

          // Fallback: If lab from localStorage doesn't exist anymore, use the first available lab
          if (!currentLab) {
            const firstLab = labs[0];
            patchState(store, { currentLabId: firstLab.id });
            localStorage.setItem(LAB_ID_KEY, firstLab.id);
            const [fNodes, fLinks] = await firstValueFrom(
              forkJoin([service.getNodes(false, firstLab.id), service.getLinks(firstLab.id)])
            );
            currentLab = firstLab;
            activeNodes = fNodes;
            activeLinks = fLinks;
          }

          patchState(store, (state) => ({
            isLoading: false,
            laboratories: labs,
            topology: {
              ...state.topology,
              id: state.currentLabId,
              name: currentLab?.name || 'Unknown Lab',
              nodes: activeNodes || [],
              links: activeLinks || []
            }
          }));
          await this.loadDomains();
        } catch (err: any) {
          const msg = err.message || 'Error loading topology';
          patchState(store, { isLoading: false, error: msg });
          toast.error(msg);
        }
      },

      // --- Lab Management ---
      async switchLab(labId: string) {
        const labName = store.laboratories().find(l => l.id === labId)?.name ?? labId;
        localStorage.setItem(LAB_ID_KEY, labId);
        patchState(store, { isLoading: true, currentLabId: labId, domains: null, activeDomainOverlay: null });
        toast.info(`Activating "${labName}"...`);

        try {
          // Returns 202 immediately — actual completion arrives via lab:activated WebSocket event
          await firstValueFrom(service.activateLaboratory(labId));
        } catch (err: any) {
          patchState(store, { isLoading: false });
          if (err.status === 409) {
            toast.error('Another lab operation is in progress, please wait');
          } else {
            toast.error('Failed to start activation: ' + (err.error?.error || err.message));
          }
        }
      },

      async onLabActivated(labId: string) {
        await this.loadTopology(); // clears isLoading when done
        const labName = store.laboratories().find(l => l.id === labId)?.name ?? labId;
        toast.success(`"${labName}" is now active`);
      },

      onLabActivationFailed(labId: string, reason: string) {
        patchState(store, { isLoading: false });
        toast.error(`Lab activation failed: ${reason}`);
      },

      async createLaboratory(name: string) {
        // Validation: Prevent duplicate names
        const nameExists = store.laboratories().some(l => l.name.toLowerCase() === name.toLowerCase());
        if (nameExists) {
          toast.error(`A laboratory with the name "${name}" already exists.`);
          return;
        }

        const id = 'lab-' + crypto.randomUUID().substring(0, 8);
        patchState(store, { isLoading: true });
        try {
          await firstValueFrom(service.createLaboratory({ id, name }));
          await this.switchLab(id); // Auto-switch to new lab
          toast.success('Laboratory created');
        } catch (err: any) {
          patchState(store, { isLoading: false, error: err.message });
          toast.error('Failed to create lab');
        }
      },

      async deleteLaboratory(id: string) {
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
        patchState(store, { isLoading: true, error: null });
        try {
          // Inject current Lab ID
          const nodeWithLab = { ...node, lab_id: store.currentLabId() };
          const createdNode = await firstValueFrom(service.createNode(nodeWithLab));
          patchState(store, (state) => ({
            isLoading: false,
            topology: {
              ...state.topology,
              nodes: [...state.topology.nodes, createdNode]
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
        patchState(store, { isLoading: true, error: null });
        try {
          await firstValueFrom(service.deleteNode(id));
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
        } catch (err: any) {
          const msg = err.message || 'Error deleting node';
          patchState(store, { isLoading: false, error: msg });
          toast.error(msg);
        }
      },

      async toggleNodePower(id: string) {
        const node = store.topology().nodes.find(n => n.id === id);
        if (!node) return;
        const shouldStart = node.status === 'stopped';
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
        }
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
        patchState(store, (state) => ({
          topology: {
            ...state.topology,
            nodes: state.topology.nodes.map(n => 
              n.id === id ? { ...n, x, y } : n
            )
          }
        }));

        if (positionDebounceTimer) clearTimeout(positionDebounceTimer);
        
        positionDebounceTimer = setTimeout(async () => {
          try {
            await firstValueFrom(service.updateNodePosition(id, x, y));
          } catch (err) {
            console.error('Failed to persist node position', err);
          }
        }, 500);
      },

      // --- Links ---
      async addLink(link: Link) {
        patchState(store, { isLoading: true, error: null });
        try {
          // Inject current Lab ID
          const linkWithLab = { ...link, lab_id: store.currentLabId() };
          const createdLink = await firstValueFrom(service.createLink(linkWithLab));
          patchState(store, (state) => ({
            isLoading: false,
            topology: {
              ...state.topology,
              links: [...state.topology.links, createdLink]
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
        } catch (err: any) {
          const msg = err.message || 'Error deleting link';
          patchState(store, { isLoading: false, error: msg });
          toast.error(msg);
        }
      },

      // --- System ---
      async syncState() {
        patchState(store, { isLoading: true });
        try {
          // Sync only current lab nodes
          const nodes = await firstValueFrom(service.getNodes(true, store.currentLabId()));
          patchState(store, (state) => ({
            isLoading: false,
            topology: {
              ...state.topology,
              nodes: nodes
            }
          }));
          toast.success('Network state synced');
        } catch (err: any) {
          patchState(store, { isLoading: false, error: err.message });
          toast.error('Sync failed');
        }
      },

      async renameLaboratory(name: string) {
        const labId = store.currentLabId();
        try {
          await firstValueFrom(service.updateLaboratory(labId, name));
          patchState(store, (state) => ({
            topology: { ...state.topology, name },
            // Also update the list
            laboratories: state.laboratories.map(l => l.id === labId ? { ...l, name } : l)
          }));
          toast.success('Laboratory renamed');
        } catch (err: any) {
          toast.error('Failed to rename: ' + err.message);
        }
      },

      async cleanupCurrentLab() {
        const labId = store.currentLabId();
        const labName = store.topology().name;
        patchState(store, { isLoading: true });
        try {
          await firstValueFrom(service.cleanupLaboratory(labId));
          patchState(store, (state) => ({
            isLoading: false,
            topology: { ...state.topology, nodes: [], links: [] },
            domains: null
          }));
          toast.success(`Lab "${labName}" cleaned successfully`);
        } catch (err: any) {
          const msg = err.message || 'Cleanup error';
          patchState(store, { isLoading: false, error: msg });
          toast.error(msg);
        }
      },

      // --- Domains ---
      async loadDomains() {
        const labId = store.currentLabId();
        try {
          const domains = await firstValueFrom(service.getDomains(labId));
          patchState(store, { domains });
        } catch {
          patchState(store, { domains: null });
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
