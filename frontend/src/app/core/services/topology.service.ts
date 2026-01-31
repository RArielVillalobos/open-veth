import { inject, Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Topology, Node, Link } from '../../models/topology.model';

@Injectable({
  providedIn: 'root'
})
export class TopologyService {
  private http = inject(HttpClient);
  private apiUrl = 'http://localhost:8080/api/v1';

  // --- Nodos ---
  getNodes(): Observable<Node[]> {
    return this.http.get<Node[]>(`${this.apiUrl}/nodes`);
  }

  createNode(node: Node): Observable<Node> {
    return this.http.post<Node>(`${this.apiUrl}/nodes`, node);
  }

  deleteNode(id: string): Observable<void> {
    return this.http.delete<void>(`${this.apiUrl}/nodes/${id}`);
  }

  // --- Links ---
  getLinks(): Observable<Link[]> {
    return this.http.get<Link[]>(`${this.apiUrl}/links`);
  }

  createLink(link: Link): Observable<Link> {
    return this.http.post<Link>(`${this.apiUrl}/links`, link);
  }

  deleteLink(id: string): Observable<void> {
    return this.http.delete<void>(`${this.apiUrl}/links/${id}`);
  }

  // --- Sistema ---

  cleanup(): Observable<any> {

    return this.http.delete(`${this.apiUrl}/system/cleanup`);

  }



  // --- Legacy (Batch Deploy) ---

  deploy(topology: Topology): Observable<any> {

    // Usamos el endpoint antiguo que restauramos en el backend

    return this.http.post(`${this.apiUrl}/topology/deploy`, topology);

  }

}

