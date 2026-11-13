import { Component, Injectable, inject } from '@angular/core';
import { DIALOG_DATA, Dialog, DialogRef } from '@angular/cdk/dialog';
import { firstValueFrom } from 'rxjs';

interface Decision {
  title: string;
  message: string;
  actions: { value: string; label: string; destructive?: boolean }[];
}

@Component({
  standalone: true,
  template: `
    <section class="decision-dialog">
      <h2 id="decision-title">{{ data.title }}</h2>
      <p id="decision-description">{{ data.message }}</p>
      <div class="decision-actions">
        <button class="btn btn--secondary" (click)="ref.close()">Cancel</button>
        @for (action of data.actions; track action.value) {
          <button class="btn" [class.btn--danger]="action.destructive"
                  [class.btn--primary]="!action.destructive" (click)="ref.close(action.value)">
            {{ action.label }}
          </button>
        }
      </div>
    </section>
  `,
})
class DecisionDialogComponent {
  readonly data = inject<Decision>(DIALOG_DATA);
  readonly ref = inject(DialogRef<string>);
}

@Injectable({ providedIn: 'root' })
export class DecisionDialogService {
  private dialog = inject(Dialog);

  choose(data: Decision) {
    return firstValueFrom(this.dialog.open<string>(DecisionDialogComponent, {
      data, ariaLabelledBy: 'decision-title', ariaDescribedBy: 'decision-description',
      autoFocus: 'first-tabbable', restoreFocus: true, maxWidth: 'calc(100vw - 32px)',
    }).closed);
  }

  async confirm(title: string, message: string, label: string) {
    return await this.choose({ title, message, actions: [{ value: 'confirm', label, destructive: true }] }) === 'confirm';
  }
}
