import { Component, inject, Output, EventEmitter, signal, computed, ElementRef, viewChildren } from '@angular/core';
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

  editingLabId = signal<string | null>(null);
  editingName = signal('');

  renameInputs = viewChildren<ElementRef<HTMLInputElement>>('renameInput');

  nameError = computed(() => {
    const name = this.editingName().trim();
    const editingId = this.editingLabId();
    if (!name) return '';
    const duplicate = this.store.laboratories().some(
      l => l.id !== editingId && l.name.toLowerCase() === name.toLowerCase()
    );
    return duplicate ? 'Name already exists' : '';
  });

  canSave = computed(() => {
    const name = this.editingName().trim();
    return name.length > 0 && !this.nameError();
  });

  createLab(name: string) {
    if (!name.trim()) return;
    this.store.createLaboratory(name.trim());
  }

  switchLab(id: string) {
    this.store.switchLab(id);
    this.close.emit();
  }

  deleteLab(id: string) {
    if (confirm('Are you sure you want to delete this laboratory? This cannot be undone.')) {
      this.store.deleteLaboratory(id);
    }
  }

  startEditing(lab: { id: string; name: string }) {
    this.editingLabId.set(lab.id);
    this.editingName.set(lab.name);
    setTimeout(() => {
      const inputs = this.renameInputs();
      if (inputs.length > 0) {
        inputs[0].nativeElement.focus();
        inputs[0].nativeElement.select();
      }
    });
  }

  cancelEditing() {
    this.editingLabId.set(null);
    this.editingName.set('');
  }

  saveEditing() {
    if (!this.canSave()) return;
    const id = this.editingLabId();
    const name = this.editingName().trim();

    if (id === this.store.currentLabId()) {
      this.store.renameLaboratory(name);
    }
    this.cancelEditing();
  }
}