import { TestBed } from '@angular/core/testing';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { NetworkEventsService } from './network-events.service';
import { TopologyStore } from '../../state/topology.store';

function createMockTopologyStore() {
  return {
    fetchNodeInterfaces: vi.fn(),
    loadTopology: vi.fn(),
  };
}

describe('NetworkEventsService', () => {
  let service: NetworkEventsService;
  let mockStore: ReturnType<typeof createMockTopologyStore>;

  beforeEach(() => {
    mockStore = createMockTopologyStore();

    TestBed.configureTestingModule({
      providers: [
        NetworkEventsService,
        { provide: TopologyStore, useValue: mockStore },
      ],
    });

    service = TestBed.inject(NetworkEventsService);
  });

  afterEach(() => {
    service.disconnect();
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });

  it('should handle interface:changed by fetching interfaces', () => {
    (service as any).handleEvent({
      type: 'interface:changed',
      nodeId: 'node-123',
      timestamp: Date.now(),
    });

    expect(mockStore.fetchNodeInterfaces).toHaveBeenCalledWith('node-123');
  });

  it('should ignore interface:changed without nodeId', () => {
    (service as any).handleEvent({
      type: 'interface:changed',
      nodeId: '',
      timestamp: Date.now(),
    });

    expect(mockStore.fetchNodeInterfaces).not.toHaveBeenCalled();
  });

  it('should handle node:created by loading topology', () => {
    (service as any).handleEvent({
      type: 'node:created',
      nodeId: 'node-123',
      timestamp: Date.now(),
    });

    expect(mockStore.loadTopology).toHaveBeenCalled();
  });

  it('should handle link:created by loading topology', () => {
    (service as any).handleEvent({
      type: 'link:created',
      nodeId: '',
      labId: 'lab-123',
      timestamp: Date.now(),
    });

    expect(mockStore.loadTopology).toHaveBeenCalled();
  });

  it('should not call store on unknown event type', () => {
    (service as any).handleEvent({
      type: 'unknown:event',
      nodeId: 'node-123',
      timestamp: Date.now(),
    });

    expect(mockStore.fetchNodeInterfaces).not.toHaveBeenCalled();
    expect(mockStore.loadTopology).not.toHaveBeenCalled();
  });

  it('should reset reconnect attempts on disconnect', () => {
    (service as any).reconnectAttempts = 3;

    service.disconnect();

    expect((service as any).reconnectAttempts).toBe(0);
  });

  it('should clear pending reconnect timeout on disconnect', () => {
    // Simulate a pending reconnect
    (service as any).reconnectTimeout = setTimeout(() => {}, 10000);

    service.disconnect();

    expect((service as any).reconnectTimeout).toBeNull();
  });

  it('should stop reconnecting after max attempts', () => {
    (service as any).reconnectAttempts = 5;

    (service as any).attemptReconnect();

    // Should not increment beyond max
    expect((service as any).reconnectAttempts).toBe(5);
    expect((service as any).reconnectTimeout).toBeNull();
  });
});
