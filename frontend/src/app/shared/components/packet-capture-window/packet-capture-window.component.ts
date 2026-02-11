import { Component, Input, Output, EventEmitter, OnInit, OnDestroy, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DragDropModule } from '@angular/cdk/drag-drop';
import { CaptureService, PacketSummary } from '../../../core/services/capture.service';
import { ToastService } from '../../../core/services/toast.service';
import { Subscription } from 'rxjs';

@Component({
  selector: 'app-packet-capture-window',
  standalone: true,
  imports: [CommonModule, DragDropModule],
  templateUrl: './packet-capture-window.component.html',
  styleUrl: './packet-capture-window.component.scss'
})
export class PacketCaptureWindowComponent implements OnInit, OnDestroy {
  @Input() nodeId!: string;
  @Input() nodeName!: string;
  @Input() interfaceName!: string;
  @Output() onClose = new EventEmitter<void>();

  private captureService = inject(CaptureService);
  private toast = inject(ToastService);
  private packetSub?: Subscription;
  private connectedSub?: Subscription;

  packets: PacketSummary[] = [];

  // UI State
  isPaused = false;
  filterTerm = '';
  activeProtocolFilter = 'ALL';
  selectedPacket: PacketSummary | null = null;
  isConnected = false;

  ngOnInit(): void {
    this.captureService.startCapture(this.nodeId, this.interfaceName);
    this.connectedSub = this.captureService.connected$.subscribe(
      connected => this.isConnected = connected
    );
    this.packetSub = this.captureService.packets$.subscribe(packet => {
      if (!this.isPaused) {
        this.packets.push(packet);
        if (this.packets.length > 500) {
          this.packets.shift();
        }
      }
    });
  }

  get filteredPackets() {
    let result = this.packets;

    // 1. Apply Protocol Chip Filter
    if (this.activeProtocolFilter !== 'ALL') {
      if (this.activeProtocolFilter === 'IPv6') {
        result = result.filter(p => p.protocol.includes('ICMPv6'));
      } else {
        result = result.filter(p => p.protocol === this.activeProtocolFilter);
      }
    }

    // 2. Apply Text Search
    if (this.filterTerm) {
      const term = this.filterTerm.toLowerCase();
      result = result.filter(p => 
        p.source.toLowerCase().includes(term) ||
        p.destination.toLowerCase().includes(term) ||
        p.info.toLowerCase().includes(term)
      );
    }

    return result;
  }

  setProtocolFilter(proto: string) {
    this.activeProtocolFilter = proto;
  }

  selectPacket(packet: PacketSummary) {
    this.selectedPacket = this.selectedPacket === packet ? null : packet;
  }

  getProtocolFilterClass(proto: string): string {
    const isActive = this.activeProtocolFilter === proto;
    const baseClasses = 'transition-all duration-200 ';
    
    switch(proto) {
      case 'ALL':
        return baseClasses + (isActive ? 'bg-slate-600 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200');
      case 'TCP':
        return baseClasses + (isActive ? 'bg-blue-600 text-white shadow-sm' : 'text-blue-400/70 hover:text-blue-400');
      case 'UDP':
        return baseClasses + (isActive ? 'bg-emerald-600 text-white shadow-sm' : 'text-emerald-400/70 hover:text-emerald-400');
      case 'ICMP':
        return baseClasses + (isActive ? 'bg-amber-600 text-white shadow-sm' : 'text-amber-400/70 hover:text-amber-400');
      case 'ARP':
        return baseClasses + (isActive ? 'bg-purple-600 text-white shadow-sm' : 'text-purple-400/70 hover:text-purple-400');
      default:
        return baseClasses + (isActive ? 'bg-slate-600 text-white' : 'text-slate-500 hover:text-slate-300');
    }
  }

  getProtocolBadgeClass(protocol: string): string {
    switch(protocol) {
      case 'TCP':
        return 'bg-blue-500/20 text-blue-300 border border-blue-500/30';
      case 'UDP':
        return 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30';
      case 'ICMP':
      case 'ICMPv6':
        return 'bg-amber-500/20 text-amber-300 border border-amber-500/30';
      case 'ARP':
        return 'bg-purple-500/20 text-purple-300 border border-purple-500/30';
      default:
        return 'bg-slate-700 text-slate-400 border border-slate-600';
    }
  }

  togglePause() {
    this.isPaused = !this.isPaused;
  }

  getRowClass(p: PacketSummary): string {
    const info = p.info.toLowerCase();
    
    // ICMP
    if (p.protocol === 'ICMP' || p.protocol === 'ICMPv6') {
      if (info.includes('request') || info.includes('solicitation')) return 'bg-blue-900/20 hover:bg-blue-900/30 text-blue-100';
      if (info.includes('reply') || info.includes('advertisement')) return 'bg-green-900/20 hover:bg-green-900/30 text-green-100';
      if (info.includes('unreachable') || info.includes('exceeded')) return 'bg-red-900/20 hover:bg-red-900/30 text-red-200';
    }

    // ARP
    if (p.protocol === 'ARP') return 'bg-amber-900/10 hover:bg-amber-900/20 text-amber-100';

    // TCP SYN/FIN
    if (p.protocol === 'TCP') {
        if (info.includes('[s]') || info.includes('[f]')) return 'bg-purple-900/20 hover:bg-purple-900/30';
    }

    // Default
    return 'border-b border-slate-800/50 hover:bg-slate-800/30';
  }

  ngOnDestroy(): void {
    this.packetSub?.unsubscribe();
    this.connectedSub?.unsubscribe();
    this.captureService.stopCapture();
  }

  clearPackets(): void {
    this.packets = [];
  }

  exportToPcap(): void {
    // TODO: Implement full PCAP export
    // This will require backend changes to send raw packet bytes
    this.toast.info(`Export to PCAP coming soon! (${this.packets.length} packets ready)`);
  }

  clearFilters(): void {
    this.activeProtocolFilter = 'ALL';
    this.filterTerm = '';
  }

  trackByPacket(index: number, packet: PacketSummary): string {
    // Create a truly unique ID for each packet
    return `${packet.timestamp}-${packet.source}-${packet.destination}-${packet.protocol}-${packet.length}-${index}`;
  }

  // Check if there are active filters that might be hiding packets
  get hasActiveFilters(): boolean {
    return this.activeProtocolFilter !== 'ALL' || this.filterTerm !== '';
  }

  // Get count of visible packets vs total
  get visiblePacketCount(): number {
    return this.filteredPackets.length;
  }
}
