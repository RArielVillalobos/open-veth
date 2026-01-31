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
  nodeMoved = output<{id: string, x: number, y: number}>(); // Nuevo evento

  private cy!: cytoscape.Core;
  sourceNodeId: string | null = null;
  
  // Context menu state
  contextMenu = {
    visible: false,
    x: 0,
    y: 0,
    nodeId: '',
    nodeName: ''
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

  onContextMenuAction(action: 'terminal' | 'delete') {
    if (action === 'terminal') {
      this.openTerminalRequest.emit(this.contextMenu.nodeName);
    }
    // TODO: Implement delete
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
                            'font-size': '9px',        // Reduced to 9px
                            'font-weight': 'bold',
                            'text-valign': 'top',
                
              // Changed from bottom to top
            'text-margin-y': -6,       // Negative margin to push it up
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
            'content': 'data(label)',      // Label below
            // Icon handling (simplified)
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
            'line-color': '#94a3b8',
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

    this.cy.on('tap', 'node', (evt) => {
      const clickedNode = evt.target;
      const clickedId = clickedNode.id();

      if (!this.sourceNodeId) {
        this.sourceNodeId = clickedId;
        clickedNode.addClass('selected-source');
      } else {
        if (this.sourceNodeId !== clickedId) {
          
          // Calculate robust dynamic interface names
          const getNextInterface = (nodeId: string) => {
             // 1. Get all used interface names for this node
             const usedNames = this.links()
               .filter(l => l.source === nodeId || l.target === nodeId)
               .map(l => l.source === nodeId ? l.source_int : l.target_int);
             
             // 2. Extract numbers (eth1 -> 1, eth2 -> 2)
             const usedNumbers = usedNames
               .map(name => parseInt(name.replace('eth', ''), 10))
               .filter(n => !isNaN(n))
               .sort((a, b) => a - b);

             // 3. Find first gap starting from 1
             let nextNum = 1;
             for (const num of usedNumbers) {
               if (num === nextNum) {
                 nextNum++;
               } else if (num > nextNum) {
                 // Gap found
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

    this.cy.on('cxttap', 'node', (evt) => {
      const node = evt.target;
      const pos = evt.renderedPosition; 
      
      this.contextMenu = {
        visible: true,
        x: pos.x + 20,
        y: pos.y + 20,
        nodeId: node.id(),
        nodeName: node.data('label')
      };
    });

    this.cy.on('tap', (evt) => {
      this.closeContextMenu();
      if (evt.target === this.cy) {
        this.cancelLinking();
      }
    });
    
    this.cy.on('zoom pan', () => this.closeContextMenu());

    // Event on drag end
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
      nodes.forEach(node => {
        if (this.cy.getElementById(node.id).empty()) {
          this.cy.add({
            group: 'nodes',
            data: { id: node.id, label: node.name, type: node.type },
            position: { x: node.x || 100, y: node.y || 100 }
          });
        }
      });

      this.links().forEach(link => {
        if (this.cy.getElementById(link.id).empty()) {
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
    });
  }
}
