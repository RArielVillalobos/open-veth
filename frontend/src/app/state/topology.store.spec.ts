import { TestBed } from '@angular/core/testing';
import { TopologyStore } from './topology.store';
import { UIStore } from './ui.store';
import { TopologyService } from '../core/services/topology.service';
import { ToastService } from '../core/services/toast.service';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { of, throwError } from 'rxjs';
import { createMockNode, createMockLink, createMockLaboratory, createMockInterface, createMockDomainsResponse } from '../../testing/mock-factories';

function createMockTopologyService() {
  return {
    getNodes: vi.fn().mockReturnValue(of([])),
    getLinks: vi.fn().mockReturnValue(of([])),
    getLaboratories: vi.fn().mockReturnValue(of([])),
    createNode: vi.fn(),
    deleteNode: vi.fn(),
    createLink: vi.fn(),
    deleteLink: vi.fn(),
    toggleLink: vi.fn(),
    createLaboratory: vi.fn(),
    activateLaboratory: vi.fn().mockReturnValue(of(undefined)),
    updateLaboratory: vi.fn(),
    deleteLaboratory: vi.fn(),
    cleanupLaboratory: vi.fn(),
    getNodeInterfaces: vi.fn().mockReturnValue(of([])),
    getNodeRoutes: vi.fn().mockReturnValue(of([])),
    updateNodePosition: vi.fn().mockReturnValue(of(undefined)),
    getDomains: vi.fn().mockReturnValue(of({ broadcast_domains: [], collision_domains: [] })),
  };
}

function createMockToastService() {
  return {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    show: vi.fn(),
    remove: vi.fn(),
    toasts: vi.fn(() => []),
  };
}

