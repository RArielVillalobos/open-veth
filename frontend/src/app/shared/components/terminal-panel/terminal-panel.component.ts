import { Component, ElementRef, ViewChildren, QueryList, AfterViewInit, OnDestroy, effect, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { UIStore } from '../../../state/ui.store';
import { TerminalStore } from '../../../state/terminal.store';
import { environment } from '../../../../environments/environment';
import { TerminalSession } from '../../../models/terminal.model';

@Component({
  selector: 'app-terminal-panel',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './terminal-panel.component.html',
  styleUrl: './terminal-panel.component.scss'
})
export class TerminalPanelComponent implements AfterViewInit, OnDestroy {
  readonly ui = inject(UIStore);
  readonly terminalStore = inject(TerminalStore);

  @ViewChildren('termContainer') termContainers!: QueryList<ElementRef>;

  private terminals = new Map<string, { term: Terminal, fit: FitAddon, socket: WebSocket }>();

  constructor() {
    // Reaccionar a cambios en la lista de nodos activos
    effect(() => {
      const sessions = this.terminalStore.sessions();
      // Esperar a que el DOM se actualice para inicializar nuevas terminales
      setTimeout(() => this.syncTerminals(sessions), 0);
    });

    // Reaccionar a cambio de pestaña para ajustar tamaño (fit)
    effect(() => {
        const currentTabId = this.terminalStore.activeNodeId();
        if (currentTabId && this.terminals.has(currentTabId)) {
            setTimeout(() => {
                this.terminals.get(currentTabId)?.fit.fit();
                this.terminals.get(currentTabId)?.term.focus();
            }, 50);
        }
    });
  }

  ngAfterViewInit() {
    // Inicialización inicial si ya hay nodos
    this.syncTerminals(this.terminalStore.sessions());
  }

  ngOnDestroy() {
    this.terminals.forEach(t => {
      t.socket.close();
      t.term.dispose();
    });
    this.terminals.clear();
  }

  setActiveTab(nodeId: string) {
    this.terminalStore.setActive(nodeId);
  }

  closeTerminal(nodeId: string) {
    this.terminalStore.closeTerminal(nodeId);
  }

  clearActiveTerminal() {
    const activeId = this.terminalStore.activeNodeId();
    if (activeId && this.terminals.has(activeId)) {
      const { term } = this.terminals.get(activeId)!;
      term.clear();          // Borra el buffer de scrollback
      term.reset();          // Resetea el estado de la terminal
      term.focus();
    }
  }

  private syncTerminals(sessions: TerminalSession[]) {
    const activeIds = sessions.map(s => s.nodeId);

    // 1. Eliminar terminales cerradas
    for (const [id, instance] of this.terminals) {
      if (!activeIds.includes(id)) {
        instance.socket.close();
        instance.term.dispose();
        this.terminals.delete(id);
      }
    }

    // 2. Crear nuevas terminales
    if (!this.termContainers) return;

    this.termContainers.forEach((el) => {
      const nodeId = el.nativeElement.getAttribute('data-node-id');
      if (activeIds.includes(nodeId) && !this.terminals.has(nodeId)) {
        const session = sessions.find(s => s.nodeId === nodeId);
        if (session) {
          this.createTerminal(session.nodeId, session.nodeName, el.nativeElement);
        }
      }
    });
  }

  private createTerminal(nodeId: string, nodeName: string, container: HTMLElement) {
    const term = new Terminal({
      cursorBlink: true,
      theme: {
        background: '#0f172a', // Slate-900 (más oscuro para diferenciar)
        foreground: '#f8fafc',
        cursor: '#3b82f6',
        selectionBackground: 'rgba(59, 130, 246, 0.3)'
      },
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      fontSize: 12,
      convertEol: true, // Importante para saltos de línea correctos
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    
    // Safety: only open and fit if container is available and has size
    setTimeout(() => {
        if (container.clientWidth > 0) {
            term.open(container);
            fitAddon.fit();
        } else {
            // Fallback: wait a bit more if still zero (e.g. during animations)
            setTimeout(() => {
                term.open(container);
                if (container.clientWidth > 0) fitAddon.fit();
            }, 100);
        }
    }, 0);

    // WebSocket Connection
    // Calculate WebSocket URL from API URL (handling http->ws and https->wss)
    const baseWsUrl = environment.apiUrl.replace(/^http/, 'ws');
    const wsUrl = `${baseWsUrl}/terminal?node=${nodeId}`;
    const socket = new WebSocket(wsUrl);
    socket.binaryType = 'arraybuffer';

    socket.onopen = () => {
      term.writeln(`\x1b[32m✔ Connected to ${nodeName}\x1b[0m`);
      fitAddon.fit();
    };

    socket.onmessage = (event) => {
      if (event.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(event.data));
      } else {
        term.write(event.data);
      }
    };

    term.onData(data => {
      if (socket.readyState === WebSocket.OPEN) socket.send(data);
    });

    socket.onclose = () => term.writeln('\r\n\x1b[31m✖ Connection closed.\x1b[0m');

    this.terminals.set(nodeId, { term, fit: fitAddon, socket });
  }
}
