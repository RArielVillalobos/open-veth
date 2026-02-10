import { TestBed } from '@angular/core/testing';
import { TerminalStore } from './terminal.store';
import { ToastService } from '../core/services/toast.service';
import { describe, it, expect, beforeEach, vi } from 'vitest';

describe('TerminalStore', () => {
  let store: InstanceType<typeof TerminalStore>;
  let toastService: any;

  beforeEach(() => {
    toastService = {
      error: vi.fn(),
      success: vi.fn(),
      info: vi.fn()
    };

    TestBed.configureTestingModule({
      providers: [
        TerminalStore,
        { provide: ToastService, useValue: toastService }
      ]
    });

    store = TestBed.inject(TerminalStore);
  });

  it('should start with empty sessions', () => {
    expect(store.sessions()).toEqual([]);
    expect(store.activeNodeId()).toBeNull();
  });

  describe('openTerminal', () => {
    it('should add new session and activate it', () => {
      store.openTerminal('n1', 'Node 1');
      
      expect(store.sessions().length).toBe(1);
      expect(store.sessions()[0]).toEqual({
        nodeId: 'n1',
        nodeName: 'Node 1',
        status: 'connecting',
        title: 'Node 1'
      });
      expect(store.activeNodeId()).toBe('n1');
    });

    it('should not add duplicate session but activate existing one', () => {
      store.openTerminal('n1', 'Node 1');
      store.openTerminal('n2', 'Node 2');
      
      // Re-open n1
      store.openTerminal('n1', 'Node 1');

      expect(store.sessions().length).toBe(2);
      expect(store.activeNodeId()).toBe('n1');
    });
  });

  describe('closeTerminal', () => {
    it('should remove session', () => {
      store.openTerminal('n1', 'Node 1');
      store.closeTerminal('n1');
      
      expect(store.sessions().length).toBe(0);
      expect(store.activeNodeId()).toBeNull();
    });

    it('should switch to next available tab when active is closed (right side)', () => {
      store.openTerminal('n1', 'Node 1');
      store.openTerminal('n2', 'Node 2'); // Active
      store.openTerminal('n3', 'Node 3');

      store.setActive('n2');
      store.closeTerminal('n2');

      // Should default to n3 (the one that took its place at index 1)
      expect(store.sessions().map(s => s.nodeId)).toEqual(['n1', 'n3']);
      expect(store.activeNodeId()).toBe('n3');
    });

    it('should switch to previous tab when active is closed (last one)', () => {
      store.openTerminal('n1', 'Node 1');
      store.openTerminal('n2', 'Node 2'); // Active

      store.setActive('n2');
      store.closeTerminal('n2');

      // Should fallback to n1
      expect(store.sessions().map(s => s.nodeId)).toEqual(['n1']);
      expect(store.activeNodeId()).toBe('n1');
    });

    it('should ignore closing non-existent session', () => {
        store.openTerminal('n1', 'Node 1');
        store.closeTerminal('n99');
        expect(store.sessions().length).toBe(1);
    });
  });

  describe('updateStatus', () => {
    it('should update status correctly', () => {
      store.openTerminal('n1', 'Node 1');
      store.updateStatus('n1', 'connected');
      
      expect(store.sessions()[0].status).toBe('connected');
    });
  });

  describe('closeAll', () => {
    it('should clear everything', () => {
        store.openTerminal('n1', 'Node 1');
        store.openTerminal('n2', 'Node 2');
        
        store.closeAll();
        
        expect(store.sessions()).toEqual([]);
        expect(store.activeNodeId()).toBeNull();
    });
  });
});
