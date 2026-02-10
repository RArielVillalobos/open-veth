import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CaptureStore } from '../../../../state/capture.store';
import { PacketCaptureWindowComponent } from '../../../../shared/components/packet-capture-window/packet-capture-window.component';

@Component({
  selector: 'app-capture-container',
  standalone: true,
  imports: [CommonModule, PacketCaptureWindowComponent],
  template: `
    <div class="fixed inset-0 pointer-events-none z-50 p-8">
      @for (cap of captureStore.sessions(); track cap.nodeId + cap.interfaceName) {
        <div class="absolute bottom-8 right-8 pointer-events-auto">
          <app-packet-capture-window 
            [nodeId]="cap.nodeId"
            [nodeName]="cap.nodeName"
            [interfaceName]="cap.interfaceName"
            (onClose)="captureStore.closeCapture(cap.nodeId, cap.interfaceName)">
          </app-packet-capture-window>
        </div>
      }
    </div>
  `,
  styles: [`
    :host { display: block; }
  `]
})
export class CaptureContainerComponent {
  readonly captureStore = inject(CaptureStore);
}
