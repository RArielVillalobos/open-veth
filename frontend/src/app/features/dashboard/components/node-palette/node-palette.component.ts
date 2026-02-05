import { Component, output, HostListener } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-node-palette',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="h-full bg-slate-900 border-r border-slate-700 flex flex-col p-4 w-64">
      <h2 class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-4">Nodes</h2>

      <div class="flex flex-col gap-2">

        <!-- Router -->
        <button (click)="addNode.emit('router')"
                class="flex items-center gap-3 p-3 rounded-lg bg-slate-800/50 hover:bg-slate-800 transition-all border border-slate-700/50 hover:border-blue-500/50 group">
          <img src="assets/icons/router.svg" alt="Router" class="w-12 h-12 flex-shrink-0">
          <div class="flex flex-col items-start">
            <span class="text-sm font-medium text-slate-200 group-hover:text-white">Router</span>
            <span class="text-[11px] text-slate-500">Layer 3</span>
          </div>
        </button>

        <!-- Switch -->
        <button (click)="addNode.emit('switch')"
                class="flex items-center gap-3 p-3 rounded-lg bg-slate-800/50 hover:bg-slate-800 transition-all border border-slate-700/50 hover:border-amber-500/50 group">
          <img src="assets/icons/switch.svg" alt="Switch" class="w-12 h-12 flex-shrink-0">
          <div class="flex flex-col items-start">
            <span class="text-sm font-medium text-slate-200 group-hover:text-white">Switch</span>
            <span class="text-[11px] text-slate-500">Layer 2</span>
          </div>
        </button>

        <!-- Host -->
        <button (click)="addNode.emit('host')"
                class="flex items-center gap-3 p-3 rounded-lg bg-slate-800/50 hover:bg-slate-800 transition-all border border-slate-700/50 hover:border-emerald-500/50 group">
          <img src="assets/icons/host.svg" alt="Host" class="w-12 h-12 flex-shrink-0">
          <div class="flex flex-col items-start">
            <span class="text-sm font-medium text-slate-200 group-hover:text-white">Host</span>
            <span class="text-[11px] text-slate-500">End device</span>
          </div>
        </button>
      </div>

      <div class="mt-auto pt-4">
        <p class="text-[10px] text-slate-600 text-center">
          Press <kbd class="px-1 py-0.5 bg-slate-800 rounded text-slate-500">R</kbd>
          <kbd class="px-1 py-0.5 bg-slate-800 rounded text-slate-500 mx-0.5">S</kbd>
          <kbd class="px-1 py-0.5 bg-slate-800 rounded text-slate-500">H</kbd>
        </p>
      </div>
    </div>
  `
})
export class NodePaletteComponent {
  addNode = output<'router' | 'host' | 'switch'>();

  @HostListener('document:keydown', ['$event'])
  handleKeyboardEvent(event: KeyboardEvent) {
    const target = event.target as HTMLElement;
    if (['INPUT', 'TEXTAREA'].includes(target.tagName)) return;
    if (event.ctrlKey || event.metaKey || event.altKey) return;

    const key = event.key.toUpperCase();
    if (key === 'R') this.addNode.emit('router');
    if (key === 'S') this.addNode.emit('switch');
    if (key === 'H') this.addNode.emit('host');
  }
}
