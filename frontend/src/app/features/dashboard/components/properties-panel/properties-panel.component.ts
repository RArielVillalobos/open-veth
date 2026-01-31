import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Node, Link } from '../../../../models/topology.model';

@Component({
  selector: 'app-properties-panel',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="h-full bg-slate-900 border-l border-slate-700 flex flex-col w-72">
      <div class="p-4 border-b border-slate-800 flex justify-between items-center">
        <h2 class="text-xs font-bold text-slate-400 uppercase tracking-wider">
          @if (selectedNode()) { Node Properties } 
          @else if (selectedLink()) { Link Properties }
          @else { Properties }
        </h2>
        @if (selectedNode() || selectedLink()) {
          <button (click)="close.emit()" class="text-slate-500 hover:text-white transition-colors text-lg">
            ✕
          </button>
        }
      </div>

      <!-- NODE DETAILS -->
      @if (selectedNode()) {
        <div class="p-4 flex flex-col gap-6">
          
          <!-- Header del Nodo -->
          <div class="flex items-center gap-3">
            <div class="w-12 h-12 rounded flex items-center justify-center text-xl font-bold"
                 [ngClass]="{
                   'bg-blue-500/20 text-blue-400': selectedNode()?.type === 'router',
                   'bg-green-500/20 text-green-400': selectedNode()?.type === 'host',
                   'bg-amber-500/20 text-amber-400': selectedNode()?.type === 'switch'
                 }">
              {{ selectedNode()?.type?.[0]?.toUpperCase() }}
            </div>
            <div>
              <h3 class="font-bold text-slate-100">{{ selectedNode()?.name }}</h3>
              <p class="text-xs text-slate-500 font-mono">{{ selectedNode()?.id }}</p>
            </div>
          </div>

          <!-- Acciones Rápidas -->
          <div class="grid grid-cols-2 gap-2">
            <button (click)="openTerminal.emit(selectedNode()?.name!)"
                    class="bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs py-2 px-3 rounded border border-slate-700 flex items-center justify-center gap-2">
              <span>Terminal</span>
            </button>
            <button (click)="deleteNode.emit(selectedNode()?.id!)"
                    class="bg-red-900/20 hover:bg-red-900/40 text-red-400 text-xs py-2 px-3 rounded border border-red-900/30 flex items-center justify-center gap-2">
              <span>Delete</span>
            </button>
          </div>

          <!-- Detalles Técnicos -->
          <div class="space-y-4">
            <div>
              <label class="text-xs text-slate-500 block mb-1">Image</label>
              <input type="text" [value]="selectedNode()?.image" disabled
                     class="w-full bg-slate-950 border border-slate-700 rounded px-2 py-1 text-xs text-slate-400 font-mono">
            </div>
            
            <div>
              <label class="text-xs text-slate-500 block mb-1">Status</label>
              <div class="flex items-center gap-2">
                <div class="w-2 h-2 rounded-full bg-green-500"></div>
                <span class="text-xs text-green-400">Running</span>
              </div>
            </div>

            <!-- Interfaces (Live) -->
            <div>
              <label class="text-xs text-slate-500 block mb-2">Network Interfaces</label>
              @if (selectedNode()?.interfaces?.length) {
                <div class="bg-slate-950 rounded border border-slate-700 overflow-hidden">
                  <table class="w-full text-xs text-left">
                    <tbody class="divide-y divide-slate-800">
                      @for (iface of selectedNode()?.interfaces; track iface.ifname) {
                        @if (iface.ifname !== 'lo') {
                          <tr>
                            <td class="p-2 font-mono text-slate-400 border-r border-slate-800 w-16">{{ iface.ifname }}</td>
                            <td class="p-2 text-slate-300">
                              @let ipv4 = getIPv4(iface.addr_info);
                              @if (ipv4) {
                                {{ ipv4.local }}/{{ ipv4.prefixlen }}
                              } @else {
                                <span class="text-slate-600 italic">No IP</span>
                              }
                            </td>
                          </tr>
                        }
                      }
                    </tbody>
                  </table>
                </div>
              } @else {
                <div class="text-xs text-slate-600 italic">Loading interfaces...</div>
              }
            </div>

          </div>
        </div>
      } 

      <!-- LINK DETAILS -->
      @if (selectedLink()) {
        <div class="p-4 flex flex-col gap-6">
          <div class="flex items-center gap-3">
            <div class="w-12 h-12 rounded bg-slate-800 border border-slate-700 flex items-center justify-center text-slate-400 text-xl font-bold">
              ⚡
            </div>
            <div>
              <h3 class="font-bold text-slate-100">Virtual Link</h3>
              <p class="text-xs text-slate-500 font-mono">{{ selectedLink()?.id }}</p>
            </div>
          </div>

          <div class="space-y-4">
            <div class="p-3 bg-slate-950 rounded border border-slate-800">
              <div class="flex justify-between items-center mb-2">
                <span class="text-xs font-bold text-slate-400">Endpoint A</span>
                <span class="text-xs font-mono text-blue-400">{{ selectedLink()?.source_int }}</span>
              </div>
              <div class="text-sm text-slate-200">{{ selectedLink()?.source }}</div>
            </div>

            <div class="flex justify-center text-slate-600">⬇️</div>

            <div class="p-3 bg-slate-950 rounded border border-slate-800">
              <div class="flex justify-between items-center mb-2">
                <span class="text-xs font-bold text-slate-400">Endpoint B</span>
                <span class="text-xs font-mono text-blue-400">{{ selectedLink()?.target_int }}</span>
              </div>
              <div class="text-sm text-slate-200">{{ selectedLink()?.target }}</div>
            </div>
          </div>
          
          <button (click)="deleteLink.emit(selectedLink()?.id!)"
                  class="w-full mt-4 bg-red-900/20 hover:bg-red-900/40 text-red-400 text-xs py-2 px-3 rounded border border-red-900/30 flex items-center justify-center gap-2">
             <span>Delete Link</span>
          </button>
        </div>
      }

      @if (!selectedNode() && !selectedLink()) {
        <div class="flex-1 flex flex-col items-center justify-center text-slate-600 p-8 text-center">
          <div class="text-4xl mb-2 opacity-20">🖱️</div>
          <p class="text-sm">Select a node or link to view its properties</p>
        </div>
      }
    </div>
  `
})
export class PropertiesPanelComponent {
  selectedNode = input<Node | null>(null);
  selectedLink = input<Link | null>(null);
  openTerminal = output<string>();
  deleteNode = output<string>();
  deleteLink = output<string>();
  close = output<void>();

  getIPv4(addrInfo: any[] | undefined) {
    if (!addrInfo) return null;
    return addrInfo.find(addr => !addr.local.includes(':'));
  }
}
