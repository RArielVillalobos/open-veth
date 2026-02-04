import { Component, output, HostListener } from '@angular/core';
import { CommonModule } from '@angular/common';

interface NodeType {
  id: 'router' | 'host' | 'switch';
  name: string;
  subtitle: string;
  hotkey: string;
  iconClass: string;
  accentClass: string;
}

@Component({
  selector: 'app-node-palette',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './node-palette.component.html',
  styleUrl: './node-palette.component.scss'
})
export class NodePaletteComponent {
  addNode = output<'router' | 'host' | 'switch'>();

  nodeTypes: NodeType[] = [
    {
      id: 'router',
      name: 'Router',
      subtitle: 'Layer 3',
      hotkey: 'R',
      iconClass: 'rounded-full',
      accentClass: 'border-blue-500 bg-blue-500/10 text-blue-400'
    },
    {
      id: 'switch',
      name: 'Switch',
      subtitle: 'Layer 2',
      hotkey: 'S',
      iconClass: 'rounded',
      accentClass: 'border-amber-500 bg-amber-500/10 text-amber-400'
    },
    {
      id: 'host',
      name: 'Host',
      subtitle: 'End device',
      hotkey: 'H',
      iconClass: 'rounded-md',
      accentClass: 'border-emerald-500 bg-emerald-500/10 text-emerald-400'
    }
  ];

  @HostListener('document:keydown', ['$event'])
  handleKeyboardEvent(event: KeyboardEvent) {
    const target = event.target as HTMLElement;
    if (['INPUT', 'TEXTAREA'].includes(target.tagName)) return;
    if (event.ctrlKey || event.metaKey || event.altKey) return;

    const key = event.key.toUpperCase();
    const node = this.nodeTypes.find(n => n.hotkey === key);

    if (node) {
      this.addNode.emit(node.id);
    }
  }
}
