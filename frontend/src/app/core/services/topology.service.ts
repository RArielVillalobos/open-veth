import { inject, Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Topology } from '../../models/topology.model';

@Injectable({
  providedIn: 'root'
})
export class TopologyService {
  private http = inject(HttpClient);
  // URL base hardcodeada por ahora (luego irá a environment)
  private apiUrl = 'http://localhost:8080/api/v1/topology';

  deploy(topology: Topology): Observable<any> {
    return this.http.post(`${this.apiUrl}/deploy`, topology);
  }

  cleanup(): Observable<any> {
    return this.http.delete(`${this.apiUrl}/cleanup`);
  }
}
