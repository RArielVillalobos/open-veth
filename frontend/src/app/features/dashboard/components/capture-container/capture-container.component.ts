import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { UIStore } from '../../../../state/ui.store';
import { PacketCaptureWindowComponent } from '../../../../shared/components/packet-capture-window/packet-capture-window.component';

@Component({
  selector: 'app-capture-container',
  standalone: true,
  imports: [CommonModule, PacketCaptureWindowComponent],
  template: `
    <div class="fixed inset-0 pointer-events-none z-50 p-8">
      @for (cap of ui.activeCaptures(); track cap.nodeId + cap.interfaceName) {
        <div class="absolute bottom-8 right-8 pointer-events-auto">
          <app-packet-capture-window 
            [nodeId]="cap.nodeId"
            [nodeName]="cap.nodeName"
            [interfaceName]="cap.interfaceName"
            (onClose)="ui.closeCapture(cap.nodeId, cap.interfaceName)">
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
  readonly ui = inject(UIStore);
}
