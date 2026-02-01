import { signalStore, withState, withMethods, patchState } from '@ngrx/signals';
import { Topology, Node, Link } from '../models/topology.model';
import { inject } from '@angular/core';
import { firstValueFrom, forkJoin } from 'rxjs';
import { TopologyService } from '../core/services/topology.service';
import { ToastService } from '../core/services/toast.service';

export interface TopologyState {
  topology: Topology;
  isLoading: boolean;
  error: string | null;
}

const initialState: TopologyState = {
  topology: {
    id: 'lab-1',
    name: 'Default Laboratory',
    nodes: [],
    links: []
  },
  isLoading: false,
  error: null
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
          const [nodes, links, labs] = await firstValueFrom(
            forkJoin([service.getNodes(), service.getLinks(), service.getLaboratories()])
          );
          
          // Find current lab name (lab-1)
          const currentLab = labs.find(l => l.id === 'lab-1');
          
          patchState(store, (state) => ({
            isLoading: false,
            topology: {
              ...state.topology,
              name: currentLab?.name || state.topology.name,
              nodes: nodes || [],
              links: links || []
            }
          }));
        } catch (err: any) {
          const msg = err.message || 'Error loading topology';
          patchState(store, { isLoading: false, error: msg });
          toast.error(msg);
        }
      },

      // --- Nodes ---
      async addNode(node: Node) {
        patchState(store, { isLoading: true, error: null });
        try {
          const createdNode = await firstValueFrom(service.createNode(node));
          patchState(store, (state) => ({
            isLoading: false,
            topology: {
              ...state.topology,
              nodes: [...state.topology.nodes, createdNode]
            }
          }));
          toast.success(`Node ${node.name} created`);
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
        } catch (err: any) {
          const msg = err.message || 'Error deleting node';
          patchState(store, { isLoading: false, error: msg });
          toast.error(msg);
        }
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
          const createdLink = await firstValueFrom(service.createLink(link));
          patchState(store, (state) => ({
            isLoading: false,
            topology: {
              ...state.topology,
              links: [...state.topology.links, createdLink]
            }
          }));
          toast.success('Link created');
        } catch (err: any) {
          const msg = err.error?.error || err.message || 'Error creating link';
          patchState(store, { isLoading: false, error: msg });
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
          const nodes = await firstValueFrom(service.getNodes(true));
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
        const labId = 'lab-1';
        try {
          await firstValueFrom(service.updateLaboratory(labId, name));
          patchState(store, (state) => ({
            topology: { ...state.topology, name }
          }));
          toast.success('Laboratory renamed');
        } catch (err: any) {
          toast.error('Failed to rename: ' + err.message);
        }
      },

      async cleanup() {
        patchState(store, { isLoading: true });
        try {
          await firstValueFrom(service.cleanup());
          patchState(store, { 
            isLoading: false, 
            topology: { ...initialState.topology, nodes: [], links: [] } 
          });
          toast.success('Topology cleaned successfully');
        } catch (err: any) {
          const msg = err.message || 'Cleanup error';
          patchState(store, { isLoading: false, error: msg });
          toast.error(msg);
        }
      }
    };
  })
);
