import { TestBed } from '@angular/core/testing';
import { UIStore } from './ui.store';
import { describe, it, expect, beforeEach } from 'vitest';

describe('UIStore', () => {
  let store: InstanceType<typeof UIStore>;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [UIStore]
    });
    store = TestBed.inject(UIStore);
  });

  it('should have initial state', () => {
    expect(store.selectedNodeId()).toBeNull();
    expect(store.selectedLinkId()).toBeNull();
    expect(store.isLabManagerOpen()).toBe(false);
    expect(store.hasUserInteracted()).toBe(false);
  });

  describe('Selection Logic', () => {
    it('should select a node and clear link selection', () => {
      store.selectNode('node-1');
      expect(store.selectedNodeId()).toBe('node-1');
      expect(store.selectedLinkId()).toBeNull();
    });

    it('should select a link and clear node selection', () => {
      store.selectLink('link-1');
      expect(store.selectedLinkId()).toBe('link-1');
      expect(store.selectedNodeId()).toBeNull();
    });

    it('should clear all selections', () => {
      store.selectNode('node-1');
      store.clearSelection();
      expect(store.selectedNodeId()).toBeNull();
      expect(store.selectedLinkId()).toBeNull();
    });
  });

  it('should mark user interaction', () => {
    expect(store.hasUserInteracted()).toBe(false);
    store.markInteraction();
    expect(store.hasUserInteracted()).toBe(true);
  });

  it('should toggle lab manager', () => {
    store.toggleLabManager(true);
    expect(store.isLabManagerOpen()).toBe(true);
    expect(store.hasUserInteracted()).toBe(true);
  });

  it('should reset session state', () => {
    store.selectNode('node-1');
    store.resetSession();
    expect(store.selectedNodeId()).toBeNull();
  });
});
