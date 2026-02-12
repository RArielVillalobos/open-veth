import { Injectable, signal } from '@angular/core';

export interface Toast {
  id: number;
  message: string;
  type: 'success' | 'error' | 'info';
  duration: number;
}

@Injectable({ providedIn: 'root' })
export class ToastService {
  readonly toasts = signal<Toast[]>([]);
  private nextId = 1;
  private maxToasts = 3;

  // Duration in milliseconds for each type
  private readonly durations = {
    error: 8000,    // Errors need more time to read
    success: 3000,  // Success is just confirmation
    info: 5000      // Info is in between
  };

  show(message: string, type: 'success' | 'error' | 'info' = 'info') {
    const id = this.nextId++;
    const duration = this.durations[type];
    
    // Limit to max toasts - remove oldest if needed
    this.toasts.update(current => {
      const newToasts = [...current, { id, message, type, duration }];
      if (newToasts.length > this.maxToasts) {
        // Remove the oldest toast (first in array)
        return newToasts.slice(1);
      }
      return newToasts;
    });
    
    // Auto-remove based on type duration
    setTimeout(() => this.remove(id), duration);
  }

  error(message: string) {
    this.show(message, 'error');
  }

  success(message: string) {
    this.show(message, 'success');
  }

  info(message: string) {
    this.show(message, 'info');
  }

  remove(id: number) {
    this.toasts.update(current => current.filter(t => t.id !== id));
  }

  clearAll() {
    this.toasts.set([]);
  }
}
