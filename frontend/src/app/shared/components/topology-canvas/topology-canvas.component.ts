import { Component, ElementRef, ViewChild, AfterViewInit, input, output, effect, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import cytoscape from 'cytoscape';
import { Node as TopologyNode, Link } from '../../../models/topology.model';

@Component({
  selector: 'app-topology-canvas',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './topology-canvas.component.html',
  styleUrl: './topology-canvas.component.scss'
})
export class TopologyCanvasComponent implements AfterViewInit, OnDestroy {
  @ViewChild('cyContainer') container!: ElementRef;
  
  nodes = input.required<TopologyNode[]>();
  links = input.required<Link[]>();
  terminalNode = input<string | null>(null);
  edgeCreated = output<Link>();
  openTerminalRequest = output<string>();
  openSniffRequest = output<{nodeId: string, nodeName: string, iface: string}>();
  nodeMoved = output<{id: string, x: number, y: number}>();
  nodeSelected = output<string | null>();
  linkSelected = output<string | null>();
  nodeDelete = output<string>();
  linkDelete = output<string>();

  private cy!: cytoscape.Core;
  private canvasReady = false;
  sourceNodeId: string | null = null;
  
  // Context menu state
  contextMenu = {
    visible: false,
    x: 0,
    y: 0,
    elementId: '',
    elementName: '',
    elementType: '',
    // Optional data for links
    sourceNode: { nodeId: '', nodeName: '', iface: '' },
    targetNode: { nodeId: '', nodeName: '', iface: '' }
  };

  constructor() {
    effect(() => {
      this.nodes();
      if (this.cy) {
        this.updateCanvas();
      }
    });

    effect(() => {
      const name = this.terminalNode();
      if (!this.cy) return;
      this.cy.nodes().removeClass('terminal-active');
      if (name) {
        const match = this.cy.nodes().filter(n => n.data('name') === name);
        match.addClass('terminal-active');
      }
    });
  }

  ngAfterViewInit() {
    this.initCytoscape();
  }

  ngOnDestroy() {
    if (this.cy) this.cy.destroy();
  }

  getNodeName(id: string): string {
    return this.nodes().find(n => n.id === id)?.name || id;
  }

  cancelLinking() {
    this.sourceNodeId = null;
    this.cy.nodes().removeClass('selected-source');
  }

  closeContextMenu() {
    this.contextMenu = { ...this.contextMenu, visible: false };
  }

  onContextMenuAction(action: 'terminal' | 'delete' | 'properties') {
    if (action === 'terminal') {
      this.openTerminalRequest.emit(this.contextMenu.elementName);
    } else if (action === 'properties') {
      if (this.contextMenu.elementType === 'edge') {
        this.linkSelected.emit(this.contextMenu.elementId);
      } else {
        this.nodeSelected.emit(this.contextMenu.elementId);
      }
    } else if (action === 'delete') {
      if (this.contextMenu.elementType === 'edge') {
        this.linkDelete.emit(this.contextMenu.elementId);
      } else {
        this.nodeDelete.emit(this.contextMenu.elementId);
      }
    }
    this.closeContextMenu();
  }

  triggerOpenSniff(data: {nodeId: string, nodeName: string, iface: string}) {
    this.openSniffRequest.emit(data);
    this.closeContextMenu();
  }

  private initCytoscape() {
    this.cy = cytoscape({
      container: this.container.nativeElement,
      style: [
        {
          selector: 'node',
          style: {
            'label': 'data(label)',
            'color': '#94a3b8',
            'font-size': '10px',
            'font-weight': 'bold',
            'text-valign': 'bottom',
            'text-wrap': 'wrap',
            'text-max-width': '120px',
            'text-margin-y': 4,
            'width': 56,
            'height': 48,
            'shape': 'rectangle',
            'background-color': 'transparent',
            'background-opacity': 0,
            'border-width': 0,
            'text-outline-color': '#0f172a',
            'text-outline-width': 0
          }
        },
        // Router Style
        {
          selector: 'node[type="router"]',
          style: {
            'background-image': 'assets/icons/router.svg',
            'background-fit': 'contain',
            'background-clip': 'none',
            'width': 52,
            'height': 52
          }
        },
        // Switch Style
        {
          selector: 'node[type="switch"]',
          style: {
            'background-image': 'assets/icons/switch.svg',
            'background-fit': 'contain',
            'background-clip': 'none',
            'width': 52,
            'height': 52
          }
        },
        // Host Style
        {
          selector: 'node[type="host"]',
          style: {
            'background-image': 'assets/icons/host.svg',
            'background-fit': 'contain',
            'background-clip': 'none',
            'width': 52,
            'height': 52
          }
        },
        {
          selector: '.terminal-active',
          style: {
            'border-width': 2,
            'border-color': '#34d399',
            'border-opacity': 0.9,
          }
        },
        {
          selector: '.selected-source',
          style: {
            'border-width': 3,
            'border-color': '#8b5cf6',
            'border-style': 'solid',
            'background-color': '#ede9fe',
            'background-opacity': 0.6
          }
        },
        {
          selector: 'edge',
          style: {
            'width': 3,
            'line-color': '#94a3b8', // Gray standard
            'curve-style': 'bezier',
            'source-label': 'data(source_int)',
            'target-label': 'data(target_int)',
            'source-text-offset': 20,
            'target-text-offset': 20,
            'font-size': '9px',
            'color': '#64748b',
            'text-background-opacity': 1,
            'text-background-color': '#f8fafc',
            'text-background-padding': '2px',
            'text-background-shape': 'roundrectangle'
          }
        }
      ]
    });

    // --- Event Listeners ---

    // Tap on Node (Start/End Link)
    this.cy.on('tap', 'node', (evt) => {
      const clickedNode = evt.target;
      const clickedId = clickedNode.id();

      if (!this.sourceNodeId) {
        // Mode: Start Link
        this.sourceNodeId = clickedId;
        clickedNode.addClass('selected-source');
      } else {
        // Mode: End Link
        if (this.sourceNodeId !== clickedId) {
          
          // Calculate robust dynamic interface names
          const getNextInterface = (nodeId: string) => {
             const usedNames = this.links()
               .filter(l => l.source === nodeId || l.target === nodeId)
               .map(l => l.source === nodeId ? l.source_int : l.target_int);
             
             const usedNumbers = usedNames
               .map(name => parseInt(name.replace('eth', ''), 10))
               .filter(n => !isNaN(n))
               .sort((a, b) => a - b);

             let nextNum = 1;
             for (const num of usedNumbers) {
               if (num === nextNum) {
                 nextNum++;
               } else if (num > nextNum) {
                 break;
               }
             }
             return `eth${nextNum}`;
          };

          const newLink: Link = {
            id: 'link-' + Math.random().toString(36).substring(2, 7),
            source: this.sourceNodeId,
            target: clickedId,
            source_int: getNextInterface(this.sourceNodeId),
            target_int: getNextInterface(clickedId)
          };
          
          this.edgeCreated.emit(newLink);
          this.cancelLinking();
        }
      }
    });

    // Tap on Edge (Select Link)
    this.cy.on('tap', 'edge', (evt) => {
      const edge = evt.target;
      this.linkSelected.emit(edge.id());
    });

    // Right Click (Context Menu) - Node
    this.cy.on('cxttap', 'node', (evt) => {
      const node = evt.target;
      const pos = evt.renderedPosition; 
      
      this.contextMenu = {
        visible: true,
        x: pos.x + 20,
        y: pos.y + 20,
        elementId: node.id(),
        elementName: node.data('label').split('\n')[0],
        elementType: node.data('type'),
        sourceNode: { nodeId: '', nodeName: '', iface: '' },
        targetNode: { nodeId: '', nodeName: '', iface: '' }
      };
    });

    // Right Click (Context Menu) - Edge
    this.cy.on('cxttap', 'edge', (evt) => {
      const edge = evt.target;
      const data = edge.data();
      const pos = evt.renderedPosition; 
      
      // Look up full node objects to get names
      const sNode = this.nodes().find(n => n.id === data.source);
      const tNode = this.nodes().find(n => n.id === data.target);

      this.contextMenu = {
        visible: true,
        x: pos.x + 20,
        y: pos.y + 20,
        elementId: edge.id(),
        elementName: 'Link',
        elementType: 'edge',
        // Populate with correct structure for the event
        sourceNode: { 
            nodeId: data.source, 
            nodeName: sNode?.name || 'A', 
            iface: data.source_int 
        },
        targetNode: { 
            nodeId: data.target, 
            nodeName: tNode?.name || 'B', 
            iface: data.target_int 
        }
      };
    });

    // Tap Background (Close everything)
    this.cy.on('tap', (evt) => {
      this.closeContextMenu();
      if (evt.target === this.cy) {
        this.cancelLinking();
        this.nodeSelected.emit(null);
        this.linkSelected.emit(null);
      }
    });
    
    this.cy.on('zoom pan', () => this.closeContextMenu());

    // Drag End (Update Position)
    this.cy.on('dragfree', 'node', (evt) => {
      const node = evt.target;
      const pos = node.position();
      this.nodeMoved.emit({
        id: node.id(),
        x: pos.x,
        y: pos.y
      });
    });
  }

  private updateCanvas(): void {
    // Access signal value once
    const nodes = this.nodes();
    const newNodeIds: string[] = [];

    this.cy.batch(() => {
      // 1. Add/Update Nodes
      nodes.forEach(node => {
        // Build rich label with IPs (Limited to 2 for clarity)
        let label = node.name;
        if (node.interfaces && node.interfaces.length > 0) {
          const relevantIfaces = node.interfaces
            .filter(i => i.ifname !== 'lo' && i.ifname !== 'mgmt0');

          const ips = relevantIfaces
            .slice(0, 2)
            .map(i => {
              const ipv4 = i.addr_info?.find(addr => !addr.local.includes(':'));
              return ipv4 ? `${ipv4.local}/${ipv4.prefixlen} (${i.ifname})` : null;
            })
            .filter(Boolean);

          if (ips.length > 0) {
            label += '\n' + ips.join('\n');
            if (relevantIfaces.length > 2) {
                label += '\n...';
            }
          }
        }

        const existing = this.cy.getElementById(node.id);
        if (existing.empty()) {
          this.cy.add({
            group: 'nodes',
            data: { id: node.id, label: label, name: node.name, type: node.type },
            position: { x: node.x || 100, y: node.y || 100 }
          });
          if (this.canvasReady) newNodeIds.push(node.id);
        } else {
          // Update Label if changed
          if (existing.data('label') !== label) {
            existing.data('label', label);
          }
        }
      });

      // 2. Add/Update Links
      // Create a set of valid node IDs for fast lookup
      const validNodeIds = new Set(nodes.map(n => n.id));

      this.links().forEach(link => {
        // Safety check: Ensure both endpoints exist
        if (!validNodeIds.has(link.source) || !validNodeIds.has(link.target)) {
          console.warn(`Skipping orphan link ${link.id}: source ${link.source} or target ${link.target} not found.`);
          return;
        }

        const existingLink = this.cy.getElementById(link.id);
        if (existingLink.empty()) {
          this.cy.add({
            group: 'edges',
            data: {
              id: link.id,
              source: link.source,
              target: link.target,
              source_int: link.source_int,
              target_int: link.target_int
            }
          });
        }
      });

      // 3. Remove deleted Nodes
      const currentIds = new Set(nodes.map(n => n.id));
      this.cy.nodes().forEach(ele => {
        if (!currentIds.has(ele.id())) {
          this.cy.remove(ele);
        }
      });

      // 4. Remove deleted Links
      const currentLinkIds = new Set(this.links().map(l => l.id));
      this.cy.edges().forEach(ele => {
        if (!currentLinkIds.has(ele.id())) {
          this.cy.remove(ele);
        }
      });
    });

    if (!this.canvasReady) {
      this.canvasReady = true;
      return;
    }

    // Highlight newly added nodes
    newNodeIds.forEach(id => {
      const el = this.cy.getElementById(id);
      el.style({ 'border-width': 2, 'border-color': '#60a5fa', 'border-opacity': 1 });
      el.animate({
        style: { 'border-opacity': 0 },
        duration: 1500,
        complete: () => el.removeStyle('border-width border-color border-opacity'),
      });
    });
  }
}
