import { TestBed } from '@angular/core/testing';
import { CaptureStore } from './capture.store';
import { describe, it, expect, beforeEach } from 'vitest';

describe('CaptureStore', () => {
  let store: InstanceType<typeof CaptureStore>;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [CaptureStore]
    });
    store = TestBed.inject(CaptureStore);
  });

  it('should start with empty sessions', () => {
    expect(store.sessions()).toEqual([]);
    expect(store.hasActiveCaptures()).toBe(false);
  });

  it('should add a capture session and prevent duplicates', () => {
    store.openCapture('n1', 'Node 1', 'eth1');
    store.openCapture('n1', 'Node 1', 'eth1'); // Duplicate
    store.openCapture('n1', 'Node 1', 'eth2'); // Different interface

    expect(store.sessions().length).toBe(2);
    expect(store.count()).toBe(2);
    expect(store.hasActiveCaptures()).toBe(true);
  });

  it('should close a specific capture', () => {
    store.openCapture('n1', 'Node 1', 'eth1');
    store.openCapture('n2', 'Node 2', 'eth1');
    
    store.closeCapture('n1', 'eth1');
    
    expect(store.sessions().length).toBe(1);
    expect(store.sessions()[0].nodeId).toBe('n2');
  });

  it('should close all captures', () => {
    store.openCapture('n1', 'Node 1', 'eth1');
    store.closeAll();
    expect(store.sessions()).toEqual([]);
  });
});
