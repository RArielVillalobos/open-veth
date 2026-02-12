import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ToastService } from '../../../core/services/toast.service';

@Component({
  selector: 'app-toast',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="fixed bottom-4 right-4 z-[9999] flex flex-col gap-3 pointer-events-none">
      @for (toast of toastService.toasts(); track toast.id) {
        <div 
          class="pointer-events-auto px-4 py-3 rounded-lg shadow-xl min-w-[320px] max-w-[400px] flex items-start gap-3 transition-all animate-slide-up border-l-4"
          [ngClass]="{
            'bg-red-50 border-red-500 text-red-900': toast.type === 'error',
            'bg-green-50 border-green-500 text-green-900': toast.type === 'success',
            'bg-blue-50 border-blue-500 text-blue-900': toast.type === 'info'
          }"
        >
          <!-- Icon -->
          <div class="flex-shrink-0 mt-0.5">
            @switch (toast.type) {
              @case ('error') {
                <svg class="w-5 h-5 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                </svg>
              }
              @case ('success') {
                <svg class="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                </svg>
              }
              @case ('info') {
                <svg class="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                </svg>
              }
            }
          </div>

          <!-- Content -->
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium leading-5">{{ toast.message }}</p>
            <p class="text-xs mt-1 opacity-70">
              @switch (toast.type) {
                @case ('error') { Error }
                @case ('success') { Success }
                @case ('info') { Info }
              }
            </p>
          </div>

          <!-- Close button -->
          <button 
            (click)="toastService.remove(toast.id)" 
            class="flex-shrink-0 -mr-1 -mt-1 p-1.5 rounded-full hover:bg-black/5 transition-colors"
            [ngClass]="{
              'text-red-400 hover:text-red-600': toast.type === 'error',
              'text-green-400 hover:text-green-600': toast.type === 'success',
              'text-blue-400 hover:text-blue-600': toast.type === 'info'
            }"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
            </svg>
          </button>
        </div>
      }
    </div>
  `,
  styles: [`
    @keyframes slide-up {
      from { 
        transform: translateY(100%) scale(0.95); 
        opacity: 0; 
      }
      to { 
        transform: translateY(0) scale(1); 
        opacity: 1; 
      }
    }
    
    @keyframes fade-out {
      from { 
        transform: translateX(0) scale(1); 
        opacity: 1; 
      }
      to { 
        transform: translateX(100%) scale(0.95); 
        opacity: 0; 
      }
    }
    
    .animate-slide-up {
      animation: slide-up 0.3s cubic-bezier(0.16, 1, 0.3, 1) forwards;
    }
    
    .animate-fade-out {
      animation: fade-out 0.2s ease-out forwards;
    }
  `]
})
export class ToastComponent {
  toastService = inject(ToastService);
}
