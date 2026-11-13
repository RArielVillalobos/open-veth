import { TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Subject, of, throwError } from 'rxjs';
import { PositionSaveService } from './position-save.service';
import { TopologyService } from './topology.service';

describe('PositionSaveService', () => {
  let service: PositionSaveService;
  const update = vi.fn();
  beforeEach(() => {
    vi.useFakeTimers(); update.mockReset().mockReturnValue(of(undefined));
    TestBed.configureTestingModule({ providers: [PositionSaveService, { provide: TopologyService, useValue: { updateNodePosition: update } }] });
    service = TestBed.inject(PositionSaveService);
  });
  afterEach(() => vi.useRealTimers());

  it('saves both nodes when they move within the debounce window', async () => {
    service.enqueue('a', { labId: 'lab', x: 10, y: 20 });
    service.enqueue('b', { labId: 'lab', x: 30, y: 40 });
    await vi.advanceTimersByTimeAsync(500);
    expect(update.mock.calls).toEqual([['a', 10, 20], ['b', 30, 40]]);
    expect(service.count()).toBe(0);
  });

  it('serializes writes and retains a newer drag while a write is in flight', async () => {
    const first = new Subject<void>(); update.mockReturnValueOnce(first);
    service.enqueue('a', { labId: 'lab', x: 1, y: 2 });
    const finished = service.flush();
    service.enqueue('a', { labId: 'lab', x: 5, y: 6 });
    service.enqueue('b', { labId: 'lab', x: 7, y: 8 });
    expect(update).toHaveBeenCalledTimes(1);
    first.next(); first.complete();
    expect(await finished).toBe(true);
    expect(update.mock.calls).toEqual([['a', 1, 2], ['a', 5, 6], ['b', 7, 8]]);
  });

  it('retains failed positions for an explicit retry', async () => {
    update.mockReturnValueOnce(throwError(() => new Error('offline')));
    service.enqueue('a', { labId: 'lab', x: 1, y: 2 });
    expect(await service.flush()).toBe(false);
    expect(service.count()).toBe(1);
    expect(service.error()).toContain('could not be saved');
    expect(await service.flush()).toBe(true);
    expect(service.count()).toBe(0);
    expect(service.error()).toBeNull();
  });
});
