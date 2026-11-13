import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ToastService } from '../../../core/services/toast.service';

@Component({
  selector: 'app-toast',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="notifications">
      @for (toast of toastService.toasts(); track toast.id) {
        <div class="notification" [attr.role]="toast.type === 'error' ? 'alert' : 'status'"
             [class.notification--error]="toast.type === 'error'"
             [class.notification--success]="toast.type === 'success'">
          <p>{{ toast.message }}</p>
          <button (click)="toastService.remove(toast.id)" aria-label="Dismiss notification">×</button>
        </div>
      }
    </div>
  `,
  styles: [`
    .notifications { position: fixed; bottom: 16px; right: 16px; z-index: 9999; display: flex; flex-direction: column; gap: 8px; pointer-events: none; }
    .notification { display: flex; align-items: flex-start; gap: 12px; width: min(380px, calc(100vw - 32px)); padding: 10px 12px; background: #131b1e; color: #dbe4e5; border: 1px solid #425256; border-left: 2px solid #648eac; border-radius: 2px; box-shadow: 0 4px 12px #0004; pointer-events: auto; }
    .notification--error { border-left-color: #d97b68; }
    .notification--success { border-left-color: #8eaf9f; }
    p { flex: 1; margin: 0; font-size: 13px; line-height: 1.5; }
    button { color: #8fa3a6; font-size: 18px; line-height: 1; padding: 2px; }
    button:hover { color: #dbe4e5; }
  `]
})
export class ToastComponent {
  toastService = inject(ToastService);
}
