import { Component, Input, Output, EventEmitter, OnInit, OnDestroy, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DragDropModule } from '@angular/cdk/drag-drop';
import { CaptureService, PacketSummary } from '../../../core/services/capture.service';
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
  private subscription?: Subscription;
  
  packets: PacketSummary[] = [];
  
  // UI State
  isPaused = false;
  filterTerm = '';
  activeProtocolFilter = 'ALL'; // New state for chips

  ngOnInit(): void {
    this.captureService.startCapture(this.nodeId, this.interfaceName);
    this.subscription = this.captureService.packets$.subscribe(packet => {
      // Only append if not paused
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
    this.subscription?.unsubscribe();
    this.captureService.stopCapture();
  }

  clearPackets(): void {
    this.packets = [];
  }

  trackByPacket(index: number, packet: PacketSummary): string {
    // Unique ID for tracking: timestamp + sequence or just index if unique enough
    return packet.timestamp + packet.info + index;
  }
}
