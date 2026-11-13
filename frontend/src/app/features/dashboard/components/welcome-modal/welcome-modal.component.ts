import { Component, output, HostListener } from '@angular/core';
import { A11yModule } from '@angular/cdk/a11y';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-welcome-modal',
  standalone: true,
  imports: [CommonModule, A11yModule],
  templateUrl: './welcome-modal.component.html',
  styleUrl: './welcome-modal.component.scss'
})
export class WelcomeModalComponent {
  createLab = output<void>();
  importLab = output<void>();
  openLabManager = output<void>();
  close = output<void>();

  @HostListener('document:keydown.escape')
  onEscape() {
    this.close.emit();
  }

  onBackdropClick(event: MouseEvent) {
    if (event.target === event.currentTarget) {
      this.close.emit();
    }
  }
}
