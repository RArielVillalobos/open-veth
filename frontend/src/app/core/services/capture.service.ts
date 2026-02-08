import { inject, Injectable } from '@angular/core';
import { Subject } from 'rxjs';
import { environment } from '../../../environments/environment';
import { ToastService } from './toast.service';

export interface PacketSummary {
  timestamp: string;
  source: string;
  destination: string;
  protocol: string;
  length: number;
  ttl: number;
  info: string;
}

@Injectable({
  providedIn: 'root'
})
export class CaptureService {
  private toast = inject(ToastService);
  private socket?: WebSocket;
  private packetSubject = new Subject<PacketSummary>();

  packets$ = this.packetSubject.asObservable();

  startCapture(nodeId: string, interfaceName: string): void {
    this.stopCapture();

    const baseWsUrl = environment.apiUrl.replace(/^http/, 'ws');
    const url = `${baseWsUrl}/sniff?node_id=${nodeId}&interface=${interfaceName}`;

    this.socket = new WebSocket(url);

    this.socket.onmessage = (event) => {
      try {
        const packet: PacketSummary = JSON.parse(event.data);
        this.packetSubject.next(packet);
      } catch (e) {
        console.error('Error parsing packet data', e);
      }
    };

    this.socket.onerror = () => {
      this.toast.error('Capture connection failed');
    };
  }

  stopCapture(): void {
    if (this.socket) {
      this.socket.close();
      this.socket = undefined;
    }
  }
}
