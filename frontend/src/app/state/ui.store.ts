import { signalStore, withState, withMethods, patchState, withComputed } from '@ngrx/signals';
import { computed } from '@angular/core';

export interface UIState {
  selectedNodeId: string | null;
  selectedLinkId: string | null;
  isLabManagerOpen: boolean;
  hasUserInteracted: boolean;
}

const initialState: UIState = {
  selectedNodeId: null,
  selectedLinkId: null,
  isLabManagerOpen: false,
  hasUserInteracted: false
};

export const UIStore = signalStore(
  { providedIn: 'root' },
  withState(initialState),
  withComputed((store) => ({
    // Computed values for templates
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

    // --- Reset ---
    resetSession() {
      patchState(store, initialState);
    }
  }))
);
