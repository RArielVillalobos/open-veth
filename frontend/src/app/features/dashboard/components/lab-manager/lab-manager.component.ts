import { Component, inject, Output, EventEmitter, signal, computed, ElementRef, viewChildren, HostListener } from '@angular/core';
import { A11yModule } from '@angular/cdk/a11y';
import { DecisionDialogService } from '../../../../core/services/decision-dialog.service';
import { CommonModule } from '@angular/common';
import { TopologyStore } from '../../../../state/topology.store';

@Component({
  selector: 'app-lab-manager',
  standalone: true,
  imports: [CommonModule, A11yModule],
  templateUrl: './lab-manager.component.html',
  styleUrl: './lab-manager.component.scss'
})
export class LabManagerComponent {
  readonly store = inject(TopologyStore);

  @Output() close = new EventEmitter<void>();

  private decisions = inject(DecisionDialogService);
  newName = signal('');
  readonly busy = computed(() => this.store.isLoading() || ['requesting', 'activating', 'uncertain'].includes(this.store.activationPhase()));
  @HostListener('document:keydown.escape') onEscape() { if (document.querySelector('cdk-dialog-container')) return; if (this.editingLabId()) this.cancelEditing(); else if (!this.busy()) this.close.emit(); }

  editingLabId = signal<string | null>(null);
  editingName = signal('');
  activatingLabId = signal<string | null>(null);

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

  async createLab() {
    if (this.busy() || !this.newName().trim()) return;
    if (await this.store.createLaboratory(this.newName().trim())) this.close.emit();
  }

  async switchLab(id: string) {
    if (this.busy()) return;
    this.activatingLabId.set(id);
    const accepted = await this.store.switchLab(id);
    this.activatingLabId.set(null);
    if (accepted) this.close.emit();
  }

  async deleteLab(id: string) {
    if (this.busy()) return;
    const lab = this.store.laboratories().find(item => item.id === id);
    if (await this.decisions.confirm('Delete laboratory?', 'Delete "' + lab?.name + '" and all its nodes, links, saved settings, and snapshots? This cannot be undone.', 'Delete laboratory')) await this.store.deleteLaboratory(id);
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