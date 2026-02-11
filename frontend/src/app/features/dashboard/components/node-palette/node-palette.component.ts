import { Component, output, input, signal, computed, HostListener, ElementRef, viewChild, inject, Injector, afterNextRender } from '@angular/core';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { toSignal } from '@angular/core/rxjs-interop';
import { Node } from '../../../../models/topology.model';

type NodeType = 'router' | 'host' | 'switch' | 'hub';

const NAME_PATTERN = /^[a-zA-Z0-9_-]+$/;

@Component({
  selector: 'app-node-palette',
  standalone: true,
  imports: [ReactiveFormsModule],
  templateUrl: './node-palette.component.html'
})
export class NodePaletteComponent {
  private injector = inject(Injector);

  nodes = input<Node[]>([]);
  addNode = output<{ type: NodeType; name: string }>();

  pendingType = signal<NodeType | null>(null);
  nameCtrl = new FormControl('', { nonNullable: true });

  private nameValue = toSignal(this.nameCtrl.valueChanges, { initialValue: '' });

  nameInputEl = viewChild<ElementRef<HTMLInputElement>>('nameInputEl');

  readonly nodeTypeConfigs: { id: NodeType; label: string; subtitle: string; icon: string; placeholder: string; accentBorder: string; hoverBorder: string }[] = [
    { id: 'router', label: 'Router', subtitle: 'Layer 3', icon: 'router.svg', placeholder: 'e.g. R1', accentBorder: 'border-blue-500', hoverBorder: 'hover:border-blue-500/50' },
    { id: 'switch', label: 'Switch', subtitle: 'Layer 2', icon: 'switch.svg', placeholder: 'e.g. SW1', accentBorder: 'border-amber-500', hoverBorder: 'hover:border-amber-500/50' },
    { id: 'hub', label: 'Hub', subtitle: 'Layer 1 - Broadcast', icon: 'hub.svg', placeholder: 'e.g. HUB1', accentBorder: 'border-violet-500', hoverBorder: 'hover:border-violet-500/50' },
    { id: 'host', label: 'Host', subtitle: 'End device', icon: 'host.svg', placeholder: 'e.g. PC1', accentBorder: 'border-emerald-500', hoverBorder: 'hover:border-emerald-500/50' },
  ];

  nameError = computed(() => {
    const name = this.nameValue().trim();
    if (!name) return '';
    if (!NAME_PATTERN.test(name)) return 'Only letters, numbers, - and _';
    if (this.nodes().some(n => n.name.toLowerCase() === name.toLowerCase())) return 'Name already exists';
    return '';
  });

  canConfirm = computed(() => {
    const name = this.nameValue().trim();
    return name.length > 0 && !this.nameError();
  });

  inputClasses = computed(() => {
    const base = 'w-full px-3 py-1.5 text-sm bg-slate-800 border rounded text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-1';
    return this.nameError()
      ? `${base} border-red-500 focus:ring-red-500`
      : `${base} border-slate-600 focus:ring-blue-500`;
  });

  startNaming(type: NodeType) {
    if (this.pendingType() === type) {
      this.cancelNaming();
      return;
    }
    this.pendingType.set(type);
    this.nameCtrl.reset();
    afterNextRender(() => {
      const el = this.nameInputEl();
      if (el) el.nativeElement.focus();
    }, { injector: this.injector });
  }

  confirmName() {
    const name = this.nameCtrl.value.trim();
    const type = this.pendingType();
    if (!name || !type || this.nameError()) return;

    this.addNode.emit({ type, name });
    this.pendingType.set(null);
    this.nameCtrl.reset();
  }

  cancelNaming() {
    this.pendingType.set(null);
    this.nameCtrl.reset();
  }

  @HostListener('document:keydown', ['$event'])
  handleKeyboardEvent(event: KeyboardEvent) {
    const target = event.target as HTMLElement;
    if (['INPUT', 'TEXTAREA'].includes(target.tagName)) return;
    if (event.ctrlKey || event.metaKey || event.altKey) return;

    const key = event.key.toUpperCase();
    if (key === 'R') this.startNaming('router');
    if (key === 'S') this.startNaming('switch');
    if (key === 'U') this.startNaming('hub');
    if (key === 'H') this.startNaming('host');
  }
}
