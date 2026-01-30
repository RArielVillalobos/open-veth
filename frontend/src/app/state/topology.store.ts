import { signalStore, withState, withMethods, patchState } from '@ngrx/signals';
import { Topology, Node, Link } from '../models/topology.model';
import { inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { TopologyService } from '../core/services/topology.service';

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
  withMethods((store, service = inject(TopologyService)) => ({
    
    addNode(node: Node) {
      patchState(store, (state: TopologyState) => ({
        topology: {
          ...state.topology,
          nodes: [...state.topology.nodes, node]
        }
      }));
    },

    addLink(link: Link) {
      patchState(store, (state: TopologyState) => ({
        topology: {
          ...state.topology,
          links: [...state.topology.links, link]
        }
      }));
    },

    updateNodePosition(id: string, x: number, y: number) {
      patchState(store, (state: TopologyState) => ({
        topology: {
          ...state.topology,
          nodes: state.topology.nodes.map(n => 
            n.id === id ? { ...n, x, y } : n
          )
        }
      }));
    },

    async deploy() {
      patchState(store, { isLoading: true, error: null });
      try {
        await firstValueFrom(service.deploy(store.topology()));
        patchState(store, { isLoading: false });
      } catch (err: any) {
        patchState(store, { isLoading: false, error: err.message });
      }
    },

    async cleanup() {
      patchState(store, { isLoading: true });
      try {
        await firstValueFrom(service.cleanup());
        patchState(store, { isLoading: false, topology: initialState.topology });
      } catch (err: any) {
        patchState(store, { isLoading: false, error: err.message });
      }
    }
  }))
);