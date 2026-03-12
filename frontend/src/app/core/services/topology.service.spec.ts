import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { TopologyService } from './topology.service';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createMockNode, createMockLink, createMockLaboratory, createMockInterface, createMockRoute, createMockDomainsResponse, createMockTracerouteResponse } from '../../../testing/mock-factories';
import { environment } from '../../../environments/environment';

describe('TopologyService', () => {
  let service: TopologyService;
  let httpMock: HttpTestingController;
  const API = environment.apiUrl;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(TopologyService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
  });

  // --- Nodes ---

  it('getNodes() should GET /nodes with live=false by default', () => {
    const mockNodes = [createMockNode()];
    service.getNodes().subscribe(nodes => expect(nodes).toEqual(mockNodes));
    const req = httpMock.expectOne(`${API}/nodes?live=false`);
    expect(req.request.method).toBe('GET');
    req.flush(mockNodes);
  });

  it('getNodes(true, labId) should include lab_id param', () => {
    service.getNodes(true, 'lab-1').subscribe();
    const req = httpMock.expectOne(`${API}/nodes?live=true&lab_id=lab-1`);
    expect(req.request.method).toBe('GET');
    req.flush([]);
  });

  it('createNode() should POST to /nodes', () => {
    const node = createMockNode();
    service.createNode(node).subscribe(res => expect(res.id).toBe(node.id));
    const req = httpMock.expectOne(`${API}/nodes`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual(node);
    req.flush(node);
  });

  it('deleteNode() should DELETE /nodes/:id', () => {
    service.deleteNode('node-1').subscribe();
    const req = httpMock.expectOne(`${API}/nodes/node-1`);
    expect(req.request.method).toBe('DELETE');
    req.flush(null);
  });

  it('getNodeInterfaces() should GET /nodes/:id/interfaces', () => {
    const ifaces = [createMockInterface()];
    service.getNodeInterfaces('node-1').subscribe(res => expect(res).toEqual(ifaces));
    const req = httpMock.expectOne(`${API}/nodes/node-1/interfaces`);
    expect(req.request.method).toBe('GET');
    req.flush(ifaces);
  });

  it('getNodeRoutes() should GET /nodes/:id/routes', () => {
    const routes = [createMockRoute()];
    service.getNodeRoutes('node-1').subscribe(res => expect(res).toEqual(routes));
    const req = httpMock.expectOne(`${API}/nodes/node-1/routes`);
    expect(req.request.method).toBe('GET');
    req.flush(routes);
  });

  it('updateNodePosition() should PATCH /nodes/:id/position', () => {
    service.updateNodePosition('node-1', 100, 200).subscribe();
    const req = httpMock.expectOne(`${API}/nodes/node-1/position`);
    expect(req.request.method).toBe('PATCH');
    expect(req.request.body).toEqual({ x: 100, y: 200 });
    req.flush(null);
  });

  // --- Links ---

  it('getLinks() should GET /links without params', () => {
    service.getLinks().subscribe();
    const req = httpMock.expectOne(`${API}/links`);
    expect(req.request.method).toBe('GET');
    req.flush([]);
  });

  it('getLinks(labId) should include lab_id param', () => {
    service.getLinks('lab-1').subscribe();
    const req = httpMock.expectOne(`${API}/links?lab_id=lab-1`);
    expect(req.request.method).toBe('GET');
    req.flush([]);
  });

  it('createLink() should POST to /links', () => {
    const link = createMockLink();
    service.createLink(link).subscribe(res => expect(res).toEqual(link));
    const req = httpMock.expectOne(`${API}/links`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual(link);
    req.flush(link);
  });

  it('deleteLink() should DELETE /links/:id', () => {
    service.deleteLink('link-1').subscribe();
    const req = httpMock.expectOne(`${API}/links/link-1`);
    expect(req.request.method).toBe('DELETE');
    req.flush(null);
  });

  // --- Laboratories ---

  it('getLaboratories() should GET /laboratories', () => {
    const labs = [createMockLaboratory()];
    service.getLaboratories().subscribe(res => expect(res).toEqual(labs));
    const req = httpMock.expectOne(`${API}/laboratories`);
    expect(req.request.method).toBe('GET');
    req.flush(labs);
  });

  it('createLaboratory() should POST to /laboratories', () => {
    const lab = { id: 'lab-2', name: 'Test Lab' };
    service.createLaboratory(lab).subscribe();
    const req = httpMock.expectOne(`${API}/laboratories`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual(lab);
    req.flush({ ...lab, status: 'inactive' });
  });

  it('activateLaboratory() should POST to /laboratories/:id/activate', () => {
    service.activateLaboratory('lab-1').subscribe();
    const req = httpMock.expectOne(`${API}/laboratories/lab-1/activate`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({});
    req.flush(null);
  });

  it('updateLaboratory() should PATCH /laboratories/:id', () => {
    service.updateLaboratory('lab-1', 'New Name').subscribe();
    const req = httpMock.expectOne(`${API}/laboratories/lab-1`);
    expect(req.request.method).toBe('PATCH');
    expect(req.request.body).toEqual({ name: 'New Name' });
    req.flush({ id: 'lab-1', name: 'New Name', status: 'active' });
  });

  it('deleteLaboratory() should DELETE /laboratories/:id', () => {
    service.deleteLaboratory('lab-1').subscribe();
    const req = httpMock.expectOne(`${API}/laboratories/lab-1`);
    expect(req.request.method).toBe('DELETE');
    req.flush(null);
  });

  // --- System ---

  it('cleanupLaboratory() should DELETE /laboratories/:id/cleanup', () => {
    service.cleanupLaboratory('lab-1').subscribe();
    const req = httpMock.expectOne(`${API}/laboratories/lab-1/cleanup`);
    expect(req.request.method).toBe('DELETE');
    req.flush(null);
  });

  it('saveLabState() should POST /laboratories/:id/save-state', () => {
    const response = { message: 'ok', ips_saved: 2, routes_saved: 1 };
    service.saveLabState('lab-1').subscribe(res => expect(res).toEqual(response));
    const req = httpMock.expectOne(`${API}/laboratories/lab-1/save-state`);
    expect(req.request.method).toBe('POST');
    req.flush(response);
  });

  it('exportTopology() should GET /topology/export as blob', () => {
    service.exportTopology().subscribe(res => expect(res instanceof Blob).toBe(true));
    const req = httpMock.expectOne(`${API}/topology/export`);
    expect(req.request.method).toBe('GET');
    expect(req.request.responseType).toBe('blob');
    req.flush(new Blob(['yaml content']));
  });

  it('importTopology() should POST /topology/import with yaml content-type', () => {
    const yaml = 'name: Test\nnodes: []';
    service.importTopology(yaml).subscribe();
    const req = httpMock.expectOne(`${API}/topology/import`);
    expect(req.request.method).toBe('POST');
    expect(req.request.headers.get('Content-Type')).toBe('application/x-yaml');
    expect(req.request.body).toBe(yaml);
    req.flush(null);
  });

  // --- Traceroute ---

  it('runTraceroute() should POST /nodes/:id/traceroute with destination', () => {
    const mockResponse = createMockTracerouteResponse();
    service.runTraceroute('node-1', '10.0.3.2').subscribe(res => {
      expect(res.hops.length).toBe(3);
      expect(res.node_ids).toEqual(['node-1', 'node-2', 'node-3', 'node-4']);
      expect(res.link_ids).toEqual(['link-1', 'link-2', 'link-3']);
    });
    const req = httpMock.expectOne(`${API}/nodes/node-1/traceroute`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({ destination: '10.0.3.2' });
    req.flush(mockResponse);
  });

  // --- Domains ---

  it('getDomains() should GET /laboratories/:id/domains', () => {
    const mockDomains = createMockDomainsResponse();
    service.getDomains('lab-1').subscribe(res => expect(res).toEqual(mockDomains));
    const req = httpMock.expectOne(`${API}/laboratories/lab-1/domains`);
    expect(req.request.method).toBe('GET');
    req.flush(mockDomains);
  });

  // --- Legacy ---

  it('deploy() should POST /topology/deploy', () => {
    const topology = { id: 'lab-1', name: 'Test', nodes: [], links: [] };
    service.deploy(topology).subscribe();
    const req = httpMock.expectOne(`${API}/topology/deploy`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual(topology);
    req.flush({ status: 'ok' });
  });
});
