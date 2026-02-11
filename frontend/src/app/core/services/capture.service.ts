import { inject, Injectable } from '@angular/core';
import { BehaviorSubject, Subject } from 'rxjs';
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
  private connectedSubject = new BehaviorSubject<boolean>(false);
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 2000;
  private currentNodeId?: string;
  private currentInterface?: string;
  private isIntentionallyClosed = false;

  packets$ = this.packetSubject.asObservable();
  connected$ = this.connectedSubject.asObservable();

  startCapture(nodeId: string, interfaceName: string): void {
    this.isIntentionallyClosed = false;
    this.currentNodeId = nodeId;
    this.currentInterface = interfaceName;
    this.reconnectAttempts = 0;
    this.connect(nodeId, interfaceName);
  }

  private connect(nodeId: string, interfaceName: string): void {
    // Close existing socket without marking as intentional
    if (this.socket) {
      this.socket.close();
      this.socket = undefined;
    }

    const baseWsUrl = environment.apiUrl.replace(/^http/, 'ws');
    const url = `${baseWsUrl}/sniff?node_id=${nodeId}&interface=${interfaceName}`;

    console.log(`[Capture] Connecting to ${url}`);
    this.socket = new WebSocket(url);

    this.socket.onopen = () => {
      console.log('[Capture] WebSocket connected');
      this.reconnectAttempts = 0;
      this.connectedSubject.next(true);
    };

    this.socket.onmessage = (event) => {
      try {
        const packet: PacketSummary = JSON.parse(event.data);
        this.packetSubject.next(packet);
      } catch (e) {
        console.error('Error parsing packet data', e);
      }
    };

    this.socket.onerror = (error) => {
      console.error('[Capture] WebSocket error:', error);
    };

    this.socket.onclose = (event) => {
      console.log(`[Capture] WebSocket closed: ${event.code} ${event.reason}`);
      this.connectedSubject.next(false);

      // Only reconnect if not intentionally closed and not exceeded max attempts
      if (!this.isIntentionallyClosed && this.reconnectAttempts < this.maxReconnectAttempts) {
        this.reconnectAttempts++;
        console.log(`[Capture] Reconnecting in ${this.reconnectDelay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
        setTimeout(() => {
          if (this.currentNodeId && this.currentInterface) {
            this.connect(this.currentNodeId, this.currentInterface);
          }
        }, this.reconnectDelay);
      } else if (this.reconnectAttempts >= this.maxReconnectAttempts) {
        this.toast.error('Capture connection lost. Please restart capture.');
      }
    };
  }

  stopCapture(): void {
    this.isIntentionallyClosed = true;
    if (this.socket) {
      this.socket.close();
      this.socket = undefined;
    }
  }
}
