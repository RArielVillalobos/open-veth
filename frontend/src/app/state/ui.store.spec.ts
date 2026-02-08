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
    expect(store.activeTerminals()).toEqual([]);
    expect(store.activeTabId()).toBeNull();
    expect(store.isTerminalPanelOpen()).toBe(false);
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

  describe('Terminal Tab Logic', () => {
    it('should add a terminal and set it as active tab', () => {
      store.openTerminal('node-1', 'Router-1');
      expect(store.activeTerminals()).toContainEqual({ nodeId: 'node-1', nodeName: 'Router-1' });
      expect(store.activeTabId()).toBe('node-1');
    });

    it('should not add duplicate terminals but update active tab', () => {
      store.openTerminal('node-1', 'Router-1');
      store.openTerminal('node-2', 'Host-1');
      store.setActiveTab('node-1');
      
      store.openTerminal('node-1', 'Router-1'); // Re-opening
      expect(store.activeTerminals().length).toBe(2);
      expect(store.activeTabId()).toBe('node-1');
    });

    it('should switch to last tab when closing current active tab', () => {
      store.openTerminal('node-a', 'Node-A');
      store.openTerminal('node-b', 'Node-B');
      store.openTerminal('node-c', 'Node-C'); // Active tab is node-c
      
      store.closeTerminal('node-c');
      
      expect(store.activeTerminals().find(t => t.nodeId === 'node-c')).toBeUndefined();
      expect(store.activeTabId()).toBe('node-b'); // Switched to last remaining
    });

    it('should set active tab to null when closing the last terminal', () => {
      store.openTerminal('node-a', 'Node-A');
      store.closeTerminal('node-a');
      
      expect(store.activeTerminals()).toEqual([]);
      expect(store.activeTabId()).toBeNull();
    });

    it('should keep current tab if closing a non-active terminal', () => {
      store.openTerminal('node-a', 'Node-A');
      store.openTerminal('node-b', 'Node-B');
      store.setActiveTab('node-b');
      
      store.closeTerminal('node-a');
      
      expect(store.activeTabId()).toBe('node-b');
    });
  });

  it('should mark user interaction', () => {
    expect(store.hasUserInteracted()).toBe(false);
    store.markInteraction();
    expect(store.hasUserInteracted()).toBe(true);
  });

  it('should reset session state', () => {
    store.selectNode('node-1');
    store.openTerminal('node-a', 'Node-A');
    
    store.resetSession();
    
    expect(store.selectedNodeId()).toBeNull();
    expect(store.activeTerminals()).toEqual([]);
  });
});
