import { Injectable, inject, NgZone, DestroyRef } from '@angular/core';
import { TopologyStore } from '../../state/topology.store';
import { environment } from '../../../environments/environment';

interface NetworkEvent {
  type: string;
  nodeId: string;
  labId?: string;
  timestamp: number;
  data?: Record<string, unknown>;
}

@Injectable({
  providedIn: 'root'
})
export class NetworkEventsService {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  private destroyRef = inject(DestroyRef);
  private store = inject(TopologyStore);
  private ngZone = inject(NgZone);

  private readonly maxReconnectAttempts = 5;
  private readonly reconnectDelay = 3000;
  private readonly enableLogs = !environment.production;

  constructor() {
    this.destroyRef.onDestroy(() => {
      this.disconnect();
    });
  }

  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.log('Already connected');
      return;
    }

    const wsUrl = environment.apiUrl
      .replace('https://', 'wss://')
      .replace('http://', 'ws://')
      .replace('/api/v1', '/api/v1/events');

    this.log('Connecting to:', wsUrl);

    try {
      this.ws = new WebSocket(wsUrl);
    } catch (err) {
      this.error('Failed to create WebSocket:', err);
      this.attemptReconnect();
      return;
    }

    this.ws.onopen = () => {
      this.ngZone.run(() => {
        this.log('Connected');
        this.reconnectAttempts = 0;
      });
    };

    this.ws.onmessage = (event) => {
      this.ngZone.run(() => {
        try {
          const networkEvent: NetworkEvent = JSON.parse(event.data);
          this.handleEvent(networkEvent);
        } catch (err) {
          this.error('Failed to parse event:', err);
        }
      });
    };

    this.ws.onclose = (event) => {
      this.ngZone.run(() => {
        this.log('Disconnected', event.code, event.reason);
        if (event.code !== 1000) {
          this.attemptReconnect();
        }
      });
    };

    this.ws.onerror = (error) => {
      this.ngZone.run(() => {
        this.error('WebSocket error:', error);
      });
    };
  }

  disconnect(): void {
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }

    this.reconnectAttempts = 0;

    if (this.ws) {
      this.ws.close(1000, 'Client disconnected');
      this.ws = null;
    }
  }

  private handleEvent(event: NetworkEvent): void {
    switch (event.type) {
      case 'interface:changed':
        if (event.nodeId) {
          this.log('Fetching interfaces for node:', event.nodeId);
          this.store.fetchNodeInterfaces(event.nodeId);
        }
        break;

      case 'node:created':
      case 'node:updated':
      case 'link:created':
        this.store.loadTopology();
        break;

      case 'node:stopped':
        if (event.nodeId) this.store.updateNodeStatus(event.nodeId, 'stopped');
        break;

      case 'node:started':
        if (event.nodeId) this.store.updateNodeStatus(event.nodeId, 'running');
        break;

      case 'lab:activated':
        if (event.labId) this.store.onLabActivated(event.labId);
        break;

      case 'lab:activation_failed':
        this.store.onLabActivationFailed(
          event.labId ?? '',
          (event.data?.['reason'] as string) || 'unknown error'
        );
        break;

      default:
        this.log('Unknown event type:', event.type);
    }
  }

  private attemptReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      this.error('Max reconnect attempts reached');
      return;
    }

    this.reconnectAttempts++;
    this.log(`Reconnecting in ${this.reconnectDelay}ms (attempt ${this.reconnectAttempts})`);

    this.reconnectTimeout = setTimeout(() => {
      this.reconnectTimeout = null;
      this.connect();
    }, this.reconnectDelay);
  }

  private log(...args: any[]): void {
    if (this.enableLogs) {
      console.log('[NetworkEvents]', ...args);
    }
  }

  private error(...args: any[]): void {
    if (this.enableLogs) {
      console.error('[NetworkEvents]', ...args);
    }
  }
}
