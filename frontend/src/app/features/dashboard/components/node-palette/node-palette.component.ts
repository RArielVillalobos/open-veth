import { Component, output } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-node-palette',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="h-full bg-slate-900 border-r border-slate-700 flex flex-col p-4 w-64">
      <h2 class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-4">Library</h2>
      
      <div class="flex flex-col gap-3">
        <!-- Router -->
        <button (click)="addNode.emit('router')" 
                class="flex items-center gap-3 p-3 rounded bg-slate-800 hover:bg-slate-700 hover:text-white transition-colors border border-slate-700 hover:border-blue-500 group">
          <div class="w-8 h-8 rounded-full bg-blue-500/10 border border-blue-500/50 flex items-center justify-center text-blue-400 group-hover:bg-blue-500 group-hover:text-white transition-colors">
            R
          </div>
          <div class="flex flex-col items-start">
            <span class="text-sm font-medium text-slate-200">Router</span>
            <span class="text-xs text-slate-500">FRRouting</span>
          </div>
        </button>

        <!-- Switch -->
        <button (click)="addNode.emit('switch')" 
                class="flex items-center gap-3 p-3 rounded bg-slate-800 hover:bg-slate-700 hover:text-white transition-colors border border-slate-700 hover:border-amber-500 group">
          <div class="w-8 h-8 rounded bg-amber-500/10 border border-amber-500/50 flex items-center justify-center text-amber-400 group-hover:bg-amber-500 group-hover:text-white transition-colors">
            S
          </div>
          <div class="flex flex-col items-start">
            <span class="text-sm font-medium text-slate-200">Switch</span>
            <span class="text-xs text-slate-500">L2 Bridge</span>
          </div>
        </button>

        <!-- Host -->
        <button (click)="addNode.emit('host')" 
                class="flex items-center gap-3 p-3 rounded bg-slate-800 hover:bg-slate-700 hover:text-white transition-colors border border-slate-700 hover:border-green-500 group">
          <div class="w-8 h-8 rounded bg-green-500/10 border border-green-500/50 flex items-center justify-center text-green-400 group-hover:bg-green-500 group-hover:text-white transition-colors">
            H
          </div>
          <div class="flex flex-col items-start">
            <span class="text-sm font-medium text-slate-200">PC Host</span>
            <span class="text-xs text-slate-500">Alpine Linux</span>
          </div>
        </button>
      </div>

      <div class="mt-auto">
        <div class="text-xs text-slate-600 text-center py-4 border-t border-slate-800">
          OpenVeth v0.4
        </div>
      </div>
    </div>
  `
})
export class NodePaletteComponent {
  addNode = output<'router' | 'host' | 'switch'>();
}
