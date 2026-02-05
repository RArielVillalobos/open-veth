import { inject, Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Topology, Node, Link, InterfaceInfo, RouteInfo } from '../../models/topology.model';

@Injectable({
  providedIn: 'root'
})
export class TopologyService {
  private http = inject(HttpClient);
  private apiUrl = 'http://localhost:8080/api/v1';

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

  deleteLink(id: string): Observable<void> {
    return this.http.delete<void>(`${this.apiUrl}/links/${id}`);
  }

  // --- Laboratories ---
  getLaboratories(): Observable<any[]> {
    return this.http.get<any[]>(`${this.apiUrl}/laboratories`);
  }

  createLaboratory(lab: { id: string, name: string }): Observable<any> {
    return this.http.post<any>(`${this.apiUrl}/laboratories`, lab);
  }

  activateLaboratory(id: string): Observable<any> {
    return this.http.post(`${this.apiUrl}/laboratories/${id}/activate`, {});
  }

  updateLaboratory(id: string, name: string): Observable<any> {
    return this.http.patch(`${this.apiUrl}/laboratories/${id}`, { name });
  }

  deleteLaboratory(id: string): Observable<void> {
    return this.http.delete<void>(`${this.apiUrl}/laboratories/${id}`);
  }

  // --- Sistema ---

  cleanupLaboratory(labId: string): Observable<any> {
    return this.http.delete(`${this.apiUrl}/laboratories/${labId}/cleanup`);
  }

  exportTopology(): Observable<Blob> {
    return this.http.get(`${this.apiUrl}/topology/export`, { responseType: 'blob' });
  }

  importTopology(yamlContent: string): Observable<any> {
    return this.http.post(`${this.apiUrl}/topology/import`, yamlContent, {
      headers: { 'Content-Type': 'application/x-yaml' }
    });
  }

  // --- Legacy (Batch Deploy) ---

  deploy(topology: Topology): Observable<any> {

    // Usamos el endpoint antiguo que restauramos en el backend

    return this.http.post(`${this.apiUrl}/topology/deploy`, topology);

  }

}

