import { Component, output, input } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-topology-toolbar',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './topology-toolbar.component.html',
  styleUrl: './topology-toolbar.component.scss'
})
export class TopologyToolbarComponent {
  isLoading = input<boolean>(false);
  
  addNode = output<'router' | 'host' | 'switch'>();
  deploy = output<void>();
  cleanup = output<void>();
}