describe('TopologyStore', () => {
  let store: InstanceType<typeof TopologyStore>;
  let mockService: ReturnType<typeof createMockTopologyService>;
  let mockToast: ReturnType<typeof createMockToastService>;

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();

    mockService = createMockTopologyService();
    mockToast = createMockToastService();

    TestBed.configureTestingModule({
      providers: [
        TopologyStore,
        UIStore,
        { provide: TopologyService, useValue: mockService },
        { provide: ToastService, useValue: mockToast },
      ],
    });
    store = TestBed.inject(TopologyStore);
  });

  // --- Initial State ---

  it('should have correct initial state', () => {
    expect(store.topology().nodes).toEqual([]);
    expect(store.topology().links).toEqual([]);
    expect(store.isLoading()).toBe(false);
    expect(store.error()).toBeNull();
  });

  it('should persist and restore currentLabId via switchLab', async () => {
    mockService.activateLaboratory.mockReturnValue(of(undefined));
    mockService.getNodes.mockReturnValue(of([]));
    mockService.getLinks.mockReturnValue(of([]));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory({ id: 'lab-custom' })]));

    await store.switchLab('lab-custom');

    expect(store.currentLabId()).toBe('lab-custom');
    expect(localStorage.getItem('openveth_current_lab_id')).toBe('lab-custom');
  });

  // --- loadTopology ---

  it('loadTopology should fetch nodes, links, and labs', async () => {
    const nodes = [createMockNode()];
    const links = [createMockLink()];
    const labs = [createMockLaboratory()];

    mockService.getNodes.mockReturnValue(of(nodes));
    mockService.getLinks.mockReturnValue(of(links));
    mockService.getLaboratories.mockReturnValue(of(labs));

    await store.loadTopology();

    expect(store.topology().nodes).toEqual(nodes);
    expect(store.topology().links).toEqual(links);
    expect(store.laboratories()).toEqual(labs);
    expect(store.isLoading()).toBe(false);
  });

  it('loadTopology should fallback when saved lab does not exist', async () => {
    localStorage.setItem('openveth_current_lab_id', 'lab-deleted');

    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        TopologyStore,
        UIStore,
        { provide: TopologyService, useValue: mockService },
        { provide: ToastService, useValue: mockToast },
      ],
    });
    const freshStore = TestBed.inject(TopologyStore);

    const labs = [createMockLaboratory({ id: 'lab-existing', name: 'Existing' })];
    mockService.getNodes.mockReturnValue(of([]));
    mockService.getLinks.mockReturnValue(of([]));
    mockService.getLaboratories.mockReturnValue(of(labs));

    await freshStore.loadTopology();

    expect(freshStore.currentLabId()).toBe('lab-existing');
  });

  it('loadTopology should handle errors', async () => {
    mockService.getNodes.mockReturnValue(throwError(() => new Error('Network error')));
    mockService.getLinks.mockReturnValue(of([]));
    mockService.getLaboratories.mockReturnValue(of([]));

    await store.loadTopology();

    expect(store.isLoading()).toBe(false);
    expect(store.error()).toBe('Network error');
    expect(mockToast.error).toHaveBeenCalledWith('Network error');
  });

  // --- addNode ---

  it('addNode should create node with lab_id and update state', async () => {
    const node = createMockNode({ id: 'new-node', name: 'H1' });
    mockService.createNode.mockReturnValue(of(node));

    await store.addNode(node);

    expect(mockService.createNode).toHaveBeenCalledWith(
      expect.objectContaining({ lab_id: store.currentLabId() })
    );
    expect(store.topology().nodes).toContainEqual(node);
    expect(mockToast.success).toHaveBeenCalled();
  });

  it('addNode should handle errors', async () => {
    mockService.createNode.mockReturnValue(throwError(() => ({ message: 'Duplicate name' })));

    await store.addNode(createMockNode());

    expect(store.isLoading()).toBe(false);
    expect(mockToast.error).toHaveBeenCalled();
  });

  // --- removeNode ---

  it('removeNode should delete node and associated links', async () => {
    const node = createMockNode({ id: 'n1' });
    const link = createMockLink({ source: 'n1', target: 'n2' });

    // Pre-populate state via loadTopology
    mockService.getNodes.mockReturnValue(of([node, createMockNode({ id: 'n2' })]));
    mockService.getLinks.mockReturnValue(of([link]));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory()]));
    await store.loadTopology();

    mockService.deleteNode.mockReturnValue(of(undefined));
    await store.removeNode('n1');

    expect(store.topology().nodes.find(n => n.id === 'n1')).toBeUndefined();
    expect(store.topology().links.find(l => l.source === 'n1')).toBeUndefined();
  });

  it('removeNode should handle errors', async () => {
    mockService.deleteNode.mockReturnValue(throwError(() => new Error('Not found')));
    await store.removeNode('n1');
    expect(mockToast.error).toHaveBeenCalled();
  });

  // --- addLink / removeLink ---

  it('addLink should create link with lab_id', async () => {
    const link = createMockLink();
    mockService.createLink.mockReturnValue(of(link));

    await store.addLink(link);

    expect(mockService.createLink).toHaveBeenCalledWith(
      expect.objectContaining({ lab_id: store.currentLabId() })
    );
    expect(store.topology().links).toContainEqual(link);
  });

  it('addLink should handle errors', async () => {
    mockService.createLink.mockReturnValue(throwError(() => ({ message: 'Error' })));
    await store.addLink(createMockLink());
    expect(mockToast.error).toHaveBeenCalled();
  });

  it('removeLink should delete link from state', async () => {
    const link = createMockLink({ id: 'l1' });
    mockService.getNodes.mockReturnValue(of([]));
    mockService.getLinks.mockReturnValue(of([link]));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory()]));
    await store.loadTopology();

    mockService.deleteLink.mockReturnValue(of(undefined));
    await store.removeLink('l1');

    expect(store.topology().links.find(l => l.id === 'l1')).toBeUndefined();
  });

  // --- toggleLink ---

  it('toggleLink should disable an enabled link and update state', async () => {
    const link = createMockLink({ id: 'l1', enabled: true });
    mockService.getNodes.mockReturnValue(of([]));
    mockService.getLinks.mockReturnValue(of([link]));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory()]));
    await store.loadTopology();

    const disabled = { ...link, enabled: false };
    mockService.toggleLink.mockReturnValue(of(disabled));

    await store.toggleLink('l1');

    expect(mockService.toggleLink).toHaveBeenCalledWith('l1', false);
    expect(store.topology().links.find(l => l.id === 'l1')?.enabled).toBe(false);
    expect(mockToast.success).toHaveBeenCalledWith('Link disabled');
  });

  it('toggleLink should enable a disabled link and update state', async () => {
    const link = createMockLink({ id: 'l1', enabled: false });
    mockService.getNodes.mockReturnValue(of([]));
    mockService.getLinks.mockReturnValue(of([link]));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory()]));
    await store.loadTopology();

    const enabled = { ...link, enabled: true };
    mockService.toggleLink.mockReturnValue(of(enabled));

    await store.toggleLink('l1');

    expect(mockService.toggleLink).toHaveBeenCalledWith('l1', true);
    expect(store.topology().links.find(l => l.id === 'l1')?.enabled).toBe(true);
    expect(mockToast.success).toHaveBeenCalledWith('Link enabled');
  });

  it('toggleLink should do nothing if link does not exist', async () => {
    await store.toggleLink('nonexistent');
    expect(mockService.toggleLink).not.toHaveBeenCalled();
  });

  it('toggleLink should handle errors', async () => {
    const link = createMockLink({ id: 'l1', enabled: true });
    mockService.getNodes.mockReturnValue(of([]));
    mockService.getLinks.mockReturnValue(of([link]));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory()]));
    await store.loadTopology();

    mockService.toggleLink.mockReturnValue(throwError(() => ({ message: 'Network error' })));

    await store.toggleLink('l1');

    expect(mockToast.error).toHaveBeenCalled();
    // State must not change on error
    expect(store.topology().links.find(l => l.id === 'l1')?.enabled).toBe(true);
  });

  // --- Lab Management ---

  it('switchLab should activate lab and reload topology', async () => {
    mockService.activateLaboratory.mockReturnValue(of(undefined));
    mockService.getNodes.mockReturnValue(of([]));
    mockService.getLinks.mockReturnValue(of([]));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory({ id: 'lab-2' })]));

    await store.switchLab('lab-2');

    expect(mockService.activateLaboratory).toHaveBeenCalledWith('lab-2');
    expect(store.currentLabId()).toBe('lab-2');
    expect(localStorage.getItem('openveth_current_lab_id')).toBe('lab-2');
  });

  it('createLaboratory should prevent duplicate names', async () => {
    mockService.getNodes.mockReturnValue(of([]));
    mockService.getLinks.mockReturnValue(of([]));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory({ name: 'Existing' })]));
    await store.loadTopology();

    await store.createLaboratory('Existing');

    expect(mockService.createLaboratory).not.toHaveBeenCalled();
    expect(mockToast.error).toHaveBeenCalled();
  });

  it('deleteLaboratory should prevent deleting active lab', async () => {
    await store.deleteLaboratory(store.currentLabId());

    expect(mockService.deleteLaboratory).not.toHaveBeenCalled();
    expect(mockToast.error).toHaveBeenCalledWith('Cannot delete active laboratory');
  });

  it('deleteLaboratory should remove lab from state', async () => {
    const labs = [
      createMockLaboratory({ id: 'lab-1' }),
      createMockLaboratory({ id: 'lab-2', name: 'Second' }),
    ];
    mockService.getNodes.mockReturnValue(of([]));
    mockService.getLinks.mockReturnValue(of([]));
    mockService.getLaboratories.mockReturnValue(of(labs));
    await store.loadTopology();

    mockService.deleteLaboratory.mockReturnValue(of(undefined));
    await store.deleteLaboratory('lab-2');

    expect(store.laboratories().find(l => l.id === 'lab-2')).toBeUndefined();
    expect(mockToast.success).toHaveBeenCalled();
  });

  it('renameLaboratory should update topology name and labs list', async () => {
    const labs = [createMockLaboratory({ id: 'lab-1', name: 'Old Name' })];
    mockService.getNodes.mockReturnValue(of([]));
    mockService.getLinks.mockReturnValue(of([]));
    mockService.getLaboratories.mockReturnValue(of(labs));
    await store.loadTopology();

    mockService.updateLaboratory.mockReturnValue(of({ id: 'lab-1', name: 'New Name', status: 'active' }));
    await store.renameLaboratory('New Name');

    expect(store.topology().name).toBe('New Name');
    expect(store.laboratories()[0].name).toBe('New Name');
  });

  // --- System ---

  it('syncState should refresh nodes from backend', async () => {
    const freshNodes = [createMockNode({ id: 'synced', name: 'Synced' })];
    mockService.getNodes.mockReturnValue(of(freshNodes));

    await store.syncState();

    expect(store.topology().nodes).toEqual(freshNodes);
    expect(mockToast.success).toHaveBeenCalledWith('Network state synced');
  });

  it('cleanupCurrentLab should clear nodes and links', async () => {
    // Pre-populate
    mockService.getNodes.mockReturnValue(of([createMockNode()]));
    mockService.getLinks.mockReturnValue(of([createMockLink()]));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory()]));
    await store.loadTopology();

    mockService.cleanupLaboratory.mockReturnValue(of(undefined));
    await store.cleanupCurrentLab();

    expect(store.topology().nodes).toEqual([]);
    expect(store.topology().links).toEqual([]);
  });

  it('cleanupCurrentLab should handle errors', async () => {
    mockService.cleanupLaboratory.mockReturnValue(throwError(() => new Error('Cleanup failed')));
    await store.cleanupCurrentLab();

    expect(store.isLoading()).toBe(false);
    expect(mockToast.error).toHaveBeenCalled();
  });

  // --- Domains ---

  it('should have null domains in initial state', () => {
    expect(store.domains()).toBeNull();
    expect(store.activeDomainOverlay()).toBeNull();
  });

  it('loadDomains should fetch and store domains', async () => {
    const mockDomains = createMockDomainsResponse();
    mockService.getDomains.mockReturnValue(of(mockDomains));

    await store.loadDomains();

    expect(mockService.getDomains).toHaveBeenCalledWith(store.currentLabId());
    expect(store.domains()).toEqual(mockDomains);
  });

  it('loadDomains should handle errors silently', async () => {
    mockService.getDomains.mockReturnValue(throwError(() => new Error('fail')));

    await store.loadDomains();

    expect(store.domains()).toBeNull();
  });

  it('loadTopology should also load domains', async () => {
    const mockDomains = createMockDomainsResponse();
    mockService.getNodes.mockReturnValue(of([]));
    mockService.getLinks.mockReturnValue(of([]));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory()]));
    mockService.getDomains.mockReturnValue(of(mockDomains));

    await store.loadTopology();

    expect(mockService.getDomains).toHaveBeenCalled();
  });

  it('addNode should reload domains', async () => {
    mockService.createNode.mockReturnValue(of(createMockNode()));
    mockService.getDomains.mockReturnValue(of(createMockDomainsResponse()));

    await store.addNode(createMockNode());

    expect(mockService.getDomains).toHaveBeenCalled();
  });

  it('addLink should reload domains', async () => {
    mockService.createLink.mockReturnValue(of(createMockLink()));
    mockService.getDomains.mockReturnValue(of(createMockDomainsResponse()));

    await store.addLink(createMockLink());

    expect(mockService.getDomains).toHaveBeenCalled();
  });

  it('setDomainOverlay should toggle overlay mode', () => {
    store.setDomainOverlay('broadcast');
    expect(store.activeDomainOverlay()).toBe('broadcast');

    store.setDomainOverlay('broadcast');
    expect(store.activeDomainOverlay()).toBeNull();

    store.setDomainOverlay('collision');
    expect(store.activeDomainOverlay()).toBe('collision');
  });

  it('setDomainOverlay should switch mode directly', () => {
    store.setDomainOverlay('broadcast');
    expect(store.activeDomainOverlay()).toBe('broadcast');

    store.setDomainOverlay('collision');
    expect(store.activeDomainOverlay()).toBe('collision');
  });

  it('switchLab should clear domains and overlay', async () => {
    mockService.getDomains.mockReturnValue(of(createMockDomainsResponse()));
    mockService.getNodes.mockReturnValue(of([]));
    mockService.getLinks.mockReturnValue(of([]));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory()]));
    await store.loadTopology();

    store.setDomainOverlay('broadcast');
    expect(store.activeDomainOverlay()).toBe('broadcast');

    mockService.activateLaboratory.mockReturnValue(of(undefined));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory({ id: 'lab-2' })]));
    await store.switchLab('lab-2');

    expect(store.activeDomainOverlay()).toBeNull();
  });

  it('cleanupCurrentLab should clear domains', async () => {
    mockService.getDomains.mockReturnValue(of(createMockDomainsResponse()));
    mockService.getNodes.mockReturnValue(of([createMockNode()]));
    mockService.getLinks.mockReturnValue(of([]));
    mockService.getLaboratories.mockReturnValue(of([createMockLaboratory()]));
    await store.loadTopology();

    expect(store.domains()).not.toBeNull();

    mockService.cleanupLaboratory.mockReturnValue(of(undefined));
    await store.cleanupCurrentLab();

    expect(store.domains()).toBeNull();
  });
});
