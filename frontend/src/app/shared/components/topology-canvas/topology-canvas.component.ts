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
  edgeCreated = output<Link>();
  openTerminalRequest = output<string>();
  nodeMoved = output<{id: string, x: number, y: number}>();
  nodeSelected = output<string | null>();
  linkSelected = output<string | null>();
  nodeDelete = output<string>();
  linkDelete = output<string>();

  private cy!: cytoscape.Core;
  sourceNodeId: string | null = null;
  
  // Context menu state
  contextMenu = {
    visible: false,
    x: 0,
    y: 0,
    elementId: '',
    elementName: '',
    elementType: ''
  };

  constructor() {
    effect(() => {
      const currentNodes = this.nodes();
      if (this.cy) {
        this.updateGraph(currentNodes);
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

  private initCytoscape() {
    this.cy = cytoscape({
      container: this.container.nativeElement,
      style: [
        {
          selector: 'node',
          style: {
            'label': 'data(label)',
            'color': '#334155', // Slate-700
            'font-size': '9px',
            'font-weight': 'bold',
            'text-valign': 'top',
            'text-wrap': 'wrap',
            'text-max-width': '120px',
            'text-margin-y': -6,
            'background-color': '#fff',
            'border-width': 2,
            'width': 40,
            'height': 40,
            'text-outline-color': '#fff',
            'text-outline-width': 2
          }
        },
        // Router Style (Blue Circle)
        {
          selector: 'node[type="router"]',
          style: {
            'shape': 'ellipse',
            'background-color': '#eff6ff', // Blue-50
            'border-color': '#3b82f6',     // Blue-500
            'content': 'data(label)',
          }
        },
        // Host Style (Green Square)
        {
          selector: 'node[type="host"]',
          style: {
            'shape': 'round-rectangle',
            'background-color': '#f0fdf4', // Green-50
            'border-color': '#10b981',     // Green-500
            'width': 40,
            'height': 30
          }
        },
        {
          selector: '.selected-source',
          style: {
            'border-width': 4,
            'border-color': '#f59e0b', // Amber-500
            'background-color': '#fffbeb'
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
        // Do NOT emit selection here (as requested)
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
            id: 'link-' + Math.random().toString(36).substr(2, 5),
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
      // No necesitamos cancelar linking porque no se puede linkear desde un edge
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
        elementType: node.data('type')
      };
    });

    // Right Click (Context Menu) - Edge
    this.cy.on('cxttap', 'edge', (evt) => {
      const edge = evt.target;
      const pos = evt.renderedPosition; 
      
      this.contextMenu = {
        visible: true,
        x: pos.x + 20,
        y: pos.y + 20,
        elementId: edge.id(),
        elementName: 'Link',
        elementType: 'edge'
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

  private updateGraph(nodes: TopologyNode[]) {
    this.cy.batch(() => {
      // 1. Add/Update Nodes
      nodes.forEach(node => {
        // Build rich label with IPs
        let label = node.name;
        if (node.interfaces && node.interfaces.length > 0) {
          const ips = node.interfaces
            .filter(i => i.ifname !== 'lo' && i.ifname !== 'mgmt0')
            .map(i => {
              const ipv4 = i.addr_info?.find(addr => !addr.local.includes(':'));
              return ipv4 ? `${ipv4.local}/${ipv4.prefixlen} (${i.ifname})` : null;
            })
            .filter(Boolean);
          
          if (ips.length > 0) {
            label += '\n' + ips.join('\n');
          }
        }

        const existing = this.cy.getElementById(node.id);
        if (existing.empty()) {
          this.cy.add({
            group: 'nodes',
            data: { id: node.id, label: label, type: node.type },
            position: { x: node.x || 100, y: node.y || 100 }
          });
        } else {
          // Update Label if changed
          if (existing.data('label') !== label) {
            existing.data('label', label);
          }
        }
      });

      // 2. Add/Update Links
      this.links().forEach(link => {
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
  }
}