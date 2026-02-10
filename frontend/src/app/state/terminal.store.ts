import { signalStore, withState, withMethods, patchState, withComputed } from '@ngrx/signals';
import { TerminalSession } from '../models/terminal.model';
import { inject, computed } from '@angular/core';
import { ToastService } from '../core/services/toast.service';

export interface TerminalState {
  sessions: TerminalSession[];
  activeNodeId: string | null;
}

const initialState: TerminalState = {
  sessions: [],
  activeNodeId: null
};

export const TerminalStore = signalStore(
  { providedIn: 'root' },
  withState(initialState),
  withComputed((store) => ({
    isOpen: computed(() => store.sessions().length > 0),
  })),
  withMethods((store, toast = inject(ToastService)) => ({
    
    /**
     * Opens a terminal session for a node.
     * If the session already exists, it just activates it.
     */
    openTerminal(nodeId: string, nodeName: string) {
      const exists = store.sessions().some(s => s.nodeId === nodeId);
      
      if (exists) {
        patchState(store, { activeNodeId: nodeId });
        return;
      }

      const newSession: TerminalSession = {
        nodeId,
        nodeName,
        status: 'connecting',
        title: nodeName
      };

      patchState(store, (state) => ({
        sessions: [...state.sessions, newSession],
        activeNodeId: nodeId
      }));
    },

    /**
     * Closes a specific terminal session.
     * Automatically switches focus to the nearest available tab if the active one is closed.
     */
    closeTerminal(nodeId: string) {
      const currentSessions = store.sessions();
      const index = currentSessions.findIndex(s => s.nodeId === nodeId);
      
      if (index === -1) return;

      const newSessions = currentSessions.filter(s => s.nodeId !== nodeId);
      let newActiveId = store.activeNodeId();

      // If we are closing the currently active tab
      if (store.activeNodeId() === nodeId) {
        if (newSessions.length > 0) {
            // Try to select the one to the right, otherwise the one to the left
            const nextSession = newSessions[index] || newSessions[index - 1];
            newActiveId = nextSession ? nextSession.nodeId : null;
        } else {
            newActiveId = null;
        }
      }

      patchState(store, {
        sessions: newSessions,
        activeNodeId: newActiveId
      });
    },

    /**
     * Closes all open terminal sessions.
     */
    closeAll() {
      patchState(store, {
        sessions: [],
        activeNodeId: null
      });
    },

    /**
     * Sets the active terminal tab.
     */
    setActive(nodeId: string) {
      const exists = store.sessions().some(s => s.nodeId === nodeId);
      if (exists) {
        patchState(store, { activeNodeId: nodeId });
      }
    },

    /**
     * Updates the connection status of a terminal.
     */
    updateStatus(nodeId: string, status: 'connecting' | 'connected' | 'disconnected') {
      patchState(store, (state) => ({
        sessions: state.sessions.map(s => 
          s.nodeId === nodeId ? { ...s, status } : s
        )
      }));
    }
  }))
);
