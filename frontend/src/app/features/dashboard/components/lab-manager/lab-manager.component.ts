import { Component, inject, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TopologyStore } from '../../../../state/topology.store';

@Component({
  selector: 'app-lab-manager',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './lab-manager.component.html',
  styleUrl: './lab-manager.component.scss'
})
export class LabManagerComponent {
  readonly store = inject(TopologyStore);
  
  @Output() close = new EventEmitter<void>();

  createLab(name: string) {
    if (!name.trim()) return;
    this.store.createLaboratory(name.trim());
  }

  switchLab(id: string) {
    this.store.switchLab(id);
    this.close.emit(); // Auto close on switch
  }

  deleteLab(id: string) {
    if(confirm('Are you sure you want to delete this laboratory? This cannot be undone.')) {
      this.store.deleteLaboratory(id);
    }
  }
}