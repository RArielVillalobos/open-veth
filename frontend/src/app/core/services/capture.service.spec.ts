import { TestBed } from '@angular/core/testing';
import { CaptureService } from './capture.service';
import { ToastService } from './toast.service';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

class MockWebSocket {
  onopen: ((event: any) => void) | null = null;
  onmessage: ((event: any) => void) | null = null;
  onerror: ((event: any) => void) | null = null;
  onclose: ((event: any) => void) | null = null;
  close = vi.fn();
  constructor(public url: string) {
    MockWebSocket.lastInstance = this;
  }
  static lastInstance: MockWebSocket | null = null;
}

describe('CaptureService', () => {
  let service: CaptureService;
  let toastService: ToastService;
  const originalWebSocket = globalThis.WebSocket;

  beforeEach(() => {
    MockWebSocket.lastInstance = null;
    vi.stubGlobal('WebSocket', MockWebSocket);

    TestBed.configureTestingModule({
      providers: [
        CaptureService,
        {
          provide: ToastService,
          useValue: { error: vi.fn(), success: vi.fn(), info: vi.fn(), show: vi.fn() },
        },
      ],
    });
    service = TestBed.inject(CaptureService);
    toastService = TestBed.inject(ToastService);
  });

  afterEach(() => {
    vi.stubGlobal('WebSocket', originalWebSocket);
  });

  it('startCapture() should create WebSocket with correct URL', () => {
    service.startCapture('node-1', 'eth1');
    const ws = MockWebSocket.lastInstance!;
    expect(ws.url).toContain('ws://');
    expect(ws.url).toContain('/sniff?node_id=node-1&interface=eth1');
  });

  it('startCapture() should close previous socket first', () => {
    service.startCapture('node-1', 'eth1');
    const firstWs = MockWebSocket.lastInstance!;

    service.startCapture('node-2', 'eth2');
    expect(firstWs.close).toHaveBeenCalled();
  });

  it('should emit parsed packet on valid message', () => {
    const packet = {
      timestamp: '2026-01-01',
      source: '10.0.0.1',
      destination: '10.0.0.2',
      protocol: 'ICMP',
      length: 64,
      ttl: 64,
      info: 'echo request',
    };

    const received: any[] = [];
    service.packets$.subscribe(p => received.push(p));

    service.startCapture('node-1', 'eth1');
    const ws = MockWebSocket.lastInstance!;
    ws.onmessage!({ data: JSON.stringify(packet) });

    expect(received.length).toBe(1);
    expect(received[0]).toEqual(packet);
  });

  it('should not emit on invalid JSON message', () => {
    const received: any[] = [];
    service.packets$.subscribe(p => received.push(p));
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    service.startCapture('node-1', 'eth1');
    const ws = MockWebSocket.lastInstance!;
    ws.onmessage!({ data: 'not json' });

    expect(received.length).toBe(0);
    expect(consoleSpy).toHaveBeenCalled();
  });

  it('should log error on socket error', () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    service.startCapture('node-1', 'eth1');
    const ws = MockWebSocket.lastInstance!;
    ws.onerror!(new Event('error'));

    expect(consoleSpy).toHaveBeenCalled();
  });

  it('connected$ should emit true on open and false on close', () => {
    const states: boolean[] = [];
    service.connected$.subscribe(s => states.push(s));

    service.startCapture('node-1', 'eth1');
    const ws = MockWebSocket.lastInstance!;

    ws.onopen!(new Event('open'));
    ws.onclose!({ code: 1000, reason: '' });

    expect(states).toEqual([false, true, false]);
  });

  it('stopCapture() should close socket', () => {
    service.startCapture('node-1', 'eth1');
    const ws = MockWebSocket.lastInstance!;

    service.stopCapture();
    expect(ws.close).toHaveBeenCalled();
  });

  it('stopCapture() should not throw when no socket exists', () => {
    expect(() => service.stopCapture()).not.toThrow();
  });
});
