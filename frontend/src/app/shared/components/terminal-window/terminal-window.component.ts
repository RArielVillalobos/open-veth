import { Component, ElementRef, ViewChild, AfterViewInit, OnDestroy, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';

@Component({
  selector: 'app-terminal-window',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './terminal-window.component.html',
  styleUrl: './terminal-window.component.scss'
})
export class TerminalWindowComponent implements AfterViewInit, OnDestroy {
  nodeName = input.required<string>();
  close = output<void>();

  @ViewChild('terminal') terminalDiv!: ElementRef;

  private term!: Terminal;
  private fitAddon!: FitAddon;
  private socket!: WebSocket;

  ngAfterViewInit() {
    this.initTerminal();
    this.connectWebSocket();
  }

  ngOnDestroy() {
    if (this.socket) {
      this.socket.close();
    }
    if (this.term) {
      this.term.dispose();
    }
  }

  private initTerminal() {
    this.term = new Terminal({
      cursorBlink: true,
      theme: {
        background: '#1e293b', // Slate-800
        foreground: '#f8fafc',
        cursor: '#3b82f6'
      },
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      fontSize: 13
    });

    this.fitAddon = new FitAddon();
    this.term.loadAddon(this.fitAddon);
    this.term.open(this.terminalDiv.nativeElement);
    this.fitAddon.fit();

    // Enviar teclas al socket
    this.term.onData(data => {
      if (this.socket && this.socket.readyState === WebSocket.OPEN) {
        this.socket.send(data);
      }
    });
  }

  private connectWebSocket() {
    const wsUrl = `ws://localhost:8080/api/v1/terminal?node=${this.nodeName()}`;
    this.socket = new WebSocket(wsUrl);

    this.socket.onopen = () => {
      this.term.writeln('\x1b[32m✔ Connected to ' + this.nodeName() + '\x1b[0m');
      this.fitAddon.fit();
    };

    this.socket.onmessage = (event) => {
      // Recibimos datos crudos del pty
      // Usamos FileReader si viene como Blob (común en WS binarios)
      if (event.data instanceof Blob) {
        const reader = new FileReader();
        reader.onload = () => {
          this.term.write(reader.result as string);
        };
        reader.readAsText(event.data);
      } else {
        this.term.write(event.data);
      }
    };

    this.socket.onclose = () => {
      this.term.writeln('\r\n\x1b[31m✖ Connection closed.\x1b[0m');
    };

    this.socket.onerror = (err) => {
      this.term.writeln('\r\n\x1b[31m✖ Connection error.\x1b[0m');
      console.error('WS Error', err);
    };
  }
}
