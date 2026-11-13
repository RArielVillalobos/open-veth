import { Component, ElementRef, ViewChildren, QueryList, AfterViewInit, OnDestroy, effect, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { SearchAddon } from 'xterm-addon-search';
import { ToastService } from '../../../core/services/toast.service';
import { UIStore } from '../../../state/ui.store';
import { TerminalStore } from '../../../state/terminal.store';
import { TopologyStore } from '../../../state/topology.store';
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
  private readonly topologyStore = inject(TopologyStore);

  @ViewChildren('termContainer') termContainers!: QueryList<ElementRef>;

  readonly connectionStates = signal<Record<string, string>>({});
  readonly fontSize = signal(13);
  readonly searchOpen = signal(false);
  readonly searchQuery = signal('');
  readonly searchResult = signal('');
  private readonly toast = inject(ToastService);
  private destroyed = false;

  private terminals = new Map<string, { term: Terminal, fit: FitAddon, search: SearchAddon, socket: WebSocket | null, resizeObserver: ResizeObserver, refreshTimer: ReturnType<typeof setTimeout> | null }>();

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
                this.fitTerminal(currentTabId);
                this.terminals.get(currentTabId)?.term.focus();
            }, 50);
        }
    });
  }

  selectAdjacentTab(offset: number) {
    const sessions = this.terminalStore.sessions();
    if (!sessions.length) return;
    const current = sessions.findIndex(session => session.nodeId === this.terminalStore.activeNodeId());
    const next = sessions[(current + offset + sessions.length) % sessions.length];
    this.setActiveTab(next.nodeId);
    document.getElementById('terminal-tab-' + next.nodeId)?.focus();
  }

  ngAfterViewInit() {
    // Inicialización inicial si ya hay nodos
    this.syncTerminals(this.terminalStore.sessions());
  }

  ngOnDestroy() {
    this.destroyed = true;
    this.terminals.forEach(t => {
      if (t.refreshTimer) clearTimeout(t.refreshTimer);
      t.resizeObserver.disconnect();
      t.socket?.close();
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

      term.focus();
    }
  }

  fitAll() {
    this.terminals.forEach(({ fit, socket, term }) => {
      fit.fit();
      if (socket?.readyState === WebSocket.OPEN && term.cols > 0) {
        socket.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
      }
    });
  }

  private syncTerminals(sessions: TerminalSession[]) {
    if (this.destroyed) return;
    const activeIds = sessions.map(s => s.nodeId);

    // 1. Eliminar terminales cerradas
    for (const [id, instance] of this.terminals) {
      if (!activeIds.includes(id)) {
        if (instance.refreshTimer) clearTimeout(instance.refreshTimer);
        instance.resizeObserver.disconnect();
        instance.socket?.close();
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
        background: '#0d1416',
        foreground: '#e8efef',
        cursor: '#83b9df',
        cursorAccent: '#0d1416',
        selectionBackground: 'rgba(131, 185, 223, 0.25)',
        black: '#131b1e',
        red: '#d97b68',
        green: '#8eaf9f',
        yellow: '#d7a153',
        blue: '#648eac',
        magenta: '#717ed0',
        cyan: '#83b9df',
        white: '#8fa3a6',
        brightBlack: '#5c6d70',
        brightRed: '#e2917f',
        brightGreen: '#a0b9a5',
        brightYellow: '#e3b471',
        brightBlue: '#76a4c5',
        brightMagenta: '#9aa3e0',
        brightCyan: '#9fe0d9',
        brightWhite: '#e8efef',
      },
      fontFamily: '"IBM Plex Mono", Menlo, Monaco, "Courier New", monospace',
      fontSize: this.fontSize(),
      convertEol: true, // Importante para saltos de línea correctos
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    
    const search = new SearchAddon();
    term.loadAddon(search);
    term.open(container);
    term.attachCustomKeyEventHandler(event => {
      if (event.type !== 'keydown' || !event.ctrlKey || !event.shiftKey) return true;
      if (event.code === 'KeyF') { event.preventDefault(); this.openSearch(); return false; }
      if (event.code === 'KeyC') { event.preventDefault(); void this.copySelection(); return false; }
      if (event.code === 'KeyV') { event.preventDefault(); void this.pasteClipboard(); return false; }
      return true;
    });
    const resizeObserver = new ResizeObserver(() => this.fitTerminal(nodeId));
    this.terminals.set(nodeId, { term, fit: fitAddon, search, socket: null, resizeObserver, refreshTimer: null });
    resizeObserver.observe(container);
    term.onData(data => {
      const entry = this.terminals.get(nodeId);
      if (entry?.socket?.readyState !== WebSocket.OPEN) return;
      entry.socket.send(data);
      if (data.includes('\r')) {
        if (entry.refreshTimer) clearTimeout(entry.refreshTimer);
        entry.refreshTimer = setTimeout(() => this.topologyStore.fetchNodeInterfaces(nodeId), 1500);
      }
    });
    this.connect(nodeId);
  }

  private fitTerminal(nodeId: string) {
    const entry = this.terminals.get(nodeId);
    if (!entry || nodeId !== this.terminalStore.activeNodeId()) return;
    entry.fit.fit();
    if (entry.socket?.readyState === WebSocket.OPEN && entry.term.cols > 0) {
      entry.socket.send(JSON.stringify({ type: 'resize', cols: entry.term.cols, rows: entry.term.rows }));
    }
  }

  private connect(nodeId: string) {
    const entry = this.terminals.get(nodeId);
    if (!entry || this.destroyed) return;
    entry.socket?.close();
    this.connectionStates.update(states => ({ ...states, [nodeId]: 'Connecting' }));
    const socket = new WebSocket(environment.apiUrl.replace(/^http/, 'ws') + '/terminal?node=' + encodeURIComponent(nodeId));
    entry.socket = socket;
    socket.binaryType = 'arraybuffer';
    const current = () => !this.destroyed && this.terminals.get(nodeId) === entry && entry.socket === socket;
    socket.onopen = () => {
      if (!current()) return;
      this.connectionStates.update(states => ({ ...states, [nodeId]: 'Connected' }));
      this.fitTerminal(nodeId);
      if (nodeId === this.terminalStore.activeNodeId()) entry.term.focus();
    };
    socket.onmessage = event => {
      if (current()) entry.term.write(event.data instanceof ArrayBuffer ? new Uint8Array(event.data) : event.data);
    };
    const disconnected = () => {
      if (!current()) return;
      this.connectionStates.update(states => ({ ...states, [nodeId]: 'Disconnected' }));
    };
    socket.onclose = disconnected;
    socket.onerror = disconnected;
  }

  reconnect() {
    const id = this.terminalStore.activeNodeId();
    if (id && this.connectionStates()[id] === 'Disconnected') this.connect(id);
  }

  changeFontSize(delta: number) {
    this.fontSize.update(size => Math.max(10, Math.min(22, size + delta)));
    this.terminals.forEach(entry => entry.term.options.fontSize = this.fontSize());
    const id = this.terminalStore.activeNodeId();
    if (id) this.fitTerminal(id);
  }

  openSearch() {
    this.searchOpen.set(true);
    setTimeout(() => document.getElementById('terminal-search')?.focus(), 0);
  }

  closeSearch() {
    this.searchOpen.set(false);
    this.searchResult.set('');
    const id = this.terminalStore.activeNodeId();
    if (id) this.terminals.get(id)?.term.focus();
  }

  find(previous = false) {
    const id = this.terminalStore.activeNodeId();
    const query = this.searchQuery();
    const entry = id ? this.terminals.get(id) : undefined;
    if (!entry || !query) { this.searchResult.set(''); return; }
    const found = previous ? entry.search.findPrevious(query) : entry.search.findNext(query);
    this.searchResult.set(found ? 'Match found' : 'No matches');
  }

  async copySelection() {
    const id = this.terminalStore.activeNodeId();
    const selection = id ? this.terminals.get(id)?.term.getSelection() : '';
    if (!selection) { this.toast.info('Select terminal text to copy.'); return; }
    try { await navigator.clipboard.writeText(selection); }
    catch { this.toast.error('Clipboard unavailable. Use the browser copy command.'); }
  }

  async pasteClipboard() {
    const id = this.terminalStore.activeNodeId();
    const entry = id ? this.terminals.get(id) : undefined;
    if (!entry || entry.socket?.readyState !== WebSocket.OPEN) return;
    try {
      const text = await navigator.clipboard.readText();
      if (this.terminalStore.activeNodeId() !== id || this.terminals.get(id!) !== entry || entry.socket?.readyState !== WebSocket.OPEN) return;
      entry.term.paste(text);
      entry.term.focus();
    } catch { this.toast.error('Clipboard unavailable. Use the browser paste command.'); }
  }
}
