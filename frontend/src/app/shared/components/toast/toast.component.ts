import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ToastService } from '../../../core/services/toast.service';

@Component({
  selector: 'app-toast',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="fixed bottom-4 right-4 z-[9999] flex flex-col gap-2 pointer-events-none">
      <!-- Los toasts individuales sí capturan eventos -->
      @for (toast of toastService.toasts(); track toast.id) {
        <div class="pointer-events-auto px-4 py-3 rounded shadow-lg text-white min-w-[300px] flex justify-between items-center transition-all animate-slide-up"
             [ngClass]="{
               'bg-red-600': toast.type === 'error',
               'bg-green-600': toast.type === 'success',
               'bg-blue-600': toast.type === 'info'
             }">
          <span class="text-sm font-medium">{{ toast.message }}</span>
          <button (click)="toastService.remove(toast.id)" class="ml-4 hover:bg-white/20 rounded p-1">
            ✕
          </button>
        </div>
      }
    </div>
  `,
  styles: [`
    @keyframes slide-up {
      from { transform: translateY(100%); opacity: 0; }
      to { transform: translateY(0); opacity: 1; }
    }
    .animate-slide-up {
      animation: slide-up 0.3s ease-out forwards;
    }
  `]
})
export class ToastComponent {
  toastService = inject(ToastService);
}
