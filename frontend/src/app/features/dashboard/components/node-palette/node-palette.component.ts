import { Component, output, input, signal, computed, HostListener, ElementRef, viewChild, inject, Injector, afterNextRender } from '@angular/core';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { toSignal } from '@angular/core/rxjs-interop';
import { Node } from '../../../../models/topology.model';

type NodeType = 'router' | 'host' | 'switch' | 'hub' | 'cloud' | 'linux' | 'server';

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
  expandedDescription = signal<NodeType | null>(null);
  nameCtrl = new FormControl('', { nonNullable: true });

  private nameValue = toSignal(this.nameCtrl.valueChanges, { initialValue: '' });

  nameInputEl = viewChild<ElementRef<HTMLInputElement>>('nameInputEl');

  readonly nodeTypeConfigs: { 
    id: NodeType; 
    label: string; 
    subtitle: string; 
    icon: string; 
    placeholder: string; 
    accentBorder: string; 
    hoverBorder: string;
    category: 'network' | 'end';
    shortcut: string;
    description: string;
  }[] = [
    { 
      id: 'router', 
      label: 'Router', 
      subtitle: 'Layer 3', 
      icon: 'router.svg', 
      placeholder: 'e.g. R1', 
      accentBorder: 'border-blue-500', 
      hoverBorder: 'hover:border-blue-500/50',
      category: 'network',
      shortcut: 'R',
      description: 'Layer 3 device with FRRouting support. Supports OSPF, BGP, IS-IS.'
    },
    { 
      id: 'switch', 
      label: 'Switch', 
      subtitle: 'Layer 2', 
      icon: 'switch.svg', 
      placeholder: 'e.g. SW1', 
      accentBorder: 'border-amber-500', 
      hoverBorder: 'hover:border-amber-500/50',
      category: 'network',
      shortcut: 'S',
      description: 'Layer 2 switch with MAC learning and broadcast domain support.'
    },
    { 
      id: 'hub', 
      label: 'Hub', 
      subtitle: 'Layer 1', 
      icon: 'hub.svg', 
      placeholder: 'e.g. HUB1', 
      accentBorder: 'border-violet-500', 
      hoverBorder: 'hover:border-violet-500/50',
      category: 'network',
      shortcut: 'U',
      description: 'Layer 1 repeater. Floods all traffic to all ports (no MAC learning).'
    },
    {
      id: 'host',
      label: 'Host',
      subtitle: 'Alpine Linux',
      icon: 'host.svg',
      placeholder: 'e.g. PC1',
      accentBorder: 'border-emerald-500',
      hoverBorder: 'hover:border-emerald-500/50',
      category: 'end',
      shortcut: 'H',
      description: 'Lightweight end device (Alpine Linux) with networking tools. Ideal for network labs.'
    },
    {
      id: 'cloud',
      label: 'Cloud',
      subtitle: 'Internet',
      icon: 'cloud.svg',
      placeholder: 'e.g. INET',
      accentBorder: 'border-sky-500',
      hoverBorder: 'hover:border-sky-500/50',
      category: 'network',
      shortcut: 'I',
      description: 'NAT gateway with automatic IP forwarding and masquerade. Connect your lab nodes to reach real internet.'
    },
    {
      id: 'linux',
      label: 'Linux',
      subtitle: 'Debian',
      icon: 'linux.svg',
      placeholder: 'e.g. VM1',
      accentBorder: 'border-red-500',
      hoverBorder: 'hover:border-red-500/50',
      category: 'end',
      shortcut: 'L',
      description: 'Debian Linux with bash, python3, cron and apt. Has direct internet access. Ideal for scripting and automation practice.'
    },
    {
      id: 'server',
      label: 'Server',
      subtitle: 'Debian + systemd',
      icon: 'server.svg',
      placeholder: 'e.g. SRV1',
      accentBorder: 'border-orange-500',
      hoverBorder: 'hover:border-orange-500/50',
      category: 'end',
      shortcut: 'V',
      description: 'Debian with systemd as PID 1. Manage services with systemctl. Includes nginx, apache2, mariadb, dnsmasq, chrony, vsftpd. Ideal for sysadmin and services labs.'
    },
  ];

  // Computed count of nodes by type
  nodeCounts = computed(() => {
    const counts: Record<NodeType, number> = { router: 0, switch: 0, hub: 0, host: 0, cloud: 0, linux: 0, server: 0 };
    this.nodes().forEach(node => {
      if (counts[node.type as NodeType] !== undefined) {
        counts[node.type as NodeType]++;
      }
    });
    return counts;
  });

  // Filtered node types by category
  networkDevices = computed(() => this.nodeTypeConfigs.filter(nt => nt.category === 'network'));
  endDevices = computed(() => this.nodeTypeConfigs.filter(nt => nt.category === 'end'));
  
  // Keyboard shortcuts for footer
  shortcuts = computed(() => this.nodeTypeConfigs.map(nt => ({ key: nt.shortcut, label: nt.label })));

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

  toggleDescription(type: NodeType) {
    this.expandedDescription.update(current => current === type ? null : type);
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
    if (key === 'I') this.startNaming('cloud');
    if (key === 'L') this.startNaming('linux');
    if (key === 'V') this.startNaming('server');
  }
}
