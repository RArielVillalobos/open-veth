import { Component, ElementRef, ViewChildren, QueryList, AfterViewInit, OnDestroy, input, output, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';

@Component({
  selector: 'app-terminal-panel',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './terminal-panel.component.html',
  styleUrl: './terminal-panel.component.scss'
})
export class TerminalPanelComponent implements AfterViewInit, OnDestroy {
  // Inputs: Lista de nodos que tienen terminal abierta
  activeNodes = input.required<string[]>();
  // Input: Cuál es la pestaña activa
  activeTab = input.required<string | null>();

  closeTerminal = output<string>();
  selectTab = output<string>();

  @ViewChildren('termContainer') termContainers!: QueryList<ElementRef>;

  private terminals = new Map<string, { term: Terminal, fit: FitAddon, socket: WebSocket }>();

  constructor() {
    // Reaccionar a cambios en la lista de nodos activos
    effect(() => {
      const nodes = this.activeNodes();
      // Esperar a que el DOM se actualice para inicializar nuevas terminales
      setTimeout(() => this.syncTerminals(nodes), 0);
    });

    // Reaccionar a cambio de pestaña para ajustar tamaño (fit)
    effect(() => {
        const currentTab = this.activeTab();
        if (currentTab && this.terminals.has(currentTab)) {
            setTimeout(() => {
                this.terminals.get(currentTab)?.fit.fit();
                this.terminals.get(currentTab)?.term.focus();
            }, 50);
        }
    });
  }

  ngAfterViewInit() {
    // Inicialización inicial si ya hay nodos
    this.syncTerminals(this.activeNodes());
  }

  ngOnDestroy() {
    this.terminals.forEach(t => {
      t.socket.close();
      t.term.dispose();
    });
    this.terminals.clear();
  }

  setActiveTab(nodeName: string) {
    this.selectTab.emit(nodeName);
  }

  clearActiveTerminal() {
    const active = this.activeTab();
    if (active && this.terminals.has(active)) {
      const { term } = this.terminals.get(active)!;
      term.clear();          // Borra el buffer de scrollback
      term.reset();          // Resetea el estado de la terminal
      term.focus();
    }
  }

  private syncTerminals(nodes: string[]) {
    // 1. Eliminar terminales cerradas
    for (const [name, instance] of this.terminals) {
      if (!nodes.includes(name)) {
        instance.socket.close();
        instance.term.dispose();
        this.terminals.delete(name);
      }
    }

    // 2. Crear nuevas terminales
    if (!this.termContainers) return;

    this.termContainers.forEach((el) => {
      const nodeName = el.nativeElement.getAttribute('data-node');
      if (nodes.includes(nodeName) && !this.terminals.has(nodeName)) {
        this.createTerminal(nodeName, el.nativeElement);
      }
    });
  }

  private createTerminal(nodeName: string, container: HTMLElement) {
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
    term.open(container);
    fitAddon.fit();

    // WebSocket Connection
    const wsUrl = `ws://localhost:8080/api/v1/terminal?node=${nodeName}`;
    const socket = new WebSocket(wsUrl);

    socket.onopen = () => {
      term.writeln(`\x1b[32m✔ Connected to ${nodeName}\x1b[0m`);
      fitAddon.fit();
    };

    socket.onmessage = (event) => {
      if (event.data instanceof Blob) {
        const reader = new FileReader();
        reader.onload = () => term.write(reader.result as string);
        reader.readAsText(event.data);
      } else {
        term.write(event.data);
      }
    };

    term.onData(data => {
      if (socket.readyState === WebSocket.OPEN) socket.send(data);
    });

    socket.onclose = () => term.writeln('\r\n\x1b[31m✖ Connection closed.\x1b[0m');

    this.terminals.set(nodeName, { term, fit: fitAddon, socket });
  }
}
