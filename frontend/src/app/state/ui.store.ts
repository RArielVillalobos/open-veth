import { signalStore, withState, withMethods, patchState, withComputed } from '@ngrx/signals';
import { computed } from '@angular/core';

export interface UIState {
  selectedNodeId: string | null;
  selectedLinkId: string | null;
  activeTerminals: { nodeId: string, nodeName: string }[]; 
  activeTabId: string | null;  // Current visible terminal tab ID
  isLabManagerOpen: boolean;
  hasUserInteracted: boolean;
  activeCaptures: { nodeId: string, nodeName: string, interfaceName: string }[];
}

const initialState: UIState = {
  selectedNodeId: null,
  selectedLinkId: null,
  activeTerminals: [],
  activeTabId: null,
  isLabManagerOpen: false,
  hasUserInteracted: false,
  activeCaptures: []
};

export const UIStore = signalStore(
  { providedIn: 'root' },
  withState(initialState),
  withComputed((store) => ({
    // Computed values for templates
    isTerminalPanelOpen: computed(() => store.activeTerminals().length > 0),
    hasSelection: computed(() => !!store.selectedNodeId() || !!store.selectedLinkId())
  })),
  withMethods((store) => ({
    // --- Selection Logic ---
    selectNode(id: string | null) {
      patchState(store, { 
        selectedNodeId: id, 
        selectedLinkId: null // Auto-deselect link
      });
    },

    selectLink(id: string | null) {
      patchState(store, { 
        selectedLinkId: id, 
        selectedNodeId: null // Auto-deselect node
      });
    },

    clearSelection() {
      patchState(store, { selectedNodeId: null, selectedLinkId: null });
    },

    // --- Interaction State ---
    markInteraction() {
      if (!store.hasUserInteracted()) {
        patchState(store, { hasUserInteracted: true });
      }
    },

    toggleLabManager(isOpen: boolean) {
      patchState(store, { isLabManagerOpen: isOpen });
      if (isOpen) this.markInteraction();
    },

    // --- Terminal & Tab Logic ---
    openTerminal(nodeId: string, nodeName: string) {
      const currentList = store.activeTerminals();
      const updates: Partial<UIState> = { activeTabId: nodeId };

      if (!currentList.find(t => t.nodeId === nodeId)) {
        updates.activeTerminals = [...currentList, { nodeId, nodeName }];
      }
      
      patchState(store, updates);
    },

    closeTerminal(nodeId: string) {
      const currentList = store.activeTerminals();
      const newList = currentList.filter(t => t.nodeId !== nodeId);
      let newActiveTabId = store.activeTabId();

      // Smart Tab Switching: If closing active tab, switch to the last one
      if (store.activeTabId() === nodeId) {
        newActiveTabId = newList.length > 0 ? newList[newList.length - 1].nodeId : null;
      }

      patchState(store, {
        activeTerminals: newList,
        activeTabId: newActiveTabId
      });
    },

    setActiveTab(nodeId: string) {
      patchState(store, { activeTabId: nodeId });
    },

    closeAllTerminals() {
      patchState(store, { activeTerminals: [], activeTabId: null });
    },

    // --- Capture/Sniffing Logic ---
    openCapture(nodeId: string, nodeName: string, interfaceName: string) {
      const exists = store.activeCaptures().find(c => c.nodeId === nodeId && c.interfaceName === interfaceName);
      if (!exists) {
        patchState(store, (state) => ({ 
          activeCaptures: [...state.activeCaptures, { nodeId, nodeName, interfaceName }] 
        }));
      }
    },

    closeCapture(nodeId: string, interfaceName: string) {
        patchState(store, (state) => ({ 
          activeCaptures: state.activeCaptures.filter(c => !(c.nodeId === nodeId && c.interfaceName === interfaceName)) 
        }));
    },

    // --- Reset ---
    resetSession() {
      patchState(store, initialState);
    }
  }))
);
