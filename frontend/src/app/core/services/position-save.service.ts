import { DestroyRef, Injectable, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { TopologyService } from './topology.service';

interface PendingPosition {
  labId: string;
  x: number;
  y: number;
}

@Injectable({ providedIn: 'root' })
export class PositionSaveService {
  private api = inject(TopologyService);
  private pending = new Map<string, PendingPosition>();
  private timer: ReturnType<typeof setTimeout> | undefined;
  private draining: Promise<boolean> | null = null;
  readonly count = signal(0);
  readonly saving = signal(false);
  readonly error = signal<string | null>(null);

  constructor() {
    inject(DestroyRef).onDestroy(() => clearTimeout(this.timer));
  }

  get(id: string) {
    return this.pending.get(id);
  }

  enqueue(id: string, position: PendingPosition) {
    this.pending.set(id, position);
    this.count.set(this.pending.size);
    clearTimeout(this.timer);
    this.timer = setTimeout(() => void this.flush(), 500);
  }

  discard(id: string) {
    this.pending.delete(id);
    this.count.set(this.pending.size);
    if (!this.pending.size) this.error.set(null);
  }

  flush(): Promise<boolean> {
    clearTimeout(this.timer);
    if (this.draining) return this.draining;
    if (!this.pending.size) return Promise.resolve(true);
    this.saving.set(true);
    this.error.set(null);
    this.draining = this.drain().finally(() => {
      this.draining = null;
      this.saving.set(false);
    });
    return this.draining;
  }

  private async drain() {
    while (this.pending.size) {
      const [id, position] = this.pending.entries().next().value!;
      try {
        await firstValueFrom(this.api.updateNodePosition(id, position.x, position.y));
        // A drag during the request must still be saved afterwards.
        if (this.pending.get(id) === position) this.pending.delete(id);
        this.count.set(this.pending.size);
      } catch {
        if (!this.pending.has(id)) continue;
        this.error.set('Some node positions could not be saved. Retry before leaving this laboratory.');
        return false;
      }
    }
    return true;
  }
}
