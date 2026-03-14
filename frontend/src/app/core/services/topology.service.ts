import { inject, Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Topology, Node, Link, InterfaceInfo, RouteInfo, Laboratory, LaboratoryCreate, SaveStateResponse, DomainsResponse, TracerouteResponse } from '../../models/topology.model';
import { environment } from '../../../environments/environment';

@Injectable({
  providedIn: 'root'
})
export class TopologyService {
  private http = inject(HttpClient);
  private apiUrl = environment.apiUrl;

  // --- Nodos ---
  getNodes(live: boolean = false, labId?: string): Observable<Node[]> {
    let url = `${this.apiUrl}/nodes?live=${live}`;
    if (labId) {
      url += `&lab_id=${labId}`;
    }
    return this.http.get<Node[]>(url);
  }

  getNodeInterfaces(id: string): Observable<InterfaceInfo[]> {
    return this.http.get<InterfaceInfo[]>(`${this.apiUrl}/nodes/${id}/interfaces`);
  }

  getNodeRoutes(id: string): Observable<RouteInfo[]> {
    return this.http.get<RouteInfo[]>(`${this.apiUrl}/nodes/${id}/routes`);
  }

  createNode(node: Node): Observable<Node> {
    return this.http.post<Node>(`${this.apiUrl}/nodes`, node);
  }

  updateNodePosition(id: string, x: number, y: number): Observable<void> {
    return this.http.patch<void>(`${this.apiUrl}/nodes/${id}/position`, { x, y });
  }

  deleteNode(id: string): Observable<void> {
    return this.http.delete<void>(`${this.apiUrl}/nodes/${id}`);
  }

  // --- Links ---
  getLinks(labId?: string): Observable<Link[]> {
    let url = `${this.apiUrl}/links`;
    if (labId) {
      url += `?lab_id=${labId}`;
    }
    return this.http.get<Link[]>(url);
  }

  createLink(link: Link): Observable<Link> {
    return this.http.post<Link>(`${this.apiUrl}/links`, link);
  }

  toggleLink(id: string, enabled: boolean): Observable<Link> {
    return this.http.patch<Link>(`${this.apiUrl}/links/${id}/toggle`, { enabled });
  }

  deleteLink(id: string): Observable<void> {
    return this.http.delete<void>(`${this.apiUrl}/links/${id}`);
  }

  // --- Laboratories ---
  getLaboratories(): Observable<Laboratory[]> {
    return this.http.get<Laboratory[]>(`${this.apiUrl}/laboratories`);
  }

  createLaboratory(lab: LaboratoryCreate): Observable<Laboratory> {
    return this.http.post<Laboratory>(`${this.apiUrl}/laboratories`, lab);
  }

  activateLaboratory(id: string): Observable<void> {
    return this.http.post<void>(`${this.apiUrl}/laboratories/${id}/activate`, {});
  }

  updateLaboratory(id: string, name: string): Observable<Laboratory> {
    return this.http.patch<Laboratory>(`${this.apiUrl}/laboratories/${id}`, { name });
  }

  deleteLaboratory(id: string): Observable<void> {
    return this.http.delete<void>(`${this.apiUrl}/laboratories/${id}`);
  }

  // --- Sistema ---

  cleanupLaboratory(labId: string): Observable<void> {
    return this.http.delete<void>(`${this.apiUrl}/laboratories/${labId}/cleanup`);
  }

  saveLabState(labId: string): Observable<SaveStateResponse> {
    return this.http.post<SaveStateResponse>(
      `${this.apiUrl}/laboratories/${labId}/save-state`, {}
    );
  }

  getDomains(labId: string): Observable<DomainsResponse> {
    return this.http.get<DomainsResponse>(`${this.apiUrl}/laboratories/${labId}/domains`);
  }

  runTraceroute(nodeId: string, destination: string): Observable<TracerouteResponse> {
    return this.http.post<TracerouteResponse>(
      `${this.apiUrl}/nodes/${nodeId}/traceroute`, { destination }
    );
  }

  exportTopology(): Observable<Blob> {
    return this.http.get(`${this.apiUrl}/topology/export`, { responseType: 'blob' });
  }

  importTopology(yamlContent: string): Observable<void> {
    return this.http.post<void>(`${this.apiUrl}/topology/import`, yamlContent, {
      headers: { 'Content-Type': 'application/x-yaml' }
    });
  }

  // --- Legacy (Batch Deploy) ---

  deploy(topology: Topology): Observable<any> {
    // Usamos el endpoint antiguo que restauramos en el backend
    return this.http.post(`${this.apiUrl}/topology/deploy`, topology);
  }
}


