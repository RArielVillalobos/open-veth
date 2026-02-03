import { Injectable, inject } from '@angular/core';
import { Observable, Subject } from 'rxjs';

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
  private socket?: WebSocket;
  private packetSubject = new Subject<PacketSummary>();

  packets$ = this.packetSubject.asObservable();

  startCapture(nodeId: string, interfaceName: string): void {
    this.stopCapture();

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.hostname;
    const port = '8080'; // Backend port
    const url = `${protocol}//${host}:${port}/api/v1/sniff?node_id=${nodeId}&interface=${interfaceName}`;

    this.socket = new WebSocket(url);

    this.socket.onmessage = (event) => {
      try {
        const packet: PacketSummary = JSON.parse(event.data);
        this.packetSubject.next(packet);
      } catch (e) {
        console.error('Error parsing packet data', e);
      }
    };

    this.socket.onclose = () => {
      console.log('Capture WebSocket closed');
    };

    this.socket.onerror = (error) => {
      console.error('Capture WebSocket error', error);
    };
  }

  stopCapture(): void {
    if (this.socket) {
      this.socket.close();
      this.socket = undefined;
    }
  }
}
