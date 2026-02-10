import { signalStore, withState, withMethods, patchState, withComputed } from '@ngrx/signals';
import { CaptureSession } from '../models/capture.model';
import { computed } from '@angular/core';

export interface CaptureState {
  sessions: CaptureSession[];
}

const initialState: CaptureState = {
  sessions: []
};

export const CaptureStore = signalStore(
  { providedIn: 'root' },
  withState(initialState),
  withComputed((store) => ({
    count: computed(() => store.sessions().length),
    hasActiveCaptures: computed(() => store.sessions().length > 0)
  })),
  withMethods((store) => ({
    
    /**
     * Opens a new packet capture window for a specific node and interface.
     */
    openCapture(nodeId: string, nodeName: string, interfaceName: string) {
      const exists = store.sessions().some(
        s => s.nodeId === nodeId && s.interfaceName === interfaceName
      );
      
      if (!exists) {
        patchState(store, (state) => ({
          sessions: [...state.sessions, { nodeId, nodeName, interfaceName }]
        }));
      }
    },

    /**
     * Closes a specific capture session.
     */
    closeCapture(nodeId: string, interfaceName: string) {
      patchState(store, (state) => ({
        sessions: state.sessions.filter(
          s => !(s.nodeId === nodeId && s.interfaceName === interfaceName)
        )
      }));
    },

    /**
     * Closes all active captures.
     */
    closeAll() {
      patchState(store, { sessions: [] });
    }
  }))
);